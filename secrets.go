package storagefirestore

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"context"
	"fmt"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// Load the AES Key from Google Secrets Manager.
//
// I don't like storing secrets in environmental variables. Feels like asking
// for a leak. This method uses Google Secrets Manager instead.
//
// TODO: Verify `caddy reload` will provision.
func (s *Storage) loadAESKeyFromSecret(ctx context.Context) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}

	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", s.ProjectId, s.AESKeySecretId),
	})

	if err != nil {
		return fmt.Errorf("failed to access secret version: %w", err)
	}

	s.AesKey = result.Payload.Data
	return nil
}
