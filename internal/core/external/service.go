package external

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
)

type Service struct {
	profiles    profile.Service
	auth        auth.Service
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	commandFunc func(name string, arg ...string) *exec.Cmd
}

const connectionTestTimeout = 5 * time.Second

func NewService(profiles profile.Service, authService auth.Service, stdin io.Reader, stdout io.Writer, stderr io.Writer) Service {
	return Service{
		profiles:    profiles,
		auth:        authService,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		commandFunc: exec.Command,
	}
}

func (s Service) Run(kind string, profileName string, args []string) int {
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		fmt.Fprintf(s.stderr, "load config: %v\n", err)
		return 1
	}

	p, err := cfg.ExternalProfile(kind, profileName)
	if err != nil {
		fmt.Fprintln(s.stderr, err)
		return 2
	}

	executable, ok := profile.ResolveExecutableWithConfig(kind, cfg, p.Executable)
	if !ok {
		executable = profile.DefaultExecutable(kind)
	}

	cmdArgs := p.CommandArgs(kind)
	cmdArgs = append(cmdArgs, args...)
	if kind == "mysql" && p.Database != "" {
		cmdArgs = append(cmdArgs, p.Database)
	}

	cmd := s.commandFunc(executable, cmdArgs...)
	cmd.Stdin = s.stdin
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
	cmd.Env = s.Env(kind, profileName)

	err = cmd.Run()
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	fmt.Fprintf(s.stderr, "run %s: %v\n", executable, err)
	return 1
}

func (s Service) Env(kind string, profileName string) []string {
	return s.envWithSecret(kind, profileName, "")
}

func (s Service) TestConnection(kind string, profileName string, p profile.ExternalProfile, secretOverride string) error {
	cfg, err := s.profiles.LoadConfig()
	if err != nil {
		return err
	}
	executable, ok := profile.ResolveExecutableWithConfig(kind, cfg, p.Executable)
	if !ok {
		return errors.New(profile.InstallHint(kind))
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTestTimeout)
	defer cancel()

	cmdArgs := testArgs(kind, p)
	cmd := exec.CommandContext(ctx, executable, cmdArgs...)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = s.envWithSecret(kind, profileName, secretOverride)

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("connection test timed out after %s", connectionTestTimeout)
		}
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

func testArgs(kind string, p profile.ExternalProfile) []string {
	args := p.CommandArgs(kind)
	switch kind {
	case "mysql":
		args = append(args, "-e", "select 1")
		if p.Database != "" {
			args = append(args, p.Database)
		}
	case "redis":
		args = append(args, "ping")
	}
	return args
}

func (s Service) envWithSecret(kind string, profileName string, secretOverride string) []string {
	env := os.Environ()
	secret := secretOverride
	if secret == "" {
		var err error
		secret, err = s.auth.Credential(kind, profileName)
		if err != nil {
			if !errors.Is(err, auth.ErrCredentialNotFound) {
				fmt.Fprintf(s.stderr, "warning: read credential: %v\n", err)
			}
			return env
		}
	}
	if secret == "" {
		return env
	}

	switch kind {
	case "mysql":
		return append(env, "MYSQL_PWD="+secret)
	case "redis":
		return append(env, "REDISCLI_AUTH="+secret)
	default:
		return env
	}
}
