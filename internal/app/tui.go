package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiScreen int

const (
	tuiList tuiScreen = iota
	tuiFormScreen
	tuiConfirmDelete
)

type tuiEntry struct {
	kind      string
	name      string
	profile   ExternalProfile
	hasSecret bool
}

type tuiModel struct {
	credentials credentialStore
	cfg         Config
	entries     []tuiEntry
	selected    int
	screen      tuiScreen
	message     string

	editKind string
	editName string
	form     tuiForm
}

type tuiForm struct {
	field      int
	kind       string
	name       string
	host       string
	port       string
	user       string
	database   string
	executable string
	args       string
	password   string
}

var (
	tuiTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	tuiHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	tuiSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	tuiMutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tuiErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	tuiOKStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	tuiPanelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
)

func (r runner) runTUI() int {
	model, err := newTUIModel(r.credentials)
	if err != nil {
		fmt.Fprintf(r.stderr, "load config: %v\n", err)
		return 1
	}

	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(r.stderr, "run tui: %v\n", err)
		return 1
	}
	return 0
}

func newTUIModel(credentials credentialStore) (tuiModel, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return tuiModel{}, err
	}
	m := tuiModel{credentials: credentials, cfg: cfg}
	m.refreshEntries()
	return m, nil
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case tuiFormScreen:
		return m.updateForm(key), nil
	case tuiConfirmDelete:
		return m.updateConfirmDelete(key), nil
	default:
		next, quit := m.updateList(key)
		if quit {
			return next, tea.Quit
		}
		return next, nil
	}
}

func (m tuiModel) View() string {
	switch m.screen {
	case tuiFormScreen:
		return m.viewForm()
	case tuiConfirmDelete:
		return m.viewConfirmDelete()
	default:
		return m.viewList()
	}
}

func (m tuiModel) updateList(key tea.KeyMsg) (tuiModel, bool) {
	switch key.String() {
	case "q", "ctrl+c", "esc":
		return m, true
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.entries)-1 {
			m.selected++
		}
	case "a":
		m.screen = tuiFormScreen
		m.message = ""
		m.form = tuiForm{kind: "mysql", name: nextProfileName(m.cfg, "mysql")}
		m.editKind = ""
		m.editName = ""
	case "e", "enter":
		if len(m.entries) == 0 {
			m.message = "no profile to edit"
			return m, false
		}
		entry := m.entries[m.selected]
		m.screen = tuiFormScreen
		m.message = ""
		m.editKind = entry.kind
		m.editName = entry.name
		m.form = formFromEntry(entry)
	case "d":
		if len(m.entries) == 0 {
			m.message = "no profile to delete"
			return m, false
		}
		m.screen = tuiConfirmDelete
		m.message = ""
	}
	return m, false
}

func (m tuiModel) updateConfirmDelete(key tea.KeyMsg) tuiModel {
	switch key.String() {
	case "y", "Y":
		if len(m.entries) == 0 {
			m.screen = tuiList
			return m
		}
		entry := m.entries[m.selected]
		m.cfg.DeleteExternalProfile(entry.kind, entry.name)
		if err := SaveConfig(m.cfg); err != nil {
			m.message = "save config: " + err.Error()
			m.screen = tuiList
			return m
		}
		if err := m.credentials.Delete(entry.kind, entry.name); err != nil {
			m.message = "delete credential: " + err.Error()
			m.screen = tuiList
			return m
		}
		m.message = fmt.Sprintf("deleted %s/%s", entry.kind, entry.name)
		m.refreshEntries()
		if m.selected >= len(m.entries) && m.selected > 0 {
			m.selected--
		}
		m.screen = tuiList
	case "n", "N", "esc", "q", "ctrl+c":
		m.screen = tuiList
	}
	return m
}

func (m tuiModel) updateForm(key tea.KeyMsg) tuiModel {
	switch key.String() {
	case "esc":
		m.screen = tuiList
		m.message = ""
		return m
	case "tab", "down":
		m.form.field = (m.form.field + 1) % 9
		return m
	case "shift+tab", "up":
		m.form.field = (m.form.field + 8) % 9
		return m
	case "enter":
		return m.saveForm()
	case "left", "right", " ":
		if m.form.field == 0 {
			if m.form.kind == "mysql" {
				m.form.kind = "redis"
			} else {
				m.form.kind = "mysql"
			}
		}
		return m
	case "backspace", "ctrl+h":
		m.form.deleteLast()
		return m
	case "ctrl+u":
		m.form.clearField()
		return m
	}

	if m.form.field == 0 {
		return m
	}
	if len(key.Runes) > 0 {
		m.form.appendText(string(key.Runes))
	}
	return m
}

