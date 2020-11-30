package storagefirestore

import (
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"
	"path"
	"strings"
	"sync"
)

// TODO: Verifications
//

// Storage uses Firestore for a backend.
type Storage struct {
	ProjectId        string `json:"project_id"`
	Collection       string `json:"collection"`
	AESKeySecretId   string `json:"aes_key_secret_id"`
	MinPollSeconds   int    `json:"min_lock_poll_seconds"`
	MaxPollSeconds   int    `json:"max_lock_poll_seconds"`
	FreshnessSeconds int    `json:"lock_freshness_seconds"`
	AesKey           []byte `json:"aes_key"`

	client *firestore.Client
	logger *zap.SugaredLogger

	// > Implementations of Storage must be safe for concurrent use.
	//
	// The consul implementation didn't seem to use a mutex to guard
	// its local locks. And the acquireLock and releaseLock methods in
	// certmagic have their own lock map that is mutex protected. But, the
	// actual call to Lock() and Unlock() in storage aren't in the critical
	// section. I added a mutex to the local locks because I'd rather be
	// safe, but it may be worth tracing the access path through certmagic.
	locks map[string]context.CancelFunc
	m     sync.Mutex

	certmagic.Storage
}

const (
	DefaultCollection = "certmagic"

	// The certmagic/filestorage.go uses a 1 second polling interval.
	// I'm using that as the minimum, but making it take up to 5 seconds
	// (uniformly distributed) because firestore has to do network
	// traversal and I'm not sure how transaction contention will
	// impact things.
	DefaultMinPollSeconds = 1
	DefaultMaxPollSeconds = 5

	// How often to update the lock's timestamp. Locks older than this
	// can be considered stale (e.g. failed process). Five seconds
	// is okay for as ingle document. The maximum sustained write rate
	// according to the firestore quotas is 1 per second.
	DefaultFreshnessIntervalSeconds = 5
)

func New() *Storage {
	return &Storage{
		Collection:       DefaultCollection,
		MinPollSeconds:   DefaultMinPollSeconds,
		MaxPollSeconds:   DefaultMaxPollSeconds,
		FreshnessSeconds: DefaultFreshnessIntervalSeconds,
		locks:            map[string]context.CancelFunc{},
	}
}

func (s *Storage) setupAfterProvision(ctx context.Context) error {
	client, err := firestore.NewClient(ctx, s.ProjectId)
	if err != nil {
		return err
	}
	s.client = client

	if s.AESKeySecretId == "" {
		return nil
	}

	return s.loadAESKeyFromSecret(ctx)
}

func (s *Storage) Store(key string, value []byte) error {
	ref := s.keyToRef(key)

	ciphertext, err := s.encrypt(value)
	if err != nil {
		return err
	}
	// TODO: add context timeout
	return s.client.RunTransaction(context.Background(), func(ctx context.Context, t *firestore.Transaction) error {
		if _, err := t.Get(ref); err != nil {
			if IsDocNotFound(err) {
				// Technically, I don't *think* this is possible. I think the lock
				// MUST be called first, so the document MUST exist at this point.
				now := UTCNow()
				return t.Create(ref, &Record{
					Raw:       ciphertext,
					CreatedAt: now,
					UpdatedAt: now,
				})
			} else {
				return err
			}
		}

		return t.Update(ref, []firestore.Update{
			{Path: "updatedAt", Value: UTCNow()},
			{Path: "raw", Value: ciphertext},
		})
	})
}

func (s *Storage) Load(key string) ([]byte, error) {
	c, err := s.loadAndDecrypt(key)
	if err != nil {
		return nil, err
	}
	return c.Raw, nil
}

func (s *Storage) loadAndDecrypt(key string) (*Record, error) {
	// TODO: add timeout
	doc, err := s.keyToRef(key).Get(context.Background())
	if err != nil {
		if IsDocNotFound(err) {
			return nil, certmagic.ErrNotExist(err)
		} else {
			// TODO: log
			return nil, err
		}
	}

	var cert Record
	err = doc.DataTo(&cert)
	if err != nil {
		// TODO: LOG
		return nil, err
	}

	plaintext, err := s.decrypt(cert.Raw)
	if err != nil {
		return nil, err
	}
	cert.Raw = plaintext
	return &cert, nil
}

func (s *Storage) Delete(key string) error {
	ref := s.keyToRef(key)

	return s.client.RunTransaction(context.Background(), func(ctx context.Context, t *firestore.Transaction) error {
		if _, err := t.Get(ref); err != nil {
			if IsDocNotFound(err) {
				return certmagic.ErrNotExist(err)
			}
			return err
		}

		return t.Delete(ref)
	})
}

func (s *Storage) Exists(key string) bool {
	_, err := s.keyToRef(key).Get(context.Background())
	return err == nil
}

func (s *Storage) List(prefix string, recursive bool) ([]string, error) {
	var keysFound []string

	// TODO: add timeout
	// TODO: use sub collections instead for optimization
	// TODO: look at List() usage
	snapshots, err := s.client.Collection(s.Collection).Documents(context.Background()).GetAll()
	if err != nil {
		return nil, err
	}

	translatedPrefix := firestoreSafeKey(prefix)
	for _, snapshot := range snapshots {
		if strings.HasPrefix(snapshot.Ref.ID, translatedPrefix) {
			keysFound = append(keysFound, strings.ReplaceAll(snapshot.Ref.ID, "\\", "/"))
		}
	}

	if len(keysFound) == 0 {
		return keysFound, certmagic.ErrNotExist(fmt.Errorf("key %s not found", prefix))
	}

	// if recursive wanted, just return all keys
	if recursive {
		return keysFound, nil
	}

	// for non-recursive split path and look for unique keys just under given prefix
	keysMap := make(map[string]bool)
	for _, key := range keysFound {
		prefixTrimmed := strings.TrimPrefix(key, prefix+"/")
		dir := strings.Split(prefixTrimmed, "/")
		keysMap[dir[0]] = true
	}

	keysFound = make([]string, 0)
	for key := range keysMap {
		keysFound = append(keysFound, path.Join(prefix, key))
	}

	return keysFound, nil
}

func (s *Storage) Stat(key string) (certmagic.KeyInfo, error) {
	c, err := s.loadAndDecrypt(key)
	if err != nil {
		return certmagic.KeyInfo{}, err
	}

	return certmagic.KeyInfo{
		Key:        key,
		Modified:   c.UpdatedAt,
		Size:       int64(len(c.Raw)),
		IsTerminal: false,
	}, nil
}
