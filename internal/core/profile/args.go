package profile

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func (p ExternalProfile) CommandArgs(kind string) []string {
	args := make([]string, 0, len(p.Args)+8)
	switch kind {
	case "mysql":
		if p.Host != "" {
			args = append(args, "-h", p.Host)
		}
		if p.Port != 0 {
			args = append(args, "-P", fmt.Sprintf("%d", p.Port))
		}
		if p.User != "" {
			args = append(args, "-u", p.User)
		}
		args = append(args, p.Args...)
	case "redis":
		if p.Host != "" {
			args = append(args, "-h", p.Host)
		}
		if p.Port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", p.Port))
		}
		if p.User != "" {
			args = append(args, "--user", p.User)
		}
		if p.Database != "" {
			args = append(args, "-n", p.Database)
		}
		args = append(args, p.Args...)
	default:
		args = append(args, p.Args...)
	}
	return args
}

func DefaultExecutable(kind string) string {
	if kind == "redis" {
		return "redis-cli"
	}
	return kind
}

func DetectExecutable(kind string) (string, bool) {
	for _, candidate := range executableCandidates(kind) {
		if path, ok := resolveExecutableCandidate(candidate); ok {
			return path, true
		}
	}
	return "", false
}

func ResolveExecutable(kind string, configured string) (string, bool) {
	if configured != "" {
		if path, ok := resolveExecutableCandidate(configured); ok {
			return path, true
		}
		return configured, false
	}
	return DetectExecutable(kind)
}

func ResolveExecutableWithConfig(kind string, cfg Config, configured string) (string, bool) {
	if configured != "" {
		return ResolveExecutable(kind, configured)
	}
	client := cfg.Client(kind)
	if client.Enabled && client.Executable != "" {
		return ResolveExecutable(kind, client.Executable)
	}
	return DetectExecutable(kind)
}

func InstallCommand(kind string) ([]string, bool) {
	switch runtime.GOOS {
	case "darwin":
		switch kind {
		case "mysql":
			return []string{"brew", "install", "mysql-client"}, true
		case "redis":
			return []string{"brew", "install", "redis"}, true
		}
	case "linux":
		if _, err := exec.LookPath("apt"); err == nil {
			switch kind {
			case "mysql":
				return []string{"sudo", "apt", "install", "-y", "mysql-client"}, true
			case "redis":
				return []string{"sudo", "apt", "install", "-y", "redis-tools"}, true
			}
		}
	}
	return nil, false
}

func InstallHint(kind string) string {
	switch runtime.GOOS {
	case "darwin":
		switch kind {
		case "mysql":
			return `mysql client not found. Install with: brew install mysql-client. If Homebrew asks, add it to PATH: echo 'export PATH="/opt/homebrew/opt/mysql-client/bin:$PATH"' >> ~/.zshrc`
		case "redis":
			return "redis-cli not found. Install with: brew install redis"
		}
	case "linux":
		switch kind {
		case "mysql":
			return "mysql client not found. Install mysql-client with your package manager, for example: sudo apt install mysql-client"
		case "redis":
			return "redis-cli not found. Install redis tools with your package manager, for example: sudo apt install redis-tools"
		}
	case "windows":
		switch kind {
		case "mysql":
			return "mysql client not found. Install MySQL Shell or MySQL client, then make mysql.exe available in PATH"
		case "redis":
			return "redis-cli not found. Install Redis CLI, then make redis-cli.exe available in PATH"
		}
	}
	return fmt.Sprintf("%s client not found. Install it and make %q available in PATH", kind, DefaultExecutable(kind))
}

func executableCandidates(kind string) []string {
	switch kind {
	case "mysql":
		return []string{
			"mysql",
			"/opt/homebrew/opt/mysql-client/bin/mysql",
			"/usr/local/opt/mysql-client/bin/mysql",
			"/opt/homebrew/bin/mysql",
			"/usr/local/bin/mysql",
		}
	case "redis":
		return []string{
			"redis-cli",
			"/opt/homebrew/bin/redis-cli",
			"/usr/local/bin/redis-cli",
			"/opt/homebrew/opt/redis/bin/redis-cli",
			"/usr/local/opt/redis/bin/redis-cli",
		}
	default:
		return []string{DefaultExecutable(kind)}
	}
}

func resolveExecutableCandidate(candidate string) (string, bool) {
	if candidate == "" {
		return "", false
	}
	if path, err := exec.LookPath(candidate); err == nil {
		return path, true
	}
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return "", false
	}
	return candidate, true
}