func (m tuiModel) saveForm() tuiModel {
	name := normalizeProfileName(strings.TrimSpace(m.form.name))
	if name == "" {
		m.message = "profile name cannot be empty"
		return m
	}
	port, err := parseOptionalPort(m.form.port)
	if err != nil {
		m.message = err.Error()
		return m
	}

	profile := ExternalProfile{
		Executable: strings.TrimSpace(m.form.executable),
		Args:       strings.Fields(m.form.args),
		Host:       strings.TrimSpace(m.form.host),
		Port:       port,
		User:       strings.TrimSpace(m.form.user),
		Database:   strings.TrimSpace(m.form.database),
	}

	oldKind, oldName := m.editKind, m.editName
	deleteOldCredential := false
	if oldName != "" && (oldKind != m.form.kind || oldName != name) {
		if secret, err := m.credentials.Get(oldKind, oldName); err == nil && secret != "" {
			if err := m.credentials.Set(m.form.kind, name, secret); err != nil {
				m.message = "move credential: " + err.Error()
				return m
			}
			deleteOldCredential = true
		}
		m.cfg.DeleteExternalProfile(oldKind, oldName)
	}

	m.cfg.SetExternalProfile(m.form.kind, name, profile)
	if secret := strings.TrimSpace(m.form.password); secret != "" {
		if err := m.credentials.Set(m.form.kind, name, secret); err != nil {
			m.message = "store credential: " + err.Error()
			return m
		}
	}
	if err := SaveConfig(m.cfg); err != nil {
		m.message = "save config: " + err.Error()
		return m
	}
	if deleteOldCredential {
		_ = m.credentials.Delete(oldKind, oldName)
	}

	m.refreshEntries()
	m.selected = m.findEntry(m.form.kind, name)
	m.message = fmt.Sprintf("saved %s/%s", m.form.kind, name)
	m.screen = tuiList
	return m
}

