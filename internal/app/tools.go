package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (r runner) runTools(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		r.printToolsHelp()
		return 0
	}

	switch args[0] {
	case "uuid":
		value, err := newUUID()
		if err != nil {
			fmt.Fprintf(r.stderr, "uuid: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.stdout, value)
		return 0
	case "now":
		fmt.Fprintln(r.stdout, time.Now().Format(time.RFC3339))
		return 0
	case "urlencode":
		input := strings.Join(args[1:], " ")
		fmt.Fprintln(r.stdout, url.QueryEscape(input))
		return 0
	case "urldecode":
		input := strings.Join(args[1:], " ")
		value, err := url.QueryUnescape(input)
		if err != nil {
			fmt.Fprintf(r.stderr, "urldecode: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.stdout, value)
		return 0
	case "base64":
		return r.runBase64(args[1:])
	case "sha256":
		input := strings.Join(args[1:], " ")
		sum := sha256.Sum256([]byte(input))
		fmt.Fprintln(r.stdout, hex.EncodeToString(sum[:]))
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown tools command: %s\n\n", args[0])
		r.printToolsHelp()
		return 2
	}
}

func (r runner) printToolsHelp() {
	fmt.Fprintf(r.stdout, `Usage:
  %s tools uuid
  %s tools now
  %s tools urlencode <text>
  %s tools urldecode <text>
  %s tools base64 encode <text>
  %s tools base64 decode <text>
  %s tools sha256 <text>
`, appName, appName, appName, appName, appName, appName, appName)
}

func (r runner) runBase64(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(r.stderr, "usage: %s tools base64 <encode|decode> <text>\n", appName)
		return 2
	}

	mode := args[0]
	input := strings.Join(args[1:], " ")

	switch mode {
	case "encode":
		fmt.Fprintln(r.stdout, base64.StdEncoding.EncodeToString([]byte(input)))
		return 0
	case "decode":
		data, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			fmt.Fprintf(r.stderr, "base64 decode: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.stdout, string(data))
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown base64 mode: %s\n", mode)
		return 2
	}
}
