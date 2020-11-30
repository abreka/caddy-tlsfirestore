package storagefirestore

import (
	"context"
	"fmt"
	"github.com/caddyserver/certmagic"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

const testKey = "0123456789abcdef"

type StorageTS struct {
	suite.Suite
	s *Storage
}

func TestStorageTestSuite(t *testing.T) {
	suite.Run(t, new(StorageTS))
}

func (ts *StorageTS) SetupSuite() {
	ctx := context.Background()
	s := New()
	ts.NotNil(s)

	os.Setenv(EnvNameProjectId, "testproj")
	os.Setenv(EnvNameAesKey, "MDEyMzQ1Njc4OWFiY2RlZg==")
	ts.NoError(s.loadOverrides(ctx))
	ts.NoError(s.setupAfterProvision(ctx))

	got, err := s.CertMagicStorage()
	ts.NoError(err)
	ts.Equal(s, got)

	ts.s = s
}

// I'm not sure if these are the only possible paths.
// If they are, it makes more sense to create sub-collections than just use
// the keys -- the query times would be better.
func (ts *StorageTS) Test_keyToRef() {
	prefix := fmt.Sprintf("projects/%s/databases/(default)/documents/", ts.s.ProjectId)

	ref := ts.s.keyToRef(certmagic.KeyBuilder{}.SiteCert("issuer", "domain.com"))
	ts.Equal(prefix+"certmagic/certificates\\issuer\\domain.com\\domain.com.crt", ref.Path)

	ref = ts.s.keyToRef(certmagic.KeyBuilder{}.CertsPrefix("acme"))
	ts.Equal(prefix+"certmagic/certificates\\acme", ref.Path)

	ref = ts.s.keyToRef(certmagic.KeyBuilder{}.CertsSitePrefix("acme", "domain.com"))
	ts.Equal(prefix+"certmagic/certificates\\acme\\domain.com", ref.Path)

	ref = ts.s.keyToRef(certmagic.KeyBuilder{}.SiteMeta("issuer", "domain.com"))
	ts.Equal(prefix+"certmagic/certificates\\issuer\\domain.com\\domain.com.json", ref.Path)

	ref = ts.s.keyToRef(certmagic.KeyBuilder{}.SitePrivateKey("issuer", "domain.com"))
	ts.Equal(prefix+"certmagic/certificates\\issuer\\domain.com\\domain.com.key", ref.Path)
}

func (ts *StorageTS) Test_attemptLock() {
	ctx := context.Background()
	key := certmagic.KeyBuilder{}.SiteCert("test", "attempt-lock.com")

	replica := New()
	replica.ProjectId = "testproj"
	replica.AesKey = []byte(testKey)
	ts.NoError(replica.setupAfterProvision(ctx))
	ts.NotNil(replica)

	defer func() {
		_, err := ts.s.keyToRef(key).Delete(ctx)
		ts.NoError(err)
	}()

	// No certificate exists.
	ts.Nil(ts.s.locks[key])
	ts.NoError(ts.s.Lock(ctx, key))
	ts.NotNil(ts.s.locks[key])

	// The certificate is already locked *elsewhere*
	ts.Error(replica.attemptLock(ctx, key), errAlreadyLocked.Error())
	ts.Nil(replica.locks[key])

	// The local process can enter the lock it already holds.
	ts.NoError(ts.s.Lock(ctx, key))
	ts.NotNil(ts.s.locks[key])

	// The certificate is now unlocked.
	ts.NoError(ts.s.Unlock(key))
	ts.Nil(ts.s.locks[key])

	// A second unlock is an error.
	err := ts.s.Unlock(key)
	ts.Error(err)
	ts.Contains(err.Error(), ".crt was not found")

	// And can be locked again
	ts.NoError(ts.s.attemptLock(ctx, key))
	ts.NotNil(ts.s.locks[key])

	// And unlocked
	ts.NoError(ts.s.Unlock(key))
	ts.Nil(ts.s.locks[key])
}

func (ts *StorageTS) getRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if !ts.NoError(err) {
		ts.FailNow("bad rng")
	}
	return b
}

func (ts *StorageTS) Test_Crud() {
	key := certmagic.KeyBuilder{}.SiteCert("test", "test-store.com")

	now := UTCNow()

	// At this point, no entity exists. Create it fresh.
	expected := ts.getRandomBytes(255)
	ts.False(ts.s.Exists(key))

	// Deleting a non-existent key is an ErrNotExists error.
	err := ts.s.Delete(key)
	ts.Error(err)
	ts.IsType(certmagic.ErrNotExist(err), err)

	// Load returns an error
	_, err = ts.s.Load(key)
	ts.Error(err)
	ts.IsType(certmagic.ErrNotExist(err), err)

	// Stat returns an error
	_, err = ts.s.Stat(key)
	ts.Error(err)
	ts.IsType(certmagic.ErrNotExist(err), err)

	// Create it.
	err = ts.s.Store(key, expected)
	ts.NoError(err)
	ts.True(ts.s.Exists(key))

	// And verify it exists.
	got, err := ts.s.Load(key)
	ts.NoError(err)
	ts.Equal(expected, got)

	keyInfo, err := ts.s.Stat(key)
	ts.NoError(err)
	ts.Equal(key, keyInfo.Key)
	ts.Len(expected, int(keyInfo.Size))
	ts.False(keyInfo.IsTerminal)
	ts.True(keyInfo.Modified.After(now))

	// Now, try to store a new value.
	expected = ts.getRandomBytes(255)
	err = ts.s.Store(key, expected)
	ts.NoError(err)

	// And verify it was updated.
	got, err = ts.s.Load(key)
	ts.NoError(err)
	ts.Equal(expected, got)
	keyInfo, err = ts.s.Stat(key)
	ts.NoError(err)
	ts.Equal(key, keyInfo.Key)
	ts.Len(expected, int(keyInfo.Size))
	ts.False(keyInfo.IsTerminal)
	ts.True(keyInfo.Modified.After(now))

	// Now delete it.
	ts.NoError(ts.s.Delete(key))
	ts.False(ts.s.Exists(key))
}

