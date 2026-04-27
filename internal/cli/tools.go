package cli

import (
	"fmt"
	"strings"

	"bcli/internal/core/tools"
)

func (r Runner) runTools(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		r.printToolsHelp()
		return 0
	}

	switch args[0] {
	case "uuid":
		value, err := tools.UUID()
		if err != nil {
			fmt.Fprintf(r.stderr, "uuid: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.stdout, value)
		return 0
	case "now":
		fmt.Fprintln(r.stdout, tools.Now())
		return 0
	case "urlencode":
		input := strings.Join(args[1:], " ")
		fmt.Fprintln(r.stdout, tools.URLEncode(input))
		return 0
	case "urldecode":
		input := strings.Join(args[1:], " ")
		value, err := tools.URLDecode(input)
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
		fmt.Fprintln(r.stdout, tools.SHA256(input))
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown tools command: %s\n\n", args[0])
		r.printToolsHelp()
		return 2
	}
}

func (r Runner) printToolsHelp() {
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

func (r Runner) runBase64(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(r.stderr, "usage: %s tools base64 <encode|decode> <text>\n", appName)
		return 2
	}

	mode := args[0]
	input := strings.Join(args[1:], " ")

	switch mode {
	case "encode":
		fmt.Fprintln(r.stdout, tools.Base64Encode(input))
		return 0
	case "decode":
		value, err := tools.Base64Decode(input)
		if err != nil {
			fmt.Fprintf(r.stderr, "base64 decode: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.stdout, value)
		return 0
	default:
		fmt.Fprintf(r.stderr, "unknown base64 mode: %s\n", mode)
		return 2
	}
}
