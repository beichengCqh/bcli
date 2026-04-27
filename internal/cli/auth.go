package cli

import (
	"fmt"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
)

func (r Runner) runAuth(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		r.printAuthHelp()
		return 0
	}

	kind := args[0]
	if !profile.IsSupportedKind(kind) {
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

func (r Runner) printAuthHelp() {
	fmt.Fprintf(r.stdout, `Usage:
  %s auth mysql [--profile name] [password]
  %s auth redis [--profile name] [password]

Examples:
  %s auth mysql --profile local
  %s auth redis --profile cache "password"
`, appName, appName, appName, appName)
}

func (r Runner) runExternalAuth(kind string, profileName string, args []string) int {
	if len(args) > 1 {
		fmt.Fprintf(r.stderr, "usage: %s auth %s [--profile name] [password]\n", appName, kind)
		return 2
	}

	secret := ""
	if len(args) == 1 {
		secret = args[0]
	} else {
		var err error
		secret, err = auth.ReadSecretFromTerminal(fmt.Sprintf("%s auth %s password for profile %q: ", appName, kind, auth.NormalizeProfileName(profileName)))
		if err != nil {
			fmt.Fprintf(r.stderr, "read password: %v\n", err)
			return 1
		}
	}

	service := auth.NewService(r.credentials)
	if secret == "" {
		fmt.Fprintln(r.stderr, "password cannot be empty")
		return 2
	}
	if err := service.StoreCredential(kind, profileName, secret); err != nil {
		fmt.Fprintf(r.stderr, "store credential: %v\n", err)
		return 1
	}

	fmt.Fprintf(r.stdout, "%s credential stored for profile %q\n", kind, auth.NormalizeProfileName(profileName))
	return 0
}
