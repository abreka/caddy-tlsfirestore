package storagefirestore

import (
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestStorage_loadAESKeyFromSecret(t *testing.T) {
	projectId := os.Getenv("TEST_PROJECT_ID")
	aesSecretKeyID := os.Getenv("TEST_AES_KEY_SECRET_ID")
	if projectId == "" && aesSecretKeyID == "" {
		t.Skip("TEST_PROJECT_ID and TEST_AES_KEY_SECRET_ID not set")
	}

	s := New()
	assert.Nil(t, s.AesKey)
	s.ProjectId = projectId
	s.AESKeySecretId = aesSecretKeyID

	err := s.loadAESKeyFromSecret(context.Background())
	assert.NoError(t, err)
	assert.Len(t, s.AesKey, 32)
}
