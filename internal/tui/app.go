package tui

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"bcli/internal/core/auth"
	"bcli/internal/core/external"
	"bcli/internal/core/profile"
	coretools "bcli/internal/core/tools"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	homeScreen screen = iota
	profilesScreen
	toolsScreen
	formScreen
	confirmDeleteScreen
)

type entry struct {
	kind      string
	name      string
	profile   profile.ExternalProfile
	hasSecret bool
}

type model struct {
	auth     auth.Service
	profiles profile.Service
	cfg      profile.Config
	entries  []entry
	selected int
	screen   screen
	message  string

	editKind string
	editName string
	form     form
	tool     toolState
}

type form struct {
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

type toolState struct {
	selected int
	input    string
	output   string
}

type toolAction struct {
	id          string
	title       string
	description string
	needsInput  bool
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	panelStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	tabStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)
)

var toolActions = []toolAction{
	{id: "uuid", title: "UUID v4", description: "Generate a random UUID.", needsInput: false},
	{id: "now", title: "Current time", description: "Print the current RFC3339 timestamp.", needsInput: false},
	{id: "urlencode", title: "URL encode", description: "Escape text for a query string.", needsInput: true},
	{id: "urldecode", title: "URL decode", description: "Decode query string text.", needsInput: true},
	{id: "base64encode", title: "Base64 encode", description: "Encode text with standard Base64.", needsInput: true},
	{id: "base64decode", title: "Base64 decode", description: "Decode standard Base64 text.", needsInput: true},
	{id: "sha256", title: "SHA256", description: "Hash text as lowercase hex.", needsInput: true},
}

func Run(profileService profile.Service, authService auth.Service, stderr io.Writer) int {
	m, err := newModel(profileService, authService)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(stderr, "run tui: %v\n", err)
		return 1
	}
	return 0
}

func newModel(profileService profile.Service, authService auth.Service) (model, error) {
	cfg, err := profileService.LoadConfig()
	if err != nil {
		return model{}, err
	}
	m := model{auth: authService, profiles: profileService, cfg: cfg}
	m.refreshEntries()
	return m, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case formScreen:
		return m.updateForm(key), nil
	case confirmDeleteScreen:
		return m.updateConfirmDelete(key), nil
	}

	switch key.String() {
	case "left":
		m.screen = previousMainScreen(m.screen)
		m.message = ""
		return m, nil
	case "right":
		m.screen = nextMainScreen(m.screen)
		m.message = ""
		return m, nil
	}

	if m.screen != toolsScreen {
		switch key.String() {
		case "1":
			m.screen = homeScreen
			m.message = ""
			return m, nil
		case "2":
			m.screen = profilesScreen
			m.message = ""
			return m, nil
		case "3":
			m.screen = toolsScreen
			m.message = ""
			return m, nil
		}
	}

	switch m.screen {
	case profilesScreen:
		next, quit := m.updateProfiles(key)
		if quit {
			return next, tea.Quit
		}
		return next, nil
	case toolsScreen:
		next, quit := m.updateTools(key)
		if quit {
			return next, tea.Quit
		}
		return next, nil
	default:
		next, quit := m.updateHome(key)
		if quit {
			return next, tea.Quit
		}
		return next, nil
	}
}

func (m model) View() string {
	switch m.screen {
	case profilesScreen:
		return m.viewProfiles()
	case toolsScreen:
		return m.viewTools()
	case formScreen:
		return m.viewForm()
	case confirmDeleteScreen:
		return m.viewConfirmDelete()
	default:
		return m.viewHome()
	}
}

func (m model) updateHome(key tea.KeyMsg) (model, bool) {
	switch key.String() {
	case "q", "ctrl+c", "esc":
		return m, true
	case "p":
		m.screen = profilesScreen
	case "t":
		m.screen = toolsScreen
	case "a":
		m.screen = formScreen
		m.message = ""
		m.form = newForm(m.cfg, "mysql")
		m.editKind = ""
		m.editName = ""
	}
	return m, false
}

func (m model) updateProfiles(key tea.KeyMsg) (model, bool) {
	switch key.String() {
	case "q", "ctrl+c":
		return m, true
	case "esc":
		m.screen = homeScreen
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.entries)-1 {
			m.selected++
		}
	case "a":
		m.screen = formScreen
		m.message = ""
		m.form = newForm(m.cfg, "mysql")
		m.editKind = ""
		m.editName = ""
	case "e", "enter":
		if len(m.entries) == 0 {
			m.message = "no profile to edit"
			return m, false
		}
		selected := m.entries[m.selected]
		m.screen = formScreen
		m.message = ""
		m.editKind = selected.kind
		m.editName = selected.name
		m.form = formFromEntry(selected)
	case "d":
		if len(m.entries) == 0 {
			m.message = "no profile to delete"
			return m, false
		}
		m.screen = confirmDeleteScreen
		m.message = ""
	}
	return m, false
}

