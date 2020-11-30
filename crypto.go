package storagefirestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func (s *Storage) encrypt(plaintext []byte) ([]byte, error) {
	gcm, err := s.getAESGCM()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, fmt.Errorf("unable to generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil

}

func (s *Storage) decrypt(ciphertext []byte) ([]byte, error) {
	gcm, err := s.getAESGCM()
	if err != nil {
		return nil, err
	}

	out, err := gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failure: %w", err)
	}

	return out, nil
}

func (s *Storage) getAESGCM() (cipher.AEAD, error) {
	c, err := aes.NewCipher(s.AesKey)
	if err != nil {
		return nil, fmt.Errorf("unable to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("unable to create GCM cipher: %w", err)
	}

	return gcm, err
}
