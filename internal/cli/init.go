package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"bcli/internal/core/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type commandRunner func(stdin io.Reader, stdout io.Writer, stderr io.Writer, name string, args ...string) error

type initScreen int

const (
	initSelectClients initScreen = iota
	initSelectInstalls
	initDone
)

type initClientOption struct {
	kind       string
	title      string
	selected   bool
	install    bool
	executable string
}

type initModel struct {
	screen   initScreen
	cursor   int
	options  []initClientOption
	canceled bool
}

var (
	initTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	initHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	initSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	initMutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	initPanelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
)

func (r Runner) runInit(args []string) int {
	if len(args) != 0 && args[0] != "-h" && args[0] != "--help" && args[0] != "help" {
		fmt.Fprintf(r.stderr, "usage: %s init\n", appName)
		return 2
	}
	if len(args) != 0 {
		fmt.Fprintf(r.stdout, "Usage:\n  %s init\n", appName)
		return 0
	}
	if err := r.runInitWizard(defaultCommandRunner); err != nil {
		fmt.Fprintf(r.stderr, "init: %v\n", err)
		return 1
	}
	return 0
}

func (r Runner) runInitWizard(runCommand commandRunner) error {
	cfg, err := profile.NewService(r.profiles).LoadConfig()
	if err != nil {
		return err
	}

	initial := newInitModel(cfg)
	finalModel, err := tea.NewProgram(initial, tea.WithInput(r.stdin), tea.WithOutput(r.stdout), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(initModel)
	if !ok || m.canceled {
		return nil
	}
	return r.applyInitModel(m, runCommand)
}

func (r Runner) applyInitModel(m initModel, runCommand commandRunner) error {
	cfg, err := profile.NewService(r.profiles).LoadConfig()
	if err != nil {
		return err
	}

	for _, option := range m.options {
		client := profile.ClientConfig{Enabled: option.selected, Executable: option.executable}
		if option.selected && option.install {
			command, ok := profile.InstallCommand(option.kind)
			if ok {
				if err := runCommand(r.stdin, r.stdout, r.stderr, command[0], command[1:]...); err != nil {
					return err
				}
				if executable, ok := profile.DetectExecutable(option.kind); ok {
					client.Executable = executable
				}
			} else {
				fmt.Fprintf(r.stdout, "No automatic installer is available for %s on this system.\n", option.kind)
			}
		}
		cfg.SetClient(option.kind, client)
	}

	if err := profile.NewService(r.profiles).SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "bcli init completed.")
	return nil
}

func newInitModel(cfg profile.Config) initModel {
	options := []initClientOption{
		newInitClientOption(cfg, "mysql", "MySQL client"),
		newInitClientOption(cfg, "redis", "Redis client"),
	}
	return initModel{screen: initSelectClients, options: options}
}

func newInitClientOption(cfg profile.Config, kind string, title string) initClientOption {
	client := cfg.Client(kind)
	executable := client.Executable
	if executable == "" {
		if detected, ok := profile.DetectExecutable(kind); ok {
			executable = detected
		}
	}
	return initClientOption{
		kind:       kind,
		title:      title,
		selected:   client.Enabled,
		executable: executable,
	}
}

func (m initModel) Init() tea.Cmd {
	return nil
}

func (m initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "esc", "q":
		m.canceled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visibleOptions())-1 {
			m.cursor++
		}
	case " ":
		m.toggleCurrent()
	case "enter":
		return m.advance()
	}
	return m, nil
}

func (m initModel) View() string {
	switch m.screen {
	case initSelectInstalls:
		return m.viewInstallSelect()
	default:
		return m.viewClientSelect()
	}
}

func (m initModel) viewClientSelect() string {
	var b strings.Builder
	b.WriteString(initTitleStyle.Render("bcli init"))
	b.WriteString("\n")
	b.WriteString(initHelpStyle.Render("↑/↓ move  space select  enter continue  esc cancel"))
	b.WriteString("\n\n")

	for i, option := range m.options {
		line := checkbox(option.selected) + " " + option.title
		if option.executable != "" {
			line += "  " + initMutedStyle.Render(option.executable)
		} else {
			line += "  " + initMutedStyle.Render("not installed")
		}
		b.WriteString(renderInitLine(i == m.cursor, line))
		b.WriteString("\n")
	}
	return initPanelStyle.Render(b.String())
}

func (m initModel) viewInstallSelect() string {
	var b strings.Builder
	b.WriteString(initTitleStyle.Render("install missing clients"))
	b.WriteString("\n")
	b.WriteString(initHelpStyle.Render("↑/↓ move  space select install  enter confirm  esc cancel"))
	b.WriteString("\n\n")

	missing := m.missingSelectedOptions()
	for i, option := range missing {
		line := checkbox(option.install) + " " + option.title + "  " + initMutedStyle.Render(profile.InstallHint(option.kind))
		b.WriteString(renderInitLine(i == m.cursor, line))
		b.WriteString("\n")
	}
	return initPanelStyle.Render(b.String())
}

func (m initModel) advance() (tea.Model, tea.Cmd) {
	switch m.screen {
	case initSelectClients:
		if len(m.missingSelectedOptions()) > 0 {
			m.screen = initSelectInstalls
			m.cursor = 0
			return m, nil
		}
		m.screen = initDone
		return m, tea.Quit
	case initSelectInstalls:
		m.screen = initDone
		return m, tea.Quit
	default:
		return m, tea.Quit
	}
}

func (m initModel) toggleCurrent() {
	switch m.screen {
	case initSelectClients:
		if m.cursor >= 0 && m.cursor < len(m.options) {
			m.options[m.cursor].selected = !m.options[m.cursor].selected
		}
	case initSelectInstalls:
		missingIndexes := m.missingSelectedIndexes()
		if m.cursor >= 0 && m.cursor < len(missingIndexes) {
			index := missingIndexes[m.cursor]
			m.options[index].install = !m.options[index].install
		}
	}
}

func (m initModel) visibleOptions() []initClientOption {
	if m.screen == initSelectInstalls {
		return m.missingSelectedOptions()
	}
	return m.options
}

func (m initModel) missingSelectedOptions() []initClientOption {
	indexes := m.missingSelectedIndexes()
	options := make([]initClientOption, 0, len(indexes))
	for _, index := range indexes {
		options = append(options, m.options[index])
	}
	return options
}

func (m initModel) missingSelectedIndexes() []int {
	indexes := []int{}
	for i, option := range m.options {
		if option.selected && option.executable == "" {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func checkbox(selected bool) string {
	if selected {
		return "[x]"
	}
	return "[ ]"
}

func renderInitLine(selected bool, line string) string {
	if selected {
		return initSelectedStyle.Render(line)
	}
	return "  " + line
}

func defaultCommandRunner(stdin io.Reader, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}