func (ts *StorageTS) Test_UpdateFreshness() {
	key := certmagic.KeyBuilder{}.SiteCert("test", "test-update-freshness.com")

	expected := ts.getRandomBytes(255)
	err := ts.s.Store(key, expected)
	ts.NoError(err)

	recordT0, err := ts.s.loadAndDecrypt(key)
	ts.NoError(err)

	time.Sleep(time.Millisecond * 2)

	ts.NoError(ts.s.updateFreshness(context.Background(), key))

	recordT1, err := ts.s.loadAndDecrypt(key)
	ts.NoError(err)

	ts.True(recordT1.LockedAt.After(recordT0.LockedAt))

	ts.s.Delete(key)
}

func (ts *StorageTS) Test_keepLockFresh() {
	ts.s.FreshnessSeconds = 1
	defer func() {
		ts.s.FreshnessSeconds = DefaultFreshnessIntervalSeconds
	}()

	key := certmagic.KeyBuilder{}.SiteCert("test", "test-keep-lock-fresh.com")

	expected := ts.getRandomBytes(255)
	err := ts.s.Store(key, expected)
	ts.NoError(err)

	recordT0, err := ts.s.loadAndDecrypt(key)
	ts.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()
	ts.s.keepLockFresh(ctx, key)

	recordT1, err := ts.s.loadAndDecrypt(key)
	ts.NoError(err)

	ts.True(recordT1.LockedAt.After(recordT0.LockedAt))

	// Context already cancelled.
	ts.s.keepLockFresh(ctx, key)
	recordT2, err := ts.s.loadAndDecrypt(key)
	ts.NoError(err)
	ts.Equal(recordT1, recordT2)

	ts.s.Delete(key)
}

func (ts *StorageTS) Test_ListNonRecursive() {
	expected := certmagic.KeyBuilder{}.CertsSitePrefix("test", "test-list.com")
	parts := strings.Split(expected, "/")
	query := strings.Join(parts[:len(parts)-1], "/")

	// List returns ErrNotFound
	gotKeys, err := ts.s.List(query, false)
	ts.Error(err)
	ts.Len(gotKeys, 0)
	ts.IsType(certmagic.ErrNotExist(err), err)

	expectedKeys := []string{
		certmagic.KeyBuilder{}.SiteCert("test", "test-list.com"),
		certmagic.KeyBuilder{}.SiteMeta("test", "test-list.com"),
		certmagic.KeyBuilder{}.SitePrivateKey("test", "test-list.com"),
		certmagic.KeyBuilder{}.SitePrivateKey("test", "test-list-other.com"),
	}

	for _, key := range expectedKeys {
		ts.NoError(ts.s.Store(key, ts.getRandomBytes(255)))
	}

	gotKeys, err = ts.s.List(query, false)
	ts.NoError(err)
	ts.Len(gotKeys, 2)
	ts.Contains(gotKeys, expected)
	ts.Contains(gotKeys, strings.ReplaceAll(expected, "test-list.com", "test-list-other.com"))

	for _, key := range expectedKeys {
		ts.NoError(ts.s.Delete(key))
	}
}

func (ts *StorageTS) Test_ListRecursive() {
	query := certmagic.KeyBuilder{}.CertsSitePrefix("test", "test-list.com")

	// List returns ErrNotFound
	gotKeys, err := ts.s.List(query, false)
	ts.Error(err)
	ts.Len(gotKeys, 0)
	ts.IsType(certmagic.ErrNotExist(err), err)

	expectedKeys := []string{
		certmagic.KeyBuilder{}.SiteCert("test", "test-list.com"),
		certmagic.KeyBuilder{}.SiteMeta("test", "test-list.com"),
		certmagic.KeyBuilder{}.SitePrivateKey("test", "test-list.com"),
		certmagic.KeyBuilder{}.SitePrivateKey("test", "test-list-other.com"),
	}

	for _, key := range expectedKeys {
		ts.NoError(ts.s.Store(key, ts.getRandomBytes(255)))
	}

	gotKeys, err = ts.s.List(query, true)
	ts.NoError(err)
	ts.Len(gotKeys, 3)
	ts.Contains(gotKeys, expectedKeys[0])
	ts.Contains(gotKeys, expectedKeys[1])
	ts.Contains(gotKeys, expectedKeys[2])

	for _, key := range expectedKeys {
		ts.NoError(ts.s.Delete(key))
	}
}
