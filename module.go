package storagefirestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
	"os"
	"strconv"
)

const (
	EnvNameProjectId      = "CADDY_CLUSTERING_PROJECT_ID"
	EnvNameAesKeySecretId = "CADDY_CLUSTERING_AES_KEY_SECRET_ID"
	EnvNameAesKey         = "CADDY_CLUSTERING_AESKEY_BASE64"
)

func init() {
	caddy.RegisterModule(&Storage{})
}

func (s *Storage) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "caddy.storage.firestore",
		New: func() caddy.Module {
			return New()
		},
	}
}

func (s *Storage) Provision(ctx caddy.Context) error {
	s.logger = ctx.Logger(s).Sugar()
	s.logger.Infof("TLS storage is using Firestore")

	err := s.loadOverrides(ctx)
	if err != nil {
		return err
	}

	return s.setupAfterProvision(ctx)
}

func (s *Storage) loadOverrides(ctx context.Context) error {
	if projectId, found := os.LookupEnv(EnvNameProjectId); found && projectId != "" {
		s.ProjectId = projectId
	}

	if secretId, found := os.LookupEnv(EnvNameAesKeySecretId); found && secretId != "" {
		s.AESKeySecretId = secretId
	}

	if b64Key, found := os.LookupEnv(EnvNameAesKey); found && b64Key != "" {
		err := s.ingestBase64Key(b64Key)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) CertMagicStorage() (certmagic.Storage, error) {
	return s, nil
}

func (s *Storage) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		key := d.Val()
		var value string

		if !d.Args(&value) {
			continue
		}

		switch key {
		case "project_id":
			if value != "" {
				s.ProjectId = value
			}
		case "collection":
			if value != "" {
				s.Collection = value
			}
		case "aes_key_secret_id":
			if value != "" {
				s.AESKeySecretId = value
			}
		case "min_lock_poll_seconds":
			if value != "" {
				seconds, err := strconv.Atoi(value)
				if err == nil {
					s.MinPollSeconds = seconds
				}
			}
		case "max_lock_poll_seconds":
			if value != "" {
				seconds, err := strconv.Atoi(value)
				if err == nil {
					s.MaxPollSeconds = seconds
				}
			}
		case "lock_freshness_seconds":
			if value != "" {
				seconds, err := strconv.Atoi(value)
				if err == nil {
					s.FreshnessSeconds = seconds
				}
			}
		case "aes_key":
			if value != "" {
				err := s.ingestBase64Key(value)
				if err == nil {
					// TODO: why not error reporting?
				}
			}
		}
	}
	return nil
}

func (s *Storage) ingestBase64Key(b64Data string) error {
	sk, err := base64.URLEncoding.DecodeString(b64Data)
	if err != nil {
		return err
	}
	if k := len(sk); k != 16 && k != 24 && k != 32 {
		return fmt.Errorf("invalid AES key size %d", k)
	}
	s.AesKey = sk
	return nil
}
