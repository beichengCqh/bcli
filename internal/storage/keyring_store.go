package storage

import (
	"errors"

	"bcli/internal/core/auth"
	"github.com/zalando/go-keyring"
)

type KeyringCredentialStore struct{}

func (KeyringCredentialStore) Set(kind string, profile string, secret string) error {
	return keyring.Set(credentialService(kind), profile, secret)
}

func (KeyringCredentialStore) Get(kind string, profile string) (string, error) {
	secret, err := keyring.Get(credentialService(kind), profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", auth.ErrCredentialNotFound
	}
	return secret, err
}

func (KeyringCredentialStore) Delete(kind string, profile string) error {
	err := keyring.Delete(credentialService(kind), profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func credentialService(kind string) string {
	return "bcli." + kind
}