func (m model) updateTools(key tea.KeyMsg) (model, bool) {
	switch key.String() {
	case "q", "ctrl+c":
		return m, true
	case "esc":
		m.screen = homeScreen
		m.message = ""
		return m, false
	case "up", "k":
		if m.tool.selected > 0 {
			m.tool.selected--
			m.tool.output = ""
			m.message = ""
		}
		return m, false
	case "down", "j":
		if m.tool.selected < len(toolActions)-1 {
			m.tool.selected++
			m.tool.output = ""
			m.message = ""
		}
		return m, false
	case "enter":
		return m.runTool(), false
	case "backspace", "ctrl+h":
		m.tool.input = trimLastRune(m.tool.input)
		m.tool.output = ""
		m.message = ""
		return m, false
	case " ":
		m.tool.input += " "
		m.tool.output = ""
		m.message = ""
		return m, false
	case "ctrl+u":
		m.tool.input = ""
		m.tool.output = ""
		m.message = ""
		return m, false
	}

	if len(key.Runes) > 0 {
		m.tool.input += string(key.Runes)
		m.tool.output = ""
		m.message = ""
	}
	return m, false
}

func (m model) updateConfirmDelete(key tea.KeyMsg) model {
	switch key.String() {
	case "y", "Y":
		if len(m.entries) == 0 {
			m.screen = profilesScreen
			return m
		}
		selected := m.entries[m.selected]
		m.cfg.DeleteExternalProfile(selected.kind, selected.name)
		if err := m.profiles.SaveConfig(m.cfg); err != nil {
			m.message = "save config: " + err.Error()
			m.screen = profilesScreen
			return m
		}
		if err := m.auth.DeleteCredential(selected.kind, selected.name); err != nil {
			m.message = "delete credential: " + err.Error()
			m.screen = profilesScreen
			return m
		}
		m.message = fmt.Sprintf("deleted %s/%s", selected.kind, selected.name)
		m.refreshEntries()
		if m.selected >= len(m.entries) && m.selected > 0 {
			m.selected--
		}
		m.screen = profilesScreen
	case "n", "N", "esc", "q", "ctrl+c":
		m.screen = profilesScreen
	}
	return m
}

func (m model) updateForm(key tea.KeyMsg) model {
	switch key.String() {
	case "esc":
		m.screen = profilesScreen
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
	case "left", "right":
		if m.form.field == 0 {
			m.form.toggleKind(m.cfg)
		}
		return m
	case " ":
		if m.form.field == 0 {
			m.form.toggleKind(m.cfg)
			return m
		}
		m.form.appendText(" ")
		return m
	case "backspace", "ctrl+h":
		m.form.deleteLast()
		return m
	case "ctrl+u":
		m.form.clearField()
		return m
	case "ctrl+t":
		return m.testConnection()
	}

	if m.form.field == 0 {
		return m
	}
	if len(key.Runes) > 0 {
		m.form.appendText(string(key.Runes))
	}
	return m
}

func (m model) saveForm() model {
	name, p, err := m.form.profile()
	if err != nil {
		m.message = err.Error()
		return m
	}

	oldKind, oldName := m.editKind, m.editName
	deleteOldCredential := false
	if oldName != "" && (oldKind != m.form.kind || oldName != name) {
		if secret, err := m.auth.Credential(oldKind, oldName); err == nil && secret != "" {
			if err := m.auth.StoreCredential(m.form.kind, name, secret); err != nil {
				m.message = "move credential: " + err.Error()
				return m
			}
			deleteOldCredential = true
		}
		m.cfg.DeleteExternalProfile(oldKind, oldName)
	}

	m.cfg.SetExternalProfile(m.form.kind, name, p)
	if secret := strings.TrimSpace(m.form.password); secret != "" {
		if err := m.auth.StoreCredential(m.form.kind, name, secret); err != nil {
			m.message = "store credential: " + err.Error()
			return m
		}
	}
	if err := m.profiles.SaveConfig(m.cfg); err != nil {
		m.message = "save config: " + err.Error()
		return m
	}
	if deleteOldCredential {
		_ = m.auth.DeleteCredential(oldKind, oldName)
	}

	m.refreshEntries()
	m.selected = m.findEntry(m.form.kind, name)
	m.message = fmt.Sprintf("saved %s/%s", m.form.kind, name)
	m.screen = profilesScreen
	return m
}

