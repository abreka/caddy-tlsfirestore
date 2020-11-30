package storagefirestore

import (
	"context"
	"encoding/json"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestStorage_loadOverrides(t *testing.T) {
	updates := map[string]string{
		EnvNameProjectId: "fake-override-project",
		EnvNameAesKey: "YWVzLW92ZXJyaWRlLWtleQ==",
		EnvNameAesKeySecretId: "override-secret-id",
	}

	original := map[string]string{}
	for k, v := range updates {
		original[k]	= os.Getenv(k)
		os.Setenv(k, v)
	}

	defer func(){
		for k := range updates {
			os.Setenv(k, original[k])
		}
	}()

	s := New()
	err := s.loadOverrides(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, updates[EnvNameProjectId], s.ProjectId)
	assert.Equal(t, []byte("aes-override-key"), s.AesKey)
	assert.Equal(t, updates[EnvNameAesKeySecretId], "override-secret-id")

	os.Setenv(EnvNameAesKey, "XXX")
	err = s.loadOverrides(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal base64")
}

func TestStorage_SERDE(t *testing.T) {
	d := caddyfile.NewTestDispenser(`{
    storage firestore {
           project_id             "cf-project-id"
           collection             "cf-collection"
           min_lock_poll_seconds  24
           max_lock_poll_seconds  42
           lock_freshness_seconds 100
           aes_key                "Y2YtdGVzdC1rZXkxMjM0NQ=="
           aes_key_secret_id      "cf-secret"
    }
}`)
	s := New()
	assert.NoError(t, s.UnmarshalCaddyfile(d))

	assert.Equal(t, "cf-project-id", s.ProjectId)
	assert.Equal(t, "cf-collection", s.Collection)
	assert.Equal(t, 24, s.MinPollSeconds)
	assert.Equal(t, 42, s.MaxPollSeconds)
	assert.Equal(t, 100, s.FreshnessSeconds)
	assert.Equal(t, []byte("cf-test-key12345"), s.AesKey)
	assert.Equal(t, "cf-secret", s.AESKeySecretId)

	// Make sure json works, too.
	b, err := json.Marshal(s)
	assert.NoError(t, err)
	var dup Storage
	err = json.Unmarshal(b, &dup)
	s.locks = nil
	assert.NoError(t, err)
	assert.Equal(t, s, &dup)

}

func TestStorage_ingestBase64Key(t *testing.T) {
	s := New()
	assert.Error(t, s.ingestBase64Key("!!!!!")) // Bad base64
	assert.Error(t, s.ingestBase64Key("YmFk"))  // Bad length
}