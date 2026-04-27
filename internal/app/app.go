package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const appName = "bcli"

type runner struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	credentials credentialStore
}

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	r := runner{
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		credentials: keyringCredentialStore{},
	}
	return r.run(args)
}

func runWithCredentialStore(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, credentials credentialStore) int {
	r := runner{
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		credentials: credentials,
	}
	return r.run(args)
}

func (r runner) run(args []string) int {
	if len(args) == 0 {
		return r.runTUI()
	}

	switch args[0] {
	case "help", "-h", "--help":
		r.printHelp()
		return 0
	case "auth":
		return r.runAuth(args[1:])
	case "mysql":
		return r.runExternal("mysql", args[1:])
	case "redis":
		return r.runExternal("redis", args[1:])
	case "tui":
		return r.runTUI()
	case "tools":
		return r.runTools(args[1:])
	case "version":
		fmt.Fprintln(r.stdout, "bcli dev")
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown command: %s\n\n", args[0])
		r.printHelp()
		return 2
	}
}

func (r runner) printHelp() {
	fmt.Fprintf(r.stdout, `%s is a personal command center.

Usage:
  %s
  %s auth <mysql|redis> [--profile name] [password]
  %s mysql [--profile name] [-- mysql args...]
  %s redis [--profile name] [-- redis-cli args...]
  %s tui
  %s tools <command> [args...]

Commands:
  auth        Store credentials for a connection profile
  mysql       Run mysql client with an optional configured profile
  redis       Run redis-cli with an optional configured profile
  tui         Manage connection profiles in a terminal UI
  tools       Small personal utilities
  version     Print version

Examples:
  %s
  %s auth mysql --profile local
  %s mysql --profile local -- -e "select 1"
  %s auth redis --profile cache
  %s redis --profile cache -- ping
  %s tui
  %s tools uuid
  %s tools base64 encode hello
`, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName)
}

func (r runner) runAuth(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		r.printAuthHelp()
		return 0
	}

	kind := args[0]
	if kind != "mysql" && kind != "redis" {
		fmt.Fprintf(r.stderr, "unknown auth target: %s\n\n", kind)
		r.printAuthHelp()
		return 2
	}

	profileName, rest, err := parseProfileArgs(args[1:])
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}
	return r.runExternalAuth(kind, profileName, rest)
}

func (r runner) printAuthHelp() {
	fmt.Fprintf(r.stdout, `Usage:
  %s auth mysql [--profile name] [password]
  %s auth redis [--profile name] [password]

Examples:
  %s auth mysql --profile local
  %s auth redis --profile cache "password"
`, appName, appName, appName, appName)
}

func (r runner) runExternal(kind string, args []string) int {
	profileName, rest, err := parseProfileArgs(args)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(r.stderr, "load config: %v\n", err)
		return 1
	}

	profile, err := cfg.ExternalProfile(kind, profileName)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}

	executable := profile.Executable
	if executable == "" {
		executable = defaultExecutable(kind)
	}

	cmdArgs := profile.CommandArgs(kind)
	cmdArgs = append(cmdArgs, rest...)
	if kind == "mysql" && profile.Database != "" {
		cmdArgs = append(cmdArgs, profile.Database)
	}

	cmd := exec.Command(executable, cmdArgs...)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	cmd.Env = r.externalEnv(kind, profileName)

	err = cmd.Run()
	if err == nil {
		return 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}

	fmt.Fprintf(r.stderr, "run %s: %v\n", executable, err)
	return 1
}

func (r runner) runExternalAuth(kind string, profileName string, args []string) int {
	if len(args) > 1 {
		fmt.Fprintf(r.stderr, "usage: %s auth %s [--profile name] [password]\n", appName, kind)
		return 2
	}

	secret := ""
	if len(args) == 1 {
		secret = args[0]
	} else {
		var err error
		secret, err = readSecretFromTerminal(fmt.Sprintf("%s auth %s password for profile %q: ", appName, kind, normalizeProfileName(profileName)))
		if err != nil {
			fmt.Fprintf(r.stderr, "read password: %v\n", err)
			return 1
		}
	}
	if secret == "" {
		fmt.Fprintln(r.stderr, "password cannot be empty")
		return 2
	}

	if err := r.credentials.Set(kind, normalizeProfileName(profileName), secret); err != nil {
		fmt.Fprintf(r.stderr, "store credential: %v\n", err)
		return 1
	}

	fmt.Fprintf(r.stdout, "%s credential stored for profile %q\n", kind, normalizeProfileName(profileName))
	return 0
}

func (r runner) externalEnv(kind string, profileName string) []string {
	env := os.Environ()
	secret, err := r.credentials.Get(kind, normalizeProfileName(profileName))
	if err != nil {
		if err != errCredentialNotFound {
			fmt.Fprintf(r.stderr, "warning: read credential: %v\n", err)
		}
		return env
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

func parseProfileArgs(args []string) (string, []string, error) {
	profile := ""
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			rest = append(rest, args[i+1:]...)
			return profile, rest, nil
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--profile requires a value")
			}
			profile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--profile="):
			profile = strings.TrimPrefix(arg, "--profile=")
			if profile == "" {
				return "", nil, fmt.Errorf("--profile requires a value")
			}
		default:
			rest = append(rest, arg)
		}
	}

	return profile, rest, nil
}

func defaultExecutable(kind string) string {
	if kind == "redis" {
		return "redis-cli"
	}
	return kind
}
