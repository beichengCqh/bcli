package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"bcli/internal/core/auth"
	"bcli/internal/core/external"
	"bcli/internal/core/profile"
	"bcli/internal/mcp"
	"bcli/internal/storage"
	"bcli/internal/tui"
)

const appName = "bcli"

type Runner struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	credentials auth.Store
	profiles    profile.Store
}

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	r := NewRunner(stdin, stdout, stderr, storage.KeyringCredentialStore{}, storage.ConfigStore{})
	return r.Run(args)
}

func NewRunner(stdin io.Reader, stdout io.Writer, stderr io.Writer, credentials auth.Store, profiles profile.Store) Runner {
	return Runner{
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		credentials: credentials,
		profiles:    profiles,
	}
}

func (r Runner) Run(args []string) int {
	if len(args) == 0 {
		return r.runTUI()
	}

	switch args[0] {
	case "help", "-h", "--help":
		r.printHelp()
		return 0
	case "auth":
		return r.runAuth(args[1:])
	case "init":
		return r.runInit(args[1:])
	case "mysql":
		return r.runExternal("mysql", args[1:])
	case "redis":
		return r.runExternal("redis", args[1:])
	case "profile":
		return r.runProfile(args[1:])
	case "tui":
		return r.runTUI()
	case "mcp":
		return r.runMCP(args[1:])
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

func (r Runner) printHelp() {
	fmt.Fprintf(r.stdout, `%s is a personal command center.

Usage:
  %s
  %s auth <mysql|redis> [--profile name] [password]
  %s init
  %s profile <list|get|set|delete> [args...]
  %s mysql [--profile name] [-- mysql args...]
  %s redis [--profile name] [-- redis-cli args...]
  %s tui
  %s mcp serve
  %s tools <command> [args...]

Commands:
  auth        Store credentials for a connection profile
  init        Choose and configure external CLI clients
  profile     Manage non-sensitive connection profile config
  mysql       Run mysql client with an optional configured profile
  redis       Run redis-cli with an optional configured profile
  tui         Manage connection profiles in a terminal UI
  mcp         Run the MCP server
  tools       Small personal utilities
  version     Print version

Examples:
  %s
  %s init
  %s auth mysql --profile local
  %s profile list --json
  %s mysql --profile local -- -e "select 1"
  %s auth redis --profile cache
  %s redis --profile cache -- ping
  %s tui
  %s tools uuid
  %s tools base64 encode hello
`, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName, appName)
}

func (r Runner) runTUI() int {
	authService := auth.NewService(r.credentials)
	profileService := profile.NewService(r.profiles)
	cfg, err := profileService.LoadConfig()
	if err != nil {
		fmt.Fprintf(r.stderr, "load config: %v\n", err)
		return 1
	}
	if !cfg.HasClientConfig() {
		if err := r.runInitWizard(defaultCommandRunner); err != nil {
			fmt.Fprintf(r.stderr, "init: %v\n", err)
			return 1
		}
	}
	return tui.Run(profileService, authService, r.stderr)
}

func (r Runner) runExternal(kind string, args []string) int {
	profileName, rest, err := parseProfileArgs(args)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}

	service := external.NewService(profile.NewService(r.profiles), auth.NewService(r.credentials), r.stdin, r.stdout, r.stderr)
	return service.Run(kind, profileName, rest)
}

func parseProfileArgs(args []string) (string, []string, error) {
	selected := ""
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			rest = append(rest, args[i+1:]...)
			return selected, rest, nil
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--profile requires a value")
			}
			selected = args[i+1]
			i++
		case strings.HasPrefix(arg, "--profile="):
			selected = strings.TrimPrefix(arg, "--profile=")
			if selected == "" {
				return "", nil, fmt.Errorf("--profile requires a value")
			}
		default:
			rest = append(rest, arg)
		}
	}

	return selected, rest, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (r Runner) runMCP(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintf(r.stdout, "Usage:\n  %s mcp serve\n", appName)
		return 0
	}
	if len(args) == 1 && args[0] == "serve" {
		server := mcp.NewServer(r.stdin, r.stdout, r.stderr, auth.NewService(r.credentials), profile.NewService(r.profiles))
		if err := server.Serve(); err != nil {
			fmt.Fprintln(r.stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(r.stderr, "usage: %s mcp serve\n", appName)
	return 2
}
