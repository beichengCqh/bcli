package app

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func normalizeProfileName(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func readSecretFromTerminal(prompt string) (string, error) {
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