func (m model) testConnection() model {
	name, p, err := m.form.profile()
	if err != nil {
		m.message = err.Error()
		return m
	}

	service := external.NewService(m.profiles, m.auth, nil, io.Discard, io.Discard)
	secretOverride := strings.TrimSpace(m.form.password)
	if err := service.TestConnection(m.form.kind, name, p, secretOverride); err != nil {
		m.message = err.Error()
		return m
	}

	m.message = fmt.Sprintf("test ok: %s/%s", m.form.kind, name)
	return m
}

func (m model) runTool() model {
	if len(toolActions) == 0 {
		return m
	}
	action := toolActions[m.tool.selected]
	input := m.tool.input
	m.tool.output = ""

	switch action.id {
	case "uuid":
		value, err := coretools.UUID()
		if err != nil {
			m.message = "uuid: " + err.Error()
			return m
		}
		m.tool.output = value
	case "now":
		m.tool.output = coretools.Now()
	case "urlencode":
		m.tool.output = coretools.URLEncode(input)
	case "urldecode":
		value, err := coretools.URLDecode(input)
		if err != nil {
			m.message = "urldecode: " + err.Error()
			return m
		}
		m.tool.output = value
	case "base64encode":
		m.tool.output = coretools.Base64Encode(input)
	case "base64decode":
		value, err := coretools.Base64Decode(input)
		if err != nil {
			m.message = "base64 decode: " + err.Error()
			return m
		}
		m.tool.output = value
	case "sha256":
		m.tool.output = coretools.SHA256(input)
	}
	m.message = ""
	return m
}

func (m model) viewHome() string {
	var b strings.Builder
	b.WriteString(renderTabs(homeScreen))
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("bcli command center"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("←/→ switch page  1 home  2 profiles  3 tools  a add profile  q quit"))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Overview"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Profiles: %d total  mysql %d  redis %d\n", len(m.entries), len(m.cfg.MySQL), len(m.cfg.Redis)))
	b.WriteString(fmt.Sprintf("Credentials: %d stored  %d missing\n", m.credentialCount(), len(m.entries)-m.credentialCount()))
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Clients"))
	b.WriteString("\n")
	b.WriteString(m.renderClientStatus("mysql", "MySQL"))
	b.WriteString("\n")
	b.WriteString(m.renderClientStatus("redis", "Redis"))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Quick Actions"))
	b.WriteString("\n")
	b.WriteString("p  Manage connection profiles\n")
	b.WriteString("a  Add a MySQL/Redis profile\n")
	b.WriteString("t  Open utilities: UUID, time, URL/Base64, SHA256\n")

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(renderMessage(m.message))
	}
	return panelStyle.Render(b.String())
}

