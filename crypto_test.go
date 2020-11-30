package storagefirestore

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStorage_encrypt(t *testing.T) {
	s := New()

	msg := []byte("hello, world")

	ciphertext, err := s.encrypt(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size 0")
	assert.Nil(t, ciphertext)

	s.AesKey = []byte("0123456789abcdef")
	ciphertext, err = s.encrypt(msg)
	assert.NoError(t, err)
	assert.NotNil(t, ciphertext)

	got, err := s.decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, msg, got)

	s.AesKey = []byte("123456789abcdef0")
	got, err = s.decrypt(ciphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message authentication failed")
	assert.Nil(t, got)
}