func (m tuiModel) viewList() string {
	var b strings.Builder
	b.WriteString(tuiTitleStyle.Render("bcli profile manager"))
	b.WriteString("\n")
	b.WriteString(tuiHelpStyle.Render("↑/↓ select  a add  e edit  d delete  q quit"))
	b.WriteString("\n\n")

	if len(m.entries) == 0 {
		b.WriteString(tuiMutedStyle.Render("No profiles yet. Press a to create one."))
	} else {
		for i, entry := range m.entries {
			line := fmt.Sprintf("%-5s %-14s %s", entry.kind, entry.name, profileSummary(entry.profile))
			if entry.hasSecret {
				line += "  " + tuiOKStyle.Render("auth")
			} else {
				line += "  " + tuiMutedStyle.Render("no auth")
			}
			if i == m.selected {
				b.WriteString(tuiSelectedStyle.Render(line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	}

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(renderMessage(m.message))
	}
	return tuiPanelStyle.Render(b.String())
}

func (m tuiModel) viewForm() string {
	rows := []string{
		m.form.renderField(0, "Type", m.form.kind+"  (space toggles)"),
		m.form.renderField(1, "Name", m.form.name),
		m.form.renderField(2, "Host", m.form.host),
		m.form.renderField(3, "Port", m.form.port),
		m.form.renderField(4, "User", m.form.user),
		m.form.renderField(5, "Database/DB", m.form.database),
		m.form.renderField(6, "Executable", m.form.executable),
		m.form.renderField(7, "Extra args", m.form.args),
		m.form.renderField(8, "Password", mask(m.form.password)+"  (leave empty to keep current)"),
	}

	title := "add profile"
	if m.editName != "" {
		title = "edit profile"
	}
	body := tuiTitleStyle.Render(title) + "\n" +
		tuiHelpStyle.Render("tab/↑/↓ move  enter save  esc cancel  ctrl+u clear") + "\n\n" +
		strings.Join(rows, "\n")
	if m.message != "" {
		body += "\n\n" + renderMessage(m.message)
	}
	return tuiPanelStyle.Render(body)
}

func (m tuiModel) viewConfirmDelete() string {
	entry := m.entries[m.selected]
	body := tuiTitleStyle.Render("delete profile") + "\n\n" +
		fmt.Sprintf("Delete %s/%s and its stored credential?\n\n", entry.kind, entry.name) +
		tuiHelpStyle.Render("y confirm  n/esc cancel")
	return tuiPanelStyle.Render(body)
}

func (m *tuiModel) refreshEntries() {
	var entries []tuiEntry
	for _, kind := range []string{"mysql", "redis"} {
		profiles := m.cfg.MySQL
		if kind == "redis" {
			profiles = m.cfg.Redis
		}
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			_, err := m.credentials.Get(kind, name)
			entries = append(entries, tuiEntry{
				kind:      kind,
				name:      name,
				profile:   profiles[name],
				hasSecret: err == nil,
			})
		}
	}
	m.entries = entries
	if m.selected >= len(entries) {
		m.selected = len(entries) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m tuiModel) findEntry(kind string, name string) int {
	for i, entry := range m.entries {
		if entry.kind == kind && entry.name == name {
			return i
		}
	}
	return 0
}

func formFromEntry(entry tuiEntry) tuiForm {
	return tuiForm{
		kind:       entry.kind,
		name:       entry.name,
		host:       entry.profile.Host,
		port:       portString(entry.profile.Port),
		user:       entry.profile.User,
		database:   entry.profile.Database,
		executable: entry.profile.Executable,
		args:       strings.Join(entry.profile.Args, " "),
	}
}

func (f tuiForm) renderField(index int, label string, value string) string {
	prefix := "  "
	if f.field == index {
		prefix = "> "
	}
	return fmt.Sprintf("%s%-12s %s", prefix, label, value)
}

func (f *tuiForm) appendText(text string) {
	switch f.field {
	case 1:
		f.name += text
	case 2:
		f.host += text
	case 3:
		f.port += text
	case 4:
		f.user += text
	case 5:
		f.database += text
	case 6:
		f.executable += text
	case 7:
		f.args += text
	case 8:
		f.password += text
	}
}

func (f *tuiForm) deleteLast() {
	switch f.field {
	case 1:
		f.name = trimLastRune(f.name)
	case 2:
		f.host = trimLastRune(f.host)
	case 3:
		f.port = trimLastRune(f.port)
	case 4:
		f.user = trimLastRune(f.user)
	case 5:
		f.database = trimLastRune(f.database)
	case 6:
		f.executable = trimLastRune(f.executable)
	case 7:
		f.args = trimLastRune(f.args)
	case 8:
		f.password = trimLastRune(f.password)
	}
}

func (f *tuiForm) clearField() {
	switch f.field {
	case 1:
		f.name = ""
	case 2:
		f.host = ""
	case 3:
		f.port = ""
	case 4:
		f.user = ""
	case 5:
		f.database = ""
	case 6:
		f.executable = ""
	case 7:
		f.args = ""
	case 8:
		f.password = ""
	}
}

func nextProfileName(cfg Config, kind string) string {
	base := "local"
	profiles := cfg.MySQL
	if kind == "redis" {
		base = "cache"
		profiles = cfg.Redis
	}
	if _, ok := profiles[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if _, ok := profiles[name]; !ok {
			return name
		}
	}
}

func profileSummary(p ExternalProfile) string {
	parts := []string{}
	if p.Host != "" {
		host := p.Host
		if p.Port != 0 {
			host += ":" + strconv.Itoa(p.Port)
		}
		parts = append(parts, host)
	}
	if p.User != "" {
		parts = append(parts, "user="+p.User)
	}
	if p.Database != "" {
		parts = append(parts, "db="+p.Database)
	}
	if p.Executable != "" {
		parts = append(parts, "bin="+p.Executable)
	}
	if len(p.Args) > 0 {
		parts = append(parts, "args="+strings.Join(p.Args, " "))
	}
	if len(parts) == 0 {
		return tuiMutedStyle.Render("default client settings")
	}
	return strings.Join(parts, "  ")
}

func parseOptionalPort(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

func portString(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func mask(value string) string {
	if value == "" {
		return ""
	}
	return strings.Repeat("*", len([]rune(value)))
}

func renderMessage(message string) string {
	if strings.Contains(message, ":") || strings.Contains(message, "cannot") || strings.Contains(message, "must") {
		return tuiErrorStyle.Render(message)
	}
	return tuiOKStyle.Render(message)
}
