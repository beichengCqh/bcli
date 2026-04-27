package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var ErrCredentialNotFound = errors.New("credential not found")

type Store interface {
	Set(kind string, profile string, secret string) error
	Get(kind string, profile string) (string, error)
	Delete(kind string, profile string) error
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (s Service) StoreCredential(kind string, name string, secret string) error {
	if secret == "" {
		return errors.New("password cannot be empty")
	}
	return s.store.Set(kind, NormalizeProfileName(name), secret)
}

func (s Service) Credential(kind string, name string) (string, error) {
	return s.store.Get(kind, NormalizeProfileName(name))
}

func (s Service) DeleteCredential(kind string, name string) error {
	return s.store.Delete(kind, NormalizeProfileName(name))
}

func (s Service) HasCredential(kind string, name string) (bool, error) {
	secret, err := s.Credential(kind, name)
	if errors.Is(err, ErrCredentialNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return secret != "", nil
}

func NormalizeProfileName(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func ReadSecretFromTerminal(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer tty.Close()

	fmt.Fprint(tty, prompt)
	if err := runStty(tty, "-echo"); err != nil {
		return "", err
	}
	defer func() {
		_ = runStty(tty, "echo")
		fmt.Fprintln(tty)
	}()

	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func runStty(tty *os.File, arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	return cmd.Run()
}
