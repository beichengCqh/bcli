package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
)

type profileView struct {
	Kind          string                  `json:"kind"`
	Name          string                  `json:"name"`
	Profile       profile.ExternalProfile `json:"profile"`
	HasCredential bool                    `json:"hasCredential"`
}

func (r Runner) runProfile(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		r.printProfileHelp()
		return 0
	}

	switch args[0] {
	case "list":
		return r.runProfileList(args[1:])
	case "get":
		return r.runProfileGet(args[1:])
	case "set":
		return r.runProfileSet(args[1:])
	case "delete":
		return r.runProfileDelete(args[1:])
	default:
		fmt.Fprintf(r.stderr, "unknown profile command: %s\n\n", args[0])
		r.printProfileHelp()
		return 2
	}
}

func (r Runner) printProfileHelp() {
	fmt.Fprintf(r.stdout, `Usage:
  %s profile list [--json]
  %s profile get <mysql|redis> <name> [--json]
  %s profile set <mysql|redis> <name> [--host host] [--port port] [--user user] [--database database] [--executable path] [--arg value...]
  %s profile delete <mysql|redis> <name>
`, appName, appName, appName, appName)
}

func (r Runner) runProfileList(args []string) int {
	jsonOutput, rest, err := parseJSONFlag(args)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}
	if len(rest) != 0 {
		fmt.Fprintf(r.stderr, "usage: %s profile list [--json]\n", appName)
		return 2
	}

	cfg, err := profile.NewService(r.profiles).LoadConfig()
	if err != nil {
		fmt.Fprintf(r.stderr, "load config: %v\n", err)
		return 1
	}
	views := r.profileViews(cfg)
	if jsonOutput {
		if err := writeJSON(r.stdout, views); err != nil {
			fmt.Fprintf(r.stderr, "write json: %v\n", err)
			return 1
		}
		return 0
	}
	if len(views) == 0 {
		fmt.Fprintln(r.stdout, "no profiles")
		return 0
	}
	for _, view := range views {
		status := "no auth"
		if view.HasCredential {
			status = "auth"
		}
		fmt.Fprintf(r.stdout, "%s/%s %s:%d user=%s database=%s %s\n",
			view.Kind, view.Name, view.Profile.Host, view.Profile.Port, view.Profile.User, view.Profile.Database, status)
	}
	return 0
}

func (r Runner) runProfileGet(args []string) int {
	jsonOutput, rest, err := parseJSONFlag(args)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}
	if len(rest) != 2 || !profile.IsSupportedKind(rest[0]) {
		fmt.Fprintf(r.stderr, "usage: %s profile get <mysql|redis> <name> [--json]\n", appName)
		return 2
	}

	kind, name := rest[0], rest[1]
	cfg, err := profile.NewService(r.profiles).LoadConfig()
	if err != nil {
		fmt.Fprintf(r.stderr, "load config: %v\n", err)
		return 1
	}
	p, err := cfg.ExternalProfile(kind, name)
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}
	view := r.profileView(kind, name, p)
	if jsonOutput {
		if err := writeJSON(r.stdout, view); err != nil {
			fmt.Fprintf(r.stderr, "write json: %v\n", err)
			return 1
		}
		return 0
	}
	status := "no auth"
	if view.HasCredential {
		status = "auth"
	}
	fmt.Fprintf(r.stdout, "%s/%s %s:%d user=%s database=%s %s\n",
		view.Kind, view.Name, view.Profile.Host, view.Profile.Port, view.Profile.User, view.Profile.Database, status)
	return 0
}

func (r Runner) runProfileSet(args []string) int {
	if len(args) < 2 || !profile.IsSupportedKind(args[0]) {
		fmt.Fprintf(r.stderr, "usage: %s profile set <mysql|redis> <name> [flags...]\n", appName)
		return 2
	}

	kind, name := args[0], args[1]
	p, err := parseProfileSetFlags(args[2:])
	if err != nil {
		fmt.Fprintln(r.stderr, err)
		return 2
	}
	if err := profile.NewService(r.profiles).Set(kind, name, p); err != nil {
		fmt.Fprintf(r.stderr, "save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(r.stdout, "%s profile %q saved\n", kind, profile.NormalizeName(name))
	return 0
}

func (r Runner) runProfileDelete(args []string) int {
	if len(args) != 2 || !profile.IsSupportedKind(args[0]) {
		fmt.Fprintf(r.stderr, "usage: %s profile delete <mysql|redis> <name>\n", appName)
		return 2
	}
	kind, name := args[0], args[1]
	if err := profile.NewService(r.profiles).Delete(kind, name); err != nil {
		fmt.Fprintf(r.stderr, "save config: %v\n", err)
		return 1
	}
	if err := auth.NewService(r.credentials).DeleteCredential(kind, name); err != nil {
		fmt.Fprintf(r.stderr, "delete credential: %v\n", err)
		return 1
	}
	fmt.Fprintf(r.stdout, "%s profile %q deleted\n", kind, profile.NormalizeName(name))
	return 0
}

func parseJSONFlag(args []string) (bool, []string, error) {
	jsonOutput := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "--json=") {
				return false, nil, fmt.Errorf("--json does not accept a value")
			}
			rest = append(rest, arg)
		}
	}
	return jsonOutput, rest, nil
}

func parseProfileSetFlags(args []string) (profile.ExternalProfile, error) {
	var p profile.ExternalProfile
	for i := 0; i < len(args); i++ {
		arg := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", arg)
			}
			i++
			return args[i], nil
		}

		switch arg {
		case "--host":
			value, err := next()
			if err != nil {
				return p, err
			}
			p.Host = value
		case "--port":
			value, err := next()
			if err != nil {
				return p, err
			}
			port, err := strconv.Atoi(value)
			if err != nil || port < 0 {
				return p, fmt.Errorf("--port must be a non-negative integer")
			}
			p.Port = port
		case "--user":
			value, err := next()
			if err != nil {
				return p, err
			}
			p.User = value
		case "--database":
			value, err := next()
			if err != nil {
				return p, err
			}
			p.Database = value
		case "--executable":
			value, err := next()
			if err != nil {
				return p, err
			}
			p.Executable = value
		case "--arg":
			value, err := next()
			if err != nil {
				return p, err
			}
			p.Args = append(p.Args, value)
		default:
			return p, fmt.Errorf("unknown profile flag: %s", arg)
		}
	}
	return p, nil
}

func (r Runner) profileViews(cfg profile.Config) []profileView {
	views := make([]profileView, 0, len(cfg.MySQL)+len(cfg.Redis))
	for _, kind := range []string{"mysql", "redis"} {
		profiles := cfg.MySQL
		if kind == "redis" {
			profiles = cfg.Redis
		}
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			views = append(views, r.profileView(kind, name, profiles[name]))
		}
	}
	return views
}

func (r Runner) profileView(kind string, name string, p profile.ExternalProfile) profileView {
	hasCredential, err := auth.NewService(r.credentials).HasCredential(kind, name)
	if err != nil {
		hasCredential = false
	}
	return profileView{
		Kind:          kind,
		Name:          profile.NormalizeName(name),
		Profile:       p,
		HasCredential: hasCredential,
	}
}
