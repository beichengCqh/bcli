package app

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

var errCredentialNotFound = errors.New("credential not found")

type credentialStore interface {
	Set(kind string, profile string, secret string) error
	Get(kind string, profile string) (string, error)
}

type keyringCredentialStore struct{}

func (keyringCredentialStore) Set(kind string, profile string, secret string) error {
	return keyring.Set(credentialService(kind), normalizeProfileName(profile), secret)
}

func (keyringCredentialStore) Get(kind string, profile string) (string, error) {
	secret, err := keyring.Get(credentialService(kind), normalizeProfileName(profile))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errCredentialNotFound
	}
	if err != nil {
		return "", err
	}
	return secret, nil
}

func credentialService(kind string) string {
	return fmt.Sprintf("%s.%s", appName, kind)
}