func (m model) viewProfiles() string {
	var b strings.Builder
	b.WriteString(renderTabs(profilesScreen))
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("Profiles"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("←/→ switch page  ↑/↓ select  a add  e/enter edit  d delete  esc home  q quit"))
	b.WriteString("\n\n")

	if len(m.entries) == 0 {
		b.WriteString(mutedStyle.Render("No profiles yet. Press a to create one."))
	} else {
		for i, selected := range m.entries {
			line := fmt.Sprintf("%-5s %-14s %s", selected.kind, selected.name, profileSummary(selected.profile))
			if selected.hasSecret {
				line += "  " + okStyle.Render("auth")
			} else {
				line += "  " + mutedStyle.Render("no auth")
			}
			if i == m.selected {
				b.WriteString(selectedStyle.Render(line))
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
	return panelStyle.Render(b.String())
}

func (m model) viewTools() string {
	var b strings.Builder
	b.WriteString(renderTabs(toolsScreen))
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("Tools"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("←/→ switch page  ↑/↓ select tool  type input  enter run  ctrl+u clear  esc home  q quit"))
	b.WriteString("\n\n")

	for i, action := range toolActions {
		line := fmt.Sprintf("%-15s %s", action.title, mutedStyle.Render(action.description))
		if i == m.tool.selected {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	action := toolActions[m.tool.selected]
	b.WriteString("\n")
	if action.needsInput {
		b.WriteString(fmt.Sprintf("%-8s %s\n", "Input", m.tool.input))
	} else {
		b.WriteString(mutedStyle.Render("This tool does not need input."))
		b.WriteString("\n")
	}
	if m.tool.output != "" {
		b.WriteString(fmt.Sprintf("%-8s %s\n", "Output", okStyle.Render(m.tool.output)))
	}
	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(renderMessage(m.message))
	}
	return panelStyle.Render(b.String())
}

func (m model) viewForm() string {
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
	body := titleStyle.Render(title) + "\n" +
		helpStyle.Render("tab/↑/↓ move  enter save  ctrl+t test  esc cancel  ctrl+u clear") + "\n\n" +
		strings.Join(rows, "\n")
	if m.message != "" {
		body += "\n\n" + renderMessage(m.message)
	}
	return panelStyle.Render(body)
}

func (m model) viewConfirmDelete() string {
	selected := m.entries[m.selected]
	body := titleStyle.Render("delete profile") + "\n\n" +
		fmt.Sprintf("Delete %s/%s and its stored credential?\n\n", selected.kind, selected.name) +
		helpStyle.Render("y confirm  n/esc cancel")
	return panelStyle.Render(body)
}

func (m *model) refreshEntries() {
	var entries []entry
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
			hasSecret, _ := m.auth.HasCredential(kind, name)
			entries = append(entries, entry{
				kind:      kind,
				name:      name,
				profile:   profiles[name],
				hasSecret: hasSecret,
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

func (m model) credentialCount() int {
	count := 0
	for _, selected := range m.entries {
		if selected.hasSecret {
			count++
		}
	}
	return count
}

func (m model) renderClientStatus(kind string, title string) string {
	client := m.cfg.Client(kind)
	state := mutedStyle.Render("disabled")
	if client.Enabled {
		state = okStyle.Render("enabled")
	}
	executable := client.Executable
	if executable == "" {
		if detected, ok := profile.DetectExecutable(kind); ok {
			executable = detected
		}
	}
	if executable == "" {
		executable = mutedStyle.Render("not found")
	}
	return fmt.Sprintf("%-5s %s  %s", title, state, executable)
}

func (m model) findEntry(kind string, name string) int {
	for i, selected := range m.entries {
		if selected.kind == kind && selected.name == name {
			return i
		}
	}
	return 0
}

func renderTabs(active screen) string {
	tabs := []struct {
		key    string
		title  string
		screen screen
	}{
		{key: "1", title: "Home", screen: homeScreen},
		{key: "2", title: "Profiles", screen: profilesScreen},
		{key: "3", title: "Tools", screen: toolsScreen},
	}
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		label := tab.key + " " + tab.title
		if active == tab.screen {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func previousMainScreen(current screen) screen {
	switch current {
	case homeScreen:
		return toolsScreen
	case profilesScreen:
		return homeScreen
	default:
		return profilesScreen
	}
}

func nextMainScreen(current screen) screen {
	switch current {
	case homeScreen:
		return profilesScreen
	case profilesScreen:
		return toolsScreen
	default:
		return homeScreen
	}
}

func formFromEntry(selected entry) form {
	return form{
		kind:       selected.kind,
		name:       selected.name,
		host:       selected.profile.Host,
		port:       portString(selected.profile.Port),
		user:       selected.profile.User,
		database:   selected.profile.Database,
		executable: selected.profile.Executable,
		args:       strings.Join(selected.profile.Args, " "),
	}
}

func newForm(cfg profile.Config, kind string) form {
	f := form{kind: kind, name: nextProfileName(cfg, kind)}
	f.refreshExecutable(cfg)
	return f
}

func (f *form) refreshExecutable(cfg profile.Config) {
	if strings.TrimSpace(f.executable) != "" {
		return
	}
	if executable, ok := profile.ResolveExecutableWithConfig(f.kind, cfg, ""); ok {
		f.executable = executable
	}
}

func (f *form) toggleKind(cfg profile.Config) {
	if f.kind == "mysql" {
		f.kind = "redis"
	} else {
		f.kind = "mysql"
	}
	f.refreshExecutable(cfg)
}

func (f form) renderField(index int, label string, value string) string {
	prefix := "  "
	if f.field == index {
		prefix = "> "
	}
	return fmt.Sprintf("%s%-12s %s", prefix, label, value)
}

func (f form) profile() (string, profile.ExternalProfile, error) {
	name := profile.NormalizeName(strings.TrimSpace(f.name))
	if name == "" {
		return "", profile.ExternalProfile{}, fmt.Errorf("profile name cannot be empty")
	}
	port, err := parseOptionalPort(f.port)
	if err != nil {
		return "", profile.ExternalProfile{}, err
	}
	return name, profile.ExternalProfile{
		Executable: strings.TrimSpace(f.executable),
		Args:       strings.Fields(f.args),
		Host:       strings.TrimSpace(f.host),
		Port:       port,
		User:       strings.TrimSpace(f.user),
		Database:   strings.TrimSpace(f.database),
	}, nil
}

func (f *form) appendText(text string) {
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

func (f *form) deleteLast() {
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

func (f *form) clearField() {
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

func nextProfileName(cfg profile.Config, kind string) string {
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

func profileSummary(p profile.ExternalProfile) string {
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
		return mutedStyle.Render("default client settings")
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
		return errorStyle.Render(message)
	}
	return okStyle.Render(message)
}
