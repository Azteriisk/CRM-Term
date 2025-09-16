package ui

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"crmterm/internal/config"
	"crmterm/internal/storage"
	"crmterm/internal/theme"
)

// Program wraps the Bubble Tea program lifecycle.
type Program struct {
	program *tea.Program
}

// NewProgram constructs a new interactive CRM session.
func NewProgram(store *storage.Store, cfg *config.Store) *Program {
	m := newModel(store, cfg)
	return &Program{program: tea.NewProgram(m)}
}

// Start launches the Bubble Tea program.
func (p *Program) Start() error {
	if p == nil || p.program == nil {
		return fmt.Errorf("nil program")
	}
	return p.program.Start()
}

type viewState int

type dashboardView int

type settingsMode int

type noteStage int

type eventStage int

const (
	stateMainMenu viewState = iota
	stateDashboard
	stateAccounts
	stateAccountDetail
	stateCreateAccount
	stateCreateChoice
	stateCreateNote
	stateCreateEvent
	stateSettings
	stateSettingsEditName
	stateSettingsEditTimezone
)

const (
	dashboardEvents dashboardView = iota
	dashboardActivity
)

const (
	settingsViewing settingsMode = iota
	settingsEditingName
	settingsEditingTimezone
)

const (
	noteStageContent noteStage = iota
	noteStageAssociatePrompt
	noteStageAssociateChoose
)

const (
	eventStageTitle eventStage = iota
	eventStageDetails
	eventStageSchedule
	eventStageAssociatePrompt
	eventStageAssociateChoose
)

type accountDetailView int

const (
	accountDetailSummary accountDetailView = iota
	accountDetailActivity
)

type model struct {
	state       viewState
	prevStates  []viewState
	store       *storage.Store
	cfg         *config.Store
	theme       theme.Theme
	width       int
	height      int
	infoMessage string
	errMessage  string
	showSplash  bool

	menuInput textinput.Model

	accounts         []storage.Account
	accountFilter    textinput.Model
	filteredAccounts []storage.Account

	accountForm accountForm

	noteWizard  noteWizard
	eventWizard eventWizard

	dashboard dashboardModel

	settings settingsModel

	accountDetail accountDetailModel
}

type accountForm struct {
	index    int
	fields   []formField
	input    textinput.Model
	err      string
	editing  bool
	original storage.Account
}

type formField struct {
	label    string
	value    string
	required bool
}

type noteWizard struct {
	stage          noteStage
	contentInput   textinput.Model
	associateInput textinput.Model
	accountInput   textinput.Model
	associate      bool
	err            string
	presetAccount  *storage.Account
}

type eventWizard struct {
	stage          eventStage
	titleInput     textinput.Model
	detailsInput   textinput.Model
	scheduleInput  textinput.Model
	associateInput textinput.Model
	accountInput   textinput.Model
	associate      bool
	err            string
	presetAccount  *storage.Account
}

type dashboardModel struct {
	view     dashboardView
	events   []storage.Event
	activity []storage.Activity
}

type settingsModel struct {
	mode  settingsMode
	input textinput.Model
	err   string
}

type accountDetailModel struct {
	account  storage.Account
	activity []storage.Activity
	view     accountDetailView
	err      string
}

type menuOption struct {
	id       string
	keywords []string
	synonyms []string
}

const (
	menuDashboard  = "dashboard"
	menuAccounts   = "accounts"
	menuAddAccount = "add-account"
	menuCreate     = "create"
	menuSettings   = "settings"
	menuQuit       = "quit"
)

const (
	accountActionActivity = "activity"
	accountActionAddNote  = "add-note"
	accountActionAddEvent = "add-event"
	accountActionEdit     = "edit-account"
	accountActionBack     = "back"
)

var mainMenuOptions = []menuOption{
	{
		id:       menuDashboard,
		keywords: []string{"dashboard"},
		synonyms: []string{"1", "d", "dash", "dashboard"},
	},
	{
		id:       menuAccounts,
		keywords: []string{"accounts"},
		synonyms: []string{"2", "accounts", "account", "view", "view accounts"},
	},
	{
		id:       menuAddAccount,
		keywords: []string{"add", "new"},
		synonyms: []string{"3", "add", "add account", "new account"},
	},
	{
		id:       menuCreate,
		keywords: []string{"create", "note", "event"},
		synonyms: []string{"4", "create", "note", "event", "create note", "create event"},
	},
	{
		id:       menuSettings,
		keywords: []string{"settings", "help"},
		synonyms: []string{"5", "settings", "help", "settings & help"},
	},
	{
		id:       menuQuit,
		keywords: []string{"quit", "exit"},
		synonyms: []string{"6", "quit", "exit", "exit.", "q"},
	},
}

var accountDetailOptions = []menuOption{
	{
		id:       accountActionActivity,
		keywords: []string{"activity", "timeline"},
		synonyms: []string{"1", "activity", "view", "timeline"},
	},
	{
		id:       accountActionAddNote,
		keywords: []string{"note"},
		synonyms: []string{"2", "note", "add note", "create note"},
	},
	{
		id:       accountActionAddEvent,
		keywords: []string{"event"},
		synonyms: []string{"3", "event", "add event", "create event"},
	},
	{
		id:       accountActionEdit,
		keywords: []string{"edit", "update"},
		synonyms: []string{"4", "edit", "update"},
	},
	{
		id:       accountActionBack,
		keywords: []string{"back", "close"},
		synonyms: []string{"5", "back", "exit", "exit.", "/"},
	},
}

const splashBanner = `   __________  __  ___    ______                  
  / ____/ __ \/  |/  /   /_  __/__  _________ ___ 
 / /   / /_/ / /|_/ /_____/ / / _ \/ ___/ __ '__ \
/ /___/ _, _/ /  / /_____/ / /  __/ /  / / / / / /
\____/_/ |_/_/  /_/     /_/  \___/_/  /_/ /_/ /_/ 
`

func newModel(store *storage.Store, cfg *config.Store) *model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Choose an option"
	ti.CharLimit = 32
	ti.Focus()

	filter := textinput.New()
	filter.Prompt = ""
	filter.Placeholder = "Type to search, / to go back"
	filter.CharLimit = 64

	now := time.Now().In(cfg.Location())

	m := model{
		state:         stateMainMenu,
		store:         store,
		cfg:           cfg,
		theme:         theme.Default(),
		menuInput:     ti,
		accountFilter: filter,
		dashboard:     dashboardModel{view: dashboardEvents},
		settings:      settingsModel{mode: settingsViewing, input: textinput.New()},
		showSplash:    true,
	}
	m.settings.input.Prompt = ""
	m.settings.input.CharLimit = 64

	m.accountForm = newAccountForm(nil)
	m.noteWizard = newNoteWizard(nil)
	m.eventWizard = newEventWizard(nil)
	m.refreshDashboard(now)
	m.refreshAccounts()
	return &m
}

func newAccountForm(existing *storage.Account) accountForm {
	ti := textinput.New()
	ti.Placeholder = "Account name"
	ti.CharLimit = 96
	ti.Focus()
	fields := []formField{
		{label: "Account name", required: true},
		{label: "Phone", required: false},
		{label: "Address", required: false},
		{label: "Email", required: false},
		{label: "Decision maker", required: false},
	}
	form := accountForm{
		index:  0,
		fields: fields,
		input:  ti,
	}
	if existing != nil {
		clone := *existing
		form.editing = true
		form.original = clone
		form.fields[0].value = existing.Name
		form.fields[1].value = existing.Phone
		form.fields[2].value = existing.Address
		form.fields[3].value = existing.Email
		form.fields[4].value = existing.DecisionMaker
		form.input.SetValue(existing.Name)
	}
	return form
}

func newNoteWizard(account *storage.Account) noteWizard {
	content := textinput.New()
	content.Placeholder = "Note details"
	content.CharLimit = 256
	content.Focus()

	assoc := textinput.New()
	assoc.Placeholder = "Associate with account? (y/n)"
	assoc.CharLimit = 5

	accountInput := textinput.New()
	accountInput.Placeholder = "Type account name"
	accountInput.CharLimit = 96

	wizard := noteWizard{
		stage:          noteStageContent,
		contentInput:   content,
		associateInput: assoc,
		accountInput:   accountInput,
	}
	if account != nil {
		clone := *account
		wizard.presetAccount = &clone
	}
	return wizard
}

func newEventWizard(account *storage.Account) eventWizard {
	title := textinput.New()
	title.Placeholder = "Event title"
	title.CharLimit = 96
	title.Focus()

	details := textinput.New()
	details.Placeholder = "Details (optional)"
	details.CharLimit = 256

	schedule := textinput.New()
	schedule.Placeholder = "YYYY-MM-DD HH:MM (blank = now)"
	schedule.CharLimit = 32

	assoc := textinput.New()
	assoc.Placeholder = "Associate with account? (y/n)"
	assoc.CharLimit = 5

	accountInput := textinput.New()
	accountInput.Placeholder = "Type account name"
	accountInput.CharLimit = 96

	wizard := eventWizard{
		stage:          eventStageTitle,
		titleInput:     title,
		detailsInput:   details,
		scheduleInput:  schedule,
		associateInput: assoc,
		accountInput:   accountInput,
	}
	if account != nil {
		clone := *account
		wizard.presetAccount = &clone
	}
	return wizard
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	switch m.state {
	case stateMainMenu:
		cmd = m.updateMainMenu(msg)
	case stateAccounts:
		cmd = m.updateAccounts(msg)
	case stateAccountDetail:
		cmd = m.updateAccountDetail(msg)
	case stateCreateAccount:
		cmd = m.updateAccountForm(msg)
	case stateCreateChoice:
		cmd = m.updateCreateChoice(msg)
	case stateCreateNote:
		cmd = m.updateNoteWizard(msg)
	case stateCreateEvent:
		cmd = m.updateEventWizard(msg)
	case stateDashboard:
		cmd = m.updateDashboard(msg)
	case stateSettings, stateSettingsEditName, stateSettingsEditTimezone:
		cmd = m.updateSettings(msg)
	default:
		m.state = stateMainMenu
		cmd = m.updateMainMenu(msg)
	}
	return m, cmd
}

func (m *model) View() string {
	switch m.state {
	case stateMainMenu:
		return m.viewMainMenu()
	case stateAccounts:
		return m.viewAccounts()
	case stateAccountDetail:
		return m.viewAccountDetail()
	case stateCreateAccount:
		return m.viewAccountForm()
	case stateCreateChoice:
		return m.viewCreateChoice()
	case stateCreateNote:
		return m.viewNoteWizard()
	case stateCreateEvent:
		return m.viewEventWizard()
	case stateDashboard:
		return m.viewDashboard()
	case stateSettings, stateSettingsEditName, stateSettingsEditTimezone:
		return m.viewSettings()
	default:
		return ""
	}
}

// Navigation helpers
func (m *model) pushState(next viewState) {
	m.prevStates = append(m.prevStates, m.state)
	m.state = next
}

func (m *model) popState() {
	if len(m.prevStates) == 0 {
		m.state = stateMainMenu
		return
	}
	idx := len(m.prevStates) - 1
	m.state = m.prevStates[idx]
	m.prevStates = m.prevStates[:idx]
}

func (m *model) resetMessages() {
	m.errMessage = ""
	m.infoMessage = ""
}

func (m *model) setMenuInput(placeholder string, limit int) tea.Cmd {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	if limit > 0 {
		input.CharLimit = limit
	}
	cmd := input.Focus()
	m.menuInput = input
	return cmd
}

func (m *model) ensureMenuInput(placeholder string, limit int) tea.Cmd {
	if strings.TrimSpace(m.menuInput.Placeholder) == placeholder {
		if limit <= 0 || m.menuInput.CharLimit == limit {
			if !m.menuInput.Focused() {
				return m.menuInput.Focus()
			}
			return nil
		}
	}
	return m.setMenuInput(placeholder, limit)
}

func resolveMainMenuSelection(input string) (string, bool) {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return "", false
	}
	// direct matches first
	for _, option := range mainMenuOptions {
		for _, syn := range option.synonyms {
			if value == syn {
				return option.id, true
			}
		}
	}

	matches := make(map[string]struct{})
	for _, option := range mainMenuOptions {
		for _, keyword := range option.keywords {
			if strings.HasPrefix(keyword, value) {
				matches[option.id] = struct{}{}
				break
			}
		}
	}
	if len(matches) == 1 {
		for id := range matches {
			return id, true
		}
	}
	return "", false
}

func (m *model) resolveAccountSelection(input string) (storage.Account, bool) {
	var empty storage.Account
	if len(m.accounts) == 0 && len(m.filteredAccounts) == 0 {
		return empty, false
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		if len(m.filteredAccounts) == 1 {
			return m.filteredAccounts[0], true
		}
		return empty, false
	}
	lower := strings.ToLower(trimmed)
	query := trimmed
	switch {
	case strings.HasPrefix(lower, "open "):
		query = strings.TrimSpace(trimmed[5:])
	case strings.HasPrefix(lower, "view "):
		query = strings.TrimSpace(trimmed[5:])
	case strings.HasPrefix(lower, "select "):
		query = strings.TrimSpace(trimmed[7:])
	case strings.HasPrefix(lower, "#"):
		query = strings.TrimSpace(trimmed[1:])
	}
	if idx, err := strconv.Atoi(query); err == nil {
		if idx > 0 && idx <= len(m.filteredAccounts) {
			return m.filteredAccounts[idx-1], true
		}
	}
	for _, list := range [][]storage.Account{m.filteredAccounts, m.accounts} {
		for i := range list {
			if strings.EqualFold(list[i].Name, query) {
				return list[i], true
			}
		}
	}
	queryLower := strings.ToLower(query)
	var match storage.Account
	count := 0
	for _, list := range [][]storage.Account{m.filteredAccounts, m.accounts} {
		for i := range list {
			if strings.HasPrefix(strings.ToLower(list[i].Name), queryLower) {
				match = list[i]
				count++
			}
		}
		if count == 1 {
			return match, true
		}
	}
	return empty, false
}

func resolveAccountDetailAction(input string) (string, bool) {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return "", false
	}
	for _, option := range accountDetailOptions {
		for _, syn := range option.synonyms {
			if value == syn {
				return option.id, true
			}
		}
	}
	matches := make(map[string]struct{})
	for _, option := range accountDetailOptions {
		for _, keyword := range option.keywords {
			if strings.HasPrefix(keyword, value) {
				matches[option.id] = struct{}{}
				break
			}
		}
	}
	if len(matches) == 1 {
		for id := range matches {
			return id, true
		}
	}
	return "", false
}

func (m *model) openAccountDetail(account storage.Account) tea.Cmd {
	m.accountDetail.account = account
	m.accountDetail.view = accountDetailSummary
	m.accountDetail.activity = nil
	m.accountDetail.err = ""
	m.refreshAccountDetailAccount()
	m.pushState(stateAccountDetail)
	return m.setMenuInput("1=Activity  2=Add note  3=Add event  4=Edit  5=Back", 64)
}

func (m *model) refreshAccountDetailAccount() {
	if m.accountDetail.account.ID == 0 {
		return
	}
	ctx := context.Background()
	account, err := m.store.AccountByID(ctx, m.accountDetail.account.ID)
	if err != nil {
		m.accountDetail.err = fmt.Sprintf("load account: %v", err)
		return
	}
	m.accountDetail.account = *account
}

func (m *model) loadAccountActivity() {
	if m.accountDetail.account.ID == 0 {
		m.accountDetail.activity = nil
		return
	}
	ctx := context.Background()
	activity, err := m.store.ListAccountActivity(ctx, m.accountDetail.account.ID, 50)
	if err != nil {
		m.accountDetail.err = fmt.Sprintf("load activity: %v", err)
		return
	}
	m.accountDetail.err = ""
	m.accountDetail.activity = activity
}

func (m *model) handleAccountImport(path string) {
	m.infoMessage = ""
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		m.errMessage = "Provide a CSV path"
		return
	}
	resolved, err := expandPath(trimmed)
	if err != nil {
		m.errMessage = fmt.Sprintf("import path: %v", err)
		return
	}
	file, err := os.Open(resolved)
	if err != nil {
		m.errMessage = fmt.Sprintf("open file: %v", err)
		return
	}
	defer file.Close()
	ctx := context.Background()
	result, err := m.store.ImportAccountsCSV(ctx, file, m.cfg.Config.Name, m.cfg.Location())
	if err != nil {
		m.errMessage = fmt.Sprintf("import csv: %v", err)
		return
	}
	parts := []string{fmt.Sprintf("Imported %d account(s)", result.Created)}
	if result.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("skipped %d", result.Skipped))
	}
	m.infoMessage = strings.Join(parts, ", ")
	if len(result.Errors) > 0 {
		m.errMessage = strings.Join(result.Errors, "; ")
	} else {
		m.errMessage = ""
	}
}

func expandPath(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			switch {
			case len(trimmed) == 1:
				trimmed = home
			case trimmed[1] == '/', trimmed[1] == '\\':
				trimmed = filepath.Join(home, trimmed[2:])
			}
		}
	}
	return filepath.Abs(trimmed)
}

func batchCmds(cmds []tea.Cmd) tea.Cmd {
	filtered := cmds[:0]
	for _, c := range cmds {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}

// global command helpers
func isExitCommand(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	return v == "exit." || v == "quit"
}

func isBackCommand(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	return v == "/" || v == "back"
}

func (m *model) refreshAccounts() {
	ctx := context.Background()
	accounts, err := m.store.ListAccounts(ctx)
	if err != nil {
		m.errMessage = fmt.Sprintf("load accounts: %v", err)
		return
	}
	m.accounts = accounts
	filter := strings.TrimSpace(m.accountFilter.Value())
	if filter == "" {
		m.filteredAccounts = accounts
		return
	}
	filtered, err := m.store.SearchAccounts(ctx, filter)
	if err != nil {
		m.errMessage = fmt.Sprintf("search accounts: %v", err)
		return
	}
	m.filteredAccounts = filtered
}

func (m *model) refreshDashboard(now time.Time) {
	ctx := context.Background()
	events, err := m.store.ListEvents(ctx)
	if err != nil {
		m.errMessage = fmt.Sprintf("load events: %v", err)
	} else {
		m.dashboard.events = events
	}
	activity, err := m.store.ListActivities(ctx, 50)
	if err != nil {
		m.errMessage = fmt.Sprintf("load activity: %v", err)
	} else {
		m.dashboard.activity = activity
	}
}

// MAIN MENU
func (m *model) updateMainMenu(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if focus := m.ensureMenuInput("Choose an option", 32); focus != nil {
		cmds = append(cmds, focus)
	}

	var cmd tea.Cmd
	m.menuInput, cmd = m.menuInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		choice := strings.TrimSpace(strings.ToLower(m.menuInput.Value()))
		m.menuInput.SetValue("")
		m.showSplash = false
		action, ok := resolveMainMenuSelection(choice)
		if !ok {
			if choice == "" || choice == "0" {
				return batchCmds(cmds)
			}
			m.errMessage = "Unknown choice"
			return batchCmds(cmds)
		}
		switch action {
		case menuDashboard:
			m.resetMessages()
			m.pushState(stateDashboard)
			m.refreshDashboard(time.Now().In(m.cfg.Location()))
			if focus := m.setMenuInput("Command (t=toggle, r=refresh, /, exit.)", 48); focus != nil {
				cmds = append(cmds, focus)
			}
		case menuAccounts:
			m.resetMessages()
			m.pushState(stateAccounts)
			if !m.accountFilter.Focused() {
				if focus := m.accountFilter.Focus(); focus != nil {
					cmds = append(cmds, focus)
				}
			}
			m.refreshAccounts()
		case menuAddAccount:
			m.resetMessages()
			m.accountForm = newAccountForm(nil)
			m.pushState(stateCreateAccount)
		case menuCreate:
			m.resetMessages()
			m.pushState(stateCreateChoice)
			if focus := m.setMenuInput("1=Note  2=Event  3=Back", 32); focus != nil {
				cmds = append(cmds, focus)
			}
		case menuSettings:
			m.resetMessages()
			m.settings = settingsModel{mode: settingsViewing, input: textinput.New()}
			m.settings.input.CharLimit = 96
			m.settings.input.Prompt = ""
			m.pushState(stateSettings)
			if focus := m.setMenuInput("1=Name  2=Timezone  3=Back", 40); focus != nil {
				cmds = append(cmds, focus)
			}
		case menuQuit:
			cmds = append(cmds, tea.Quit)
		}
	}

	return batchCmds(cmds)
}

func (m *model) viewMainMenu() string {
	lines := []string{}
	if m.showSplash {
		lines = append(lines, splashBanner)
		lines = append(lines, "")
	}
	lines = append(lines, m.theme.Title.Render("CRM-Term"))
	lines = append(lines, m.theme.Secondary.Render("A lightning-fast terminal CRM"))
	if m.infoMessage != "" {
		lines = append(lines, m.theme.Success.Render(m.infoMessage))
	}
	if m.errMessage != "" {
		lines = append(lines, m.theme.Danger.Render(m.errMessage))
	}
	menu := []string{
		"1. Dashboard",
		"2. View accounts",
		"3. Add account",
		"4. Create note/event",
		"5. Settings & Help",
		"6. Quit",
	}
	lines = append(lines, "")
	for _, item := range menu {
		lines = append(lines, m.theme.Primary.Render(item))
	}
	lines = append(lines, "")
	lines = append(lines, m.theme.Accent.Render("> ")+m.menuInput.View())
	return strings.Join(lines, "\n") + "\n"
}

// ACCOUNTS LIST
func (m *model) updateAccounts(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.accountFilter, cmd = m.accountFilter.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch key := msg.(type) {
	case tea.KeyMsg:
		switch key.Type {
		case tea.KeyEnter:
			value := strings.TrimSpace(m.accountFilter.Value())
			if isExitCommand(value) {
				m.accountFilter.SetValue("")
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			}
			if isBackCommand(value) {
				m.accountFilter.SetValue("")
				m.popState()
				if m.state == stateMainMenu {
					if focus := m.setMenuInput("Choose an option", 32); focus != nil {
						cmds = append(cmds, focus)
					}
				}
				return batchCmds(cmds)
			}
			trimmedValue := strings.TrimSpace(value)
			if strings.HasPrefix(strings.ToLower(trimmedValue), "import ") {
				path := strings.TrimSpace(trimmedValue[len("import "):])
				m.handleAccountImport(path)
				m.accountFilter.SetValue("")
				m.refreshAccounts()
				return batchCmds(cmds)
			}
			if account, ok := m.resolveAccountSelection(trimmedValue); ok {
				m.accountFilter.SetValue("")
				if focus := m.openAccountDetail(account); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			}
			m.refreshAccounts()
		case tea.KeyEsc:
			m.popState()
			if m.state == stateMainMenu {
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			}
			return batchCmds(cmds)
		}
	}

	filter := strings.TrimSpace(m.accountFilter.Value())
	if filter == "" {
		m.filteredAccounts = m.accounts
	} else {
		accounts, err := m.store.SearchAccounts(context.Background(), filter)
		if err == nil {
			m.filteredAccounts = accounts
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewAccounts() string {
	lines := []string{m.theme.Title.Render("Accounts")}
	lines = append(lines, m.theme.Faint.Render("Type to search. Enter a number or name to manage, or 'import <path>' to load CSV. '/' to go back, 'exit.' home."))
	lines = append(lines, "")
	if len(m.filteredAccounts) == 0 {
		lines = append(lines, m.theme.Warning.Render("No accounts found."))
	} else {
		for i, a := range m.filteredAccounts {
			created := a.CreatedAt.In(m.cfg.Location()).Format("Jan 02 2006 15:04")
			header := fmt.Sprintf("%d. %s", i+1, a.Name)
			lines = append(lines, m.theme.Primary.Render(header))
			meta := []string{}
			if a.Phone != "" {
				meta = append(meta, fmt.Sprintf("Phone: %s", a.Phone))
			}
			if a.Email != "" {
				meta = append(meta, fmt.Sprintf("Email: %s", a.Email))
			}
			if a.DecisionMaker != "" {
				meta = append(meta, fmt.Sprintf("Decision Maker: %s", a.DecisionMaker))
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+m.theme.Secondary.Render(strings.Join(meta, "  •  ")))
			}
			if a.Address != "" {
				lines = append(lines, "  "+m.theme.Faint.Render(a.Address))
			}
			lines = append(lines, "  "+m.theme.Faint.Render(fmt.Sprintf("Created by %s on %s", a.Creator, created)))
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.theme.Border.Render(strings.Repeat("─", 40)))
	lines = append(lines, m.theme.Accent.Render("find> ")+m.accountFilter.View())
	return strings.Join(lines, "\n") + "\n"
}

// ACCOUNT FORM
func (m *model) updateAccountForm(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.accountForm.input, cmd = m.accountForm.input.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEnter:
			value := strings.TrimSpace(m.accountForm.input.Value())
			if isExitCommand(value) {
				m.accountForm = newAccountForm(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			}
			if isBackCommand(value) {
				if m.accountForm.index == 0 {
					var focus tea.Cmd
					if m.accountForm.editing {
						account := m.accountForm.original
						m.accountForm = newAccountForm(&account)
					} else {
						m.accountForm = newAccountForm(nil)
					}
					m.popState()
					if m.state == stateMainMenu {
						focus = m.setMenuInput("Choose an option", 32)
					} else if m.state == stateAccountDetail {
						focus = m.setMenuInput("1=Activity  2=Add note  3=Add event  4=Edit  5=Back", 64)
					}
					if focus != nil {
						cmds = append(cmds, focus)
					}
					return batchCmds(cmds)
				}
				m.accountForm.index--
				prev := m.accountForm.fields[m.accountForm.index]
				m.accountForm.input.Placeholder = prev.label
				m.accountForm.input.SetValue(prev.value)
				m.accountForm.err = ""
				return batchCmds(cmds)
			}
			if m.accountForm.fields[m.accountForm.index].required && value == "" {
				m.accountForm.err = "This field is required"
				return batchCmds(cmds)
			}
			m.accountForm.fields[m.accountForm.index].value = value
			m.accountForm.input.SetValue("")
			m.accountForm.err = ""
			if m.accountForm.index >= len(m.accountForm.fields)-1 {
				base := storage.Account{}
				if m.accountForm.editing {
					base = m.accountForm.original
				}
				account := buildAccount(m.accountForm.fields, base)
				ctx := context.Background()
				if m.accountForm.editing {
					if err := m.store.UpdateAccount(ctx, &account); err != nil {
						if err == storage.ErrAccountExists {
							m.accountForm.err = "An account with that name already exists"
							m.accountForm.index = 0
							m.accountForm.input.SetValue(account.Name)
							m.accountForm.input.Placeholder = m.accountForm.fields[0].label
							return batchCmds(cmds)
						}
						m.accountForm.err = err.Error()
						return batchCmds(cmds)
					}
					m.infoMessage = fmt.Sprintf("Account '%s' updated", account.Name)
				} else {
					account.Creator = m.cfg.Config.Name
					account.CreatedAt = time.Now().In(m.cfg.Location())
					if err := m.store.CreateAccount(ctx, &account); err != nil {
						if err == storage.ErrAccountExists {
							m.accountForm.err = "An account with that name already exists"
							m.accountForm.index = 0
							m.accountForm.input.SetValue("")
							m.accountForm.input.Placeholder = m.accountForm.fields[0].label
							return batchCmds(cmds)
						}
						m.accountForm.err = err.Error()
						return batchCmds(cmds)
					}
					m.infoMessage = fmt.Sprintf("Account '%s' created", account.Name)
				}
				if m.accountForm.editing {
					m.accountDetail.account = account
					m.refreshAccountDetailAccount()
				}
				m.accountForm = newAccountForm(nil)
				m.popState()
				var focus tea.Cmd
				if m.state == stateMainMenu {
					focus = m.setMenuInput("Choose an option", 32)
				} else if m.state == stateAccountDetail {
					focus = m.setMenuInput("1=Activity  2=Add note  3=Add event  4=Edit  5=Back", 64)
				}
				if focus != nil {
					cmds = append(cmds, focus)
				}
				m.refreshAccounts()
				if m.state == stateAccountDetail {
					m.loadAccountActivity()
				}
				return batchCmds(cmds)
			}
			m.accountForm.index++
			next := m.accountForm.fields[m.accountForm.index]
			m.accountForm.input.Placeholder = next.label
			m.accountForm.input.SetValue(next.value)
		case tea.KeyEsc:
			var focus tea.Cmd
			if m.accountForm.editing {
				account := m.accountForm.original
				m.accountForm = newAccountForm(&account)
			} else {
				m.accountForm = newAccountForm(nil)
			}
			m.popState()
			if m.state == stateMainMenu {
				focus = m.setMenuInput("Choose an option", 32)
			} else if m.state == stateAccountDetail {
				focus = m.setMenuInput("1=Activity  2=Add note  3=Add event  4=Edit  5=Back", 64)
			}
			if focus != nil {
				cmds = append(cmds, focus)
			}
			return batchCmds(cmds)
		}
	}
	return batchCmds(cmds)
}

func buildAccount(fields []formField, base storage.Account) storage.Account {
	account := base
	if len(fields) > 0 {
		account.Name = fields[0].value
	}
	if len(fields) > 1 {
		account.Phone = fields[1].value
	}
	if len(fields) > 2 {
		account.Address = fields[2].value
	}
	if len(fields) > 3 {
		account.Email = fields[3].value
	}
	if len(fields) > 4 {
		account.DecisionMaker = fields[4].value
	}
	return account
}

func (m *model) viewAccountForm() string {
	field := m.accountForm.fields[m.accountForm.index]
	title := "Add Account"
	if m.accountForm.editing {
		title = "Edit Account"
	}
	lines := []string{
		m.theme.Title.Render(title),
		m.theme.Faint.Render("Enter details. '/' to go back, 'exit.' to cancel."),
		"",
		m.theme.Secondary.Render(fmt.Sprintf("%d/%d", m.accountForm.index+1, len(m.accountForm.fields))),
		m.theme.Primary.Render(field.label + ":"),
		m.accountForm.input.View(),
	}
	if m.accountForm.err != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.accountForm.err))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m *model) updateAccountDetail(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if focus := m.ensureMenuInput("1=Activity  2=Add note  3=Add event  4=Edit  5=Back", 64); focus != nil {
		cmds = append(cmds, focus)
	}
	var cmd tea.Cmd
	m.menuInput, cmd = m.menuInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch key := msg.(type) {
	case tea.KeyMsg:
		switch key.Type {
		case tea.KeyEnter:
			choice := strings.TrimSpace(strings.ToLower(m.menuInput.Value()))
			m.menuInput.SetValue("")
			action, ok := resolveAccountDetailAction(choice)
			if !ok {
				if choice == "" {
					return batchCmds(cmds)
				}
				m.accountDetail.err = "Unknown choice"
				return batchCmds(cmds)
			}
			m.accountDetail.err = ""
			switch action {
			case accountActionActivity:
				m.accountDetail.view = accountDetailActivity
				m.loadAccountActivity()
			case accountActionAddNote:
				m.accountDetail.view = accountDetailSummary
				account := m.accountDetail.account
				m.noteWizard = newNoteWizard(&account)
				m.pushState(stateCreateNote)
				return batchCmds(cmds)
			case accountActionAddEvent:
				m.accountDetail.view = accountDetailSummary
				account := m.accountDetail.account
				m.eventWizard = newEventWizard(&account)
				m.pushState(stateCreateEvent)
				return batchCmds(cmds)
			case accountActionEdit:
				m.accountDetail.view = accountDetailSummary
				account := m.accountDetail.account
				m.accountForm = newAccountForm(&account)
				m.pushState(stateCreateAccount)
				return batchCmds(cmds)
			case accountActionBack:
				m.popState()
				if m.state == stateMainMenu {
					if focus := m.setMenuInput("Choose an option", 32); focus != nil {
						cmds = append(cmds, focus)
					}
				}
				return batchCmds(cmds)
			}
		case tea.KeyEsc:
			m.popState()
			if m.state == stateMainMenu {
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			}
			return batchCmds(cmds)
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewAccountDetail() string {
	a := m.accountDetail.account
	lines := []string{m.theme.Title.Render(a.Name)}
	meta := []string{}
	if a.Phone != "" {
		meta = append(meta, fmt.Sprintf("Phone: %s", a.Phone))
	}
	if a.Email != "" {
		meta = append(meta, fmt.Sprintf("Email: %s", a.Email))
	}
	if a.DecisionMaker != "" {
		meta = append(meta, fmt.Sprintf("Decision Maker: %s", a.DecisionMaker))
	}
	if len(meta) > 0 {
		lines = append(lines, m.theme.Secondary.Render(strings.Join(meta, "  •  ")))
	}
	if a.Address != "" {
		lines = append(lines, m.theme.Faint.Render(a.Address))
	}
	created := a.CreatedAt.In(m.cfg.Location()).Format("Jan 02 2006 15:04")
	lines = append(lines, m.theme.Faint.Render(fmt.Sprintf("Created by %s on %s", a.Creator, created)))
	lines = append(lines, "")

	if m.accountDetail.view == accountDetailActivity {
		lines = append(lines, m.theme.Subtitle.Render("Recent Activity"))
		if len(m.accountDetail.activity) == 0 {
			lines = append(lines, m.theme.Faint.Render("No activity yet."))
		} else {
			for _, act := range m.accountDetail.activity {
				stamp := act.CreatedAt.In(m.cfg.Location()).Format("Jan 02 15:04")
				typeLabel := act.Type
				if len(typeLabel) > 0 {
					typeLabel = strings.ToUpper(typeLabel[:1]) + typeLabel[1:]
				}
				item := fmt.Sprintf("[%s] %s — %s", typeLabel, act.Title, stamp)
				lines = append(lines, m.theme.Primary.Render(item))
			}
		}
		lines = append(lines, "")
	}

	lines = append(lines, m.theme.Subtitle.Render("Actions"))
	lines = append(lines, m.theme.Secondary.Render("1. View activity"))
	lines = append(lines, m.theme.Secondary.Render("2. Add note (auto links)"))
	lines = append(lines, m.theme.Secondary.Render("3. Add event (auto links)"))
	lines = append(lines, m.theme.Secondary.Render("4. Edit account"))
	lines = append(lines, m.theme.Faint.Render("5. Back"))
	lines = append(lines, "")
	lines = append(lines, m.theme.Accent.Render("> ")+m.menuInput.View())
	if m.accountDetail.err != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.accountDetail.err))
	}
	if m.infoMessage != "" {
		lines = append(lines, "", m.theme.Success.Render(m.infoMessage))
	}
	if m.errMessage != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.errMessage))
	}
	return strings.Join(lines, "\n") + "\n"
}

// CREATE CHOICE
func (m *model) updateCreateChoice(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if focus := m.ensureMenuInput("1=Note  2=Event  3=Back", 32); focus != nil {
		cmds = append(cmds, focus)
	}
	var cmd tea.Cmd
	m.menuInput, cmd = m.menuInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		input := strings.TrimSpace(strings.ToLower(m.menuInput.Value()))
		m.menuInput.SetValue("")
		switch input {
		case "1", "note", "n":
			m.noteWizard = newNoteWizard(nil)
			m.state = stateCreateNote
		case "2", "event", "e":
			m.eventWizard = newEventWizard(nil)
			m.state = stateCreateEvent
		case "3", "back", "/":
			m.popState()
			if m.state == stateMainMenu {
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			}
		case "exit.", "exit", "quit":
			m.prevStates = nil
			m.state = stateMainMenu
			if focus := m.setMenuInput("Choose an option", 32); focus != nil {
				cmds = append(cmds, focus)
			}
		default:
			m.errMessage = "Choose 1 for note or 2 for event"
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewCreateChoice() string {
	lines := []string{
		m.theme.Title.Render("Create Note or Event"),
		m.theme.Secondary.Render("1. Note"),
		m.theme.Secondary.Render("2. Event"),
		m.theme.Faint.Render("3. Back"),
		"",
		m.theme.Accent.Render("> ") + m.menuInput.View(),
	}
	if m.errMessage != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.errMessage))
	}
	lines = append(lines, "", m.theme.Accent.Render("cmd> ")+m.menuInput.View())
	return strings.Join(lines, "\n") + "\n"
}

// NOTE WIZARD
func (m *model) updateNoteWizard(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch m.noteWizard.stage {
	case noteStageContent:
		if !m.noteWizard.contentInput.Focused() {
			if focus := m.noteWizard.contentInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.noteWizard.contentInput, cmd = m.noteWizard.contentInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.noteWizard.contentInput.Value())
			switch {
			case isExitCommand(value):
				m.noteWizard = newNoteWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.noteWizard = newNoteWizard(nil)
				m.popState()
				if m.state == stateMainMenu {
					if focus := m.setMenuInput("Choose an option", 32); focus != nil {
						cmds = append(cmds, focus)
					}
				}
				return batchCmds(cmds)
			case value == "":
				m.noteWizard.err = "Note cannot be empty"
			default:
				m.noteWizard.err = ""
				if m.noteWizard.presetAccount != nil {
					accountID := sql.NullInt64{Int64: m.noteWizard.presetAccount.ID, Valid: true}
					if err := m.saveNote(&accountID); err != nil {
						m.noteWizard.err = err.Error()
					} else {
						name := m.noteWizard.presetAccount.Name
						if name == "" {
							name = m.accountDetail.account.Name
						}
						message := "Note saved"
						if name != "" {
							message = fmt.Sprintf("Note saved for %s", name)
						}
						m.completeNoteSave(message)
						return batchCmds(cmds)
					}
				} else {
					m.noteWizard.stage = noteStageAssociatePrompt
				}
			}
		}
	case noteStageAssociatePrompt:
		if !m.noteWizard.associateInput.Focused() {
			if focus := m.noteWizard.associateInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.noteWizard.associateInput, cmd = m.noteWizard.associateInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.ToLower(strings.TrimSpace(m.noteWizard.associateInput.Value()))
			m.noteWizard.associateInput.SetValue("")
			switch {
			case isExitCommand(value):
				m.noteWizard = newNoteWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.noteWizard.stage = noteStageContent
			case value == "y" || value == "yes":
				m.noteWizard.associate = true
				m.noteWizard.stage = noteStageAssociateChoose
			case value == "n" || value == "no" || value == "":
				m.noteWizard.associate = false
				if err := m.saveNote(nil); err != nil {
					m.noteWizard.err = err.Error()
				} else {
					m.completeNoteSave("Note saved")
					return batchCmds(cmds)
				}
			default:
				m.noteWizard.err = "Please answer y or n"
			}
		}
	case noteStageAssociateChoose:
		if !m.noteWizard.accountInput.Focused() {
			if focus := m.noteWizard.accountInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.noteWizard.accountInput, cmd = m.noteWizard.accountInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.noteWizard.accountInput.Value())
			switch {
			case isExitCommand(value):
				m.noteWizard = newNoteWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.noteWizard.stage = noteStageAssociatePrompt
			case value == "":
				if err := m.saveNote(nil); err != nil {
					m.noteWizard.err = err.Error()
				} else {
					m.completeNoteSave("Note saved")
					return batchCmds(cmds)
				}
			default:
				account, err := m.store.AccountByName(context.Background(), value)
				if err != nil {
					m.noteWizard.err = "Account not found"
				} else {
					accountID := sql.NullInt64{Int64: account.ID, Valid: true}
					if err := m.saveNote(&accountID); err != nil {
						m.noteWizard.err = err.Error()
					} else {
						m.completeNoteSave(fmt.Sprintf("Note saved for %s", account.Name))
						return batchCmds(cmds)
					}
				}
			}
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewNoteWizard() string {
	lines := []string{m.theme.Title.Render("New Note")}
	switch m.noteWizard.stage {
	case noteStageContent:
		lines = append(lines, m.theme.Faint.Render("Type note text and press enter. '/' to cancel."))
		if m.noteWizard.presetAccount != nil {
			lines = append(lines, m.theme.Faint.Render(fmt.Sprintf("Will link to %s", m.noteWizard.presetAccount.Name)))
		}
		lines = append(lines, "")
		lines = append(lines, m.noteWizard.contentInput.View())
	case noteStageAssociatePrompt:
		lines = append(lines, m.theme.Secondary.Render("Associate with an account? (y/n)"))
		lines = append(lines, m.noteWizard.associateInput.View())
	case noteStageAssociateChoose:
		lines = append(lines, m.theme.Secondary.Render("Enter account name (blank to skip)"))
		lines = append(lines, m.noteWizard.accountInput.View())
	}
	if m.noteWizard.err != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.noteWizard.err))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m *model) saveNote(accountID *sql.NullInt64) error {
	content := strings.TrimSpace(m.noteWizard.contentInput.Value())
	note := storage.Note{
		Content:   content,
		Creator:   m.cfg.Config.Name,
		CreatedAt: time.Now().In(m.cfg.Location()),
	}
	if accountID != nil {
		note.AccountID = *accountID
	}
	ctx := context.Background()
	return m.store.CreateNote(ctx, &note)
}

func (m *model) completeNoteSave(message string) {
	m.noteWizard = newNoteWizard(nil)
	m.infoMessage = message
	m.popState()
	if m.state == stateAccountDetail {
		m.refreshAccountDetailAccount()
		m.loadAccountActivity()
	} else if m.state == stateMainMenu {
		m.setMenuInput("Choose an option", 32)
	}
	m.refreshDashboard(time.Now().In(m.cfg.Location()))
}

// EVENT WIZARD
func (m *model) updateEventWizard(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch m.eventWizard.stage {
	case eventStageTitle:
		if !m.eventWizard.titleInput.Focused() {
			if focus := m.eventWizard.titleInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.eventWizard.titleInput, cmd = m.eventWizard.titleInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.eventWizard.titleInput.Value())
			switch {
			case isExitCommand(value):
				m.eventWizard = newEventWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.eventWizard = newEventWizard(nil)
				m.popState()
				if m.state == stateMainMenu {
					if focus := m.setMenuInput("Choose an option", 32); focus != nil {
						cmds = append(cmds, focus)
					}
				}
				return batchCmds(cmds)
			case value == "":
				m.eventWizard.err = "Title is required"
			default:
				m.eventWizard.err = ""
				m.eventWizard.stage = eventStageDetails
			}
		}
	case eventStageDetails:
		if !m.eventWizard.detailsInput.Focused() {
			if focus := m.eventWizard.detailsInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.eventWizard.detailsInput, cmd = m.eventWizard.detailsInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.eventWizard.detailsInput.Value())
			if isExitCommand(value) {
				m.eventWizard = newEventWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			}
			if isBackCommand(value) {
				m.eventWizard.stage = eventStageTitle
				return batchCmds(cmds)
			}
			m.eventWizard.stage = eventStageSchedule
		}
	case eventStageSchedule:
		if !m.eventWizard.scheduleInput.Focused() {
			if focus := m.eventWizard.scheduleInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.eventWizard.scheduleInput, cmd = m.eventWizard.scheduleInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.eventWizard.scheduleInput.Value())
			if isExitCommand(value) {
				m.eventWizard = newEventWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			}
			if isBackCommand(value) {
				m.eventWizard.stage = eventStageDetails
				return batchCmds(cmds)
			}
			if value != "" {
				loc := m.cfg.Location()
				if _, err := time.ParseInLocation("2006-01-02 15:04", value, loc); err != nil {
					m.eventWizard.err = "Use format YYYY-MM-DD HH:MM"
					return batchCmds(cmds)
				}
			}
			m.eventWizard.err = ""
			if m.eventWizard.presetAccount != nil {
				accountID := sql.NullInt64{Int64: m.eventWizard.presetAccount.ID, Valid: true}
				if err := m.saveEvent(&accountID); err != nil {
					m.eventWizard.err = err.Error()
				} else {
					name := m.eventWizard.presetAccount.Name
					if name == "" {
						name = m.accountDetail.account.Name
					}
					message := "Event created"
					if name != "" {
						message = fmt.Sprintf("Event created for %s", name)
					}
					m.completeEventSave(message)
					return batchCmds(cmds)
				}
			} else {
				m.eventWizard.stage = eventStageAssociatePrompt
			}
		}
	case eventStageAssociatePrompt:
		if !m.eventWizard.associateInput.Focused() {
			if focus := m.eventWizard.associateInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.eventWizard.associateInput, cmd = m.eventWizard.associateInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.ToLower(strings.TrimSpace(m.eventWizard.associateInput.Value()))
			m.eventWizard.associateInput.SetValue("")
			switch {
			case isExitCommand(value):
				m.eventWizard = newEventWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.eventWizard.stage = eventStageSchedule
			case value == "y" || value == "yes":
				m.eventWizard.associate = true
				m.eventWizard.stage = eventStageAssociateChoose
			case value == "n" || value == "no" || value == "":
				m.eventWizard.associate = false
				if err := m.saveEvent(nil); err != nil {
					m.eventWizard.err = err.Error()
				} else {
					m.completeEventSave("Event created")
					return batchCmds(cmds)
				}
			default:
				m.eventWizard.err = "Please answer y or n"
			}
		}
	case eventStageAssociateChoose:
		if !m.eventWizard.accountInput.Focused() {
			if focus := m.eventWizard.accountInput.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.eventWizard.accountInput, cmd = m.eventWizard.accountInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.eventWizard.accountInput.Value())
			switch {
			case isExitCommand(value):
				m.eventWizard = newEventWizard(nil)
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
				return batchCmds(cmds)
			case isBackCommand(value):
				m.eventWizard.stage = eventStageAssociatePrompt
			case value == "":
				if err := m.saveEvent(nil); err != nil {
					m.eventWizard.err = err.Error()
				} else {
					m.completeEventSave("Event created")
					return batchCmds(cmds)
				}
			default:
				account, err := m.store.AccountByName(context.Background(), value)
				if err != nil {
					m.eventWizard.err = "Account not found"
				} else {
					id := sql.NullInt64{Int64: account.ID, Valid: true}
					if err := m.saveEvent(&id); err != nil {
						m.eventWizard.err = err.Error()
					} else {
						m.completeEventSave(fmt.Sprintf("Event created for %s", account.Name))
						return batchCmds(cmds)
					}
				}
			}
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewEventWizard() string {
	lines := []string{m.theme.Title.Render("New Event")}
	switch m.eventWizard.stage {
	case eventStageTitle:
		lines = append(lines, m.theme.Secondary.Render("Event title:"))
		lines = append(lines, m.eventWizard.titleInput.View())
		if m.eventWizard.presetAccount != nil {
			lines = append(lines, m.theme.Faint.Render(fmt.Sprintf("Will link to %s", m.eventWizard.presetAccount.Name)))
		}
	case eventStageDetails:
		lines = append(lines, m.theme.Secondary.Render("Details (optional):"))
		lines = append(lines, m.eventWizard.detailsInput.View())
	case eventStageSchedule:
		lines = append(lines, m.theme.Secondary.Render("Schedule time (YYYY-MM-DD HH:MM, blank = now):"))
		lines = append(lines, m.eventWizard.scheduleInput.View())
	case eventStageAssociatePrompt:
		lines = append(lines, m.theme.Secondary.Render("Associate with an account? (y/n)"))
		lines = append(lines, m.eventWizard.associateInput.View())
	case eventStageAssociateChoose:
		lines = append(lines, m.theme.Secondary.Render("Enter account name (blank to skip):"))
		lines = append(lines, m.eventWizard.accountInput.View())
	}
	lines = append(lines, m.theme.Faint.Render("'/' goes back, 'exit.' returns home."))
	if m.eventWizard.err != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.eventWizard.err))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m *model) saveEvent(accountID *sql.NullInt64) error {
	title := strings.TrimSpace(m.eventWizard.titleInput.Value())
	details := strings.TrimSpace(m.eventWizard.detailsInput.Value())
	scheduleStr := strings.TrimSpace(m.eventWizard.scheduleInput.Value())
	loc := m.cfg.Location()
	var eventTime time.Time
	if scheduleStr == "" {
		eventTime = time.Now().In(loc)
	} else {
		parsed, err := time.ParseInLocation("2006-01-02 15:04", scheduleStr, loc)
		if err != nil {
			return fmt.Errorf("invalid schedule format")
		}
		eventTime = parsed
	}
	evt := storage.Event{
		Title:     title,
		Details:   details,
		EventTime: eventTime,
		Creator:   m.cfg.Config.Name,
		CreatedAt: time.Now().In(loc),
	}
	if accountID != nil {
		evt.AccountID = *accountID
	}
	return m.store.CreateEvent(context.Background(), &evt)
}

func (m *model) completeEventSave(message string) {
	m.eventWizard = newEventWizard(nil)
	m.infoMessage = message
	m.popState()
	if m.state == stateAccountDetail {
		m.refreshAccountDetailAccount()
		m.loadAccountActivity()
	} else if m.state == stateMainMenu {
		m.setMenuInput("Choose an option", 32)
	}
	m.refreshDashboard(time.Now().In(m.cfg.Location()))
}

// DASHBOARD
func (m *model) updateDashboard(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if focus := m.ensureMenuInput("Command (t=toggle, r=refresh, /, exit.)", 48); focus != nil {
		cmds = append(cmds, focus)
	}
	var cmd tea.Cmd
	m.menuInput, cmd = m.menuInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		command := strings.TrimSpace(strings.ToLower(m.menuInput.Value()))
		m.menuInput.SetValue("")
		switch command {
		case "t", "toggle":
			if m.dashboard.view == dashboardEvents {
				m.dashboard.view = dashboardActivity
			} else {
				m.dashboard.view = dashboardEvents
			}
		case "r", "refresh":
			m.refreshDashboard(time.Now().In(m.cfg.Location()))
		case "/", "back":
			m.popState()
			if m.state == stateMainMenu {
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			}
		case "exit.", "exit", "quit":
			m.prevStates = nil
			m.state = stateMainMenu
			if focus := m.setMenuInput("Choose an option", 32); focus != nil {
				cmds = append(cmds, focus)
			}
		case "":
			// ignore
		default:
			m.errMessage = "Unknown dashboard command"
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewDashboard() string {
	lines := []string{m.theme.Title.Render("Dashboard")}
	lines = append(lines, m.theme.Faint.Render("Press t to toggle events/activity, r to refresh, '/' to go back."))
	lines = append(lines, "")
	if m.dashboard.view == dashboardEvents {
		now := time.Now().In(m.cfg.Location())
		today, upcoming, past := storage.SplitEvents(m.dashboard.events, now)
		lines = append(lines, m.theme.Subtitle.Render("Today's Events"))
		if len(today) == 0 {
			lines = append(lines, m.theme.Faint.Render("Nothing scheduled today."))
		}
		for _, e := range today {
			lines = append(lines, m.theme.Success.Render(formatEventLine(m, e)))
		}
		lines = append(lines, "")
		lines = append(lines, m.theme.Subtitle.Render("Upcoming"))
		if len(upcoming) == 0 {
			lines = append(lines, m.theme.Faint.Render("No upcoming events."))
		}
		for i, e := range upcoming {
			if i >= 5 {
				break
			}
			lines = append(lines, m.theme.Warning.Render(formatEventLine(m, e)))
		}
		lines = append(lines, "")
		lines = append(lines, m.theme.Subtitle.Render("Recent"))
		if len(past) == 0 {
			lines = append(lines, m.theme.Faint.Render("No recent events."))
		}
		for i, e := range past {
			if i >= 3 {
				break
			}
			lines = append(lines, m.theme.Danger.Render(formatEventLine(m, e)))
		}
	} else {
		lines = append(lines, m.theme.Subtitle.Render("Recent CRM Activity"))
		if len(m.dashboard.activity) == 0 {
			lines = append(lines, m.theme.Faint.Render("No activity yet."))
		}
		for _, a := range m.dashboard.activity {
			stamp := a.CreatedAt.In(m.cfg.Location()).Format("Jan 02 15:04")
			item := fmt.Sprintf("[%s] %s — %s", strings.ToUpper(a.Type[:1])+a.Type[1:], a.Title, stamp)
			colorized := m.theme.Primary.Render(item)
			switch a.Type {
			case "account":
				colorized = m.theme.Accent.Render(item)
			case "note":
				colorized = m.theme.Success.Render(item)
			case "event":
				colorized = m.theme.Warning.Render(item)
			}
			lines = append(lines, colorized)
		}
	}
	if m.infoMessage != "" {
		lines = append(lines, "", m.theme.Success.Render(m.infoMessage))
	}
	if m.errMessage != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.errMessage))
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatEventLine(m *model, e storage.Event) string {
	when := e.EventTime.In(m.cfg.Location()).Format("Mon Jan 02 15:04")
	var builder strings.Builder
	builder.WriteString(when)
	builder.WriteString(" — ")
	builder.WriteString(e.Title)
	if e.AccountName.Valid {
		builder.WriteString(" (" + e.AccountName.String + ")")
	}
	if e.Details != "" {
		builder.WriteString(" • " + e.Details)
	}
	builder.WriteString(" • by ")
	builder.WriteString(e.Creator)
	return builder.String()
}

// SETTINGS
func (m *model) updateSettings(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch m.settings.mode {
	case settingsViewing:
		if focus := m.ensureMenuInput("1=Name  2=Timezone  3=Back", 40); focus != nil {
			cmds = append(cmds, focus)
		}
		var cmd tea.Cmd
		m.menuInput, cmd = m.menuInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(strings.ToLower(m.menuInput.Value()))
			m.menuInput.SetValue("")
			switch value {
			case "1", "name":
				m.settings.mode = settingsEditingName
				m.settings.input = textinput.New()
				m.settings.input.Prompt = ""
				m.settings.input.CharLimit = 64
				m.settings.input.SetValue(m.cfg.Config.Name)
				if focus := m.settings.input.Focus(); focus != nil {
					cmds = append(cmds, focus)
				}
			case "2", "timezone":
				m.settings.mode = settingsEditingTimezone
				m.settings.input = textinput.New()
				m.settings.input.Prompt = ""
				m.settings.input.CharLimit = 64
				m.settings.input.SetValue(m.cfg.Config.Timezone)
				if focus := m.settings.input.Focus(); focus != nil {
					cmds = append(cmds, focus)
				}
			case "3", "back", "/":
				m.popState()
				if m.state == stateMainMenu {
					if focus := m.setMenuInput("Choose an option", 32); focus != nil {
						cmds = append(cmds, focus)
					}
				}
			case "exit.", "exit", "quit":
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			default:
				m.settings.err = "Choose 1 or 2 to edit settings"
			}
		}
	case settingsEditingName:
		if !m.settings.input.Focused() {
			if focus := m.settings.input.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.settings.input, cmd = m.settings.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.settings.input.Value())
			switch {
			case isExitCommand(value):
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			case isBackCommand(value):
				m.settings.mode = settingsViewing
			case value == "":
				m.settings.err = "Name cannot be empty"
			default:
				m.cfg.Config.Name = value
				if err := m.cfg.Save(); err != nil {
					m.settings.err = err.Error()
				} else {
					m.settings.err = ""
					m.infoMessage = "Name updated"
					m.settings.mode = settingsViewing
				}
			}
		}
	case settingsEditingTimezone:
		if !m.settings.input.Focused() {
			if focus := m.settings.input.Focus(); focus != nil {
				cmds = append(cmds, focus)
			}
		}
		var cmd tea.Cmd
		m.settings.input, cmd = m.settings.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.settings.input.Value())
			switch {
			case isExitCommand(value):
				m.prevStates = nil
				m.state = stateMainMenu
				if focus := m.setMenuInput("Choose an option", 32); focus != nil {
					cmds = append(cmds, focus)
				}
			case isBackCommand(value):
				m.settings.mode = settingsViewing
			case value == "":
				m.settings.err = "Timezone cannot be empty"
			default:
				if _, err := time.LoadLocation(value); err != nil {
					m.settings.err = "Invalid timezone"
				} else {
					m.cfg.Config.Timezone = value
					if err := m.cfg.Save(); err != nil {
						m.settings.err = err.Error()
					} else {
						m.settings.err = ""
						m.infoMessage = "Timezone updated"
						m.settings.mode = settingsViewing
					}
				}
			}
		}
	}
	return batchCmds(cmds)
}

func (m *model) viewSettings() string {
	lines := []string{m.theme.Title.Render("Settings & Help")}
	lines = append(lines, m.theme.Faint.Render("'/' goes back, 'exit.' returns home."))
	lines = append(lines, "")
	lines = append(lines, m.theme.Secondary.Render("Name: "+m.cfg.Config.Name))
	lines = append(lines, m.theme.Secondary.Render("Timezone: "+m.cfg.Config.Timezone))
	lines = append(lines, "")
	lines = append(lines, m.theme.Highlight.Render("Shortcuts"))
	lines = append(lines, m.theme.HelpKey.Render("/")+" → "+m.theme.HelpValue.Render("Back"))
	lines = append(lines, m.theme.HelpKey.Render("exit.")+" → "+m.theme.HelpValue.Render("Main menu"))
	lines = append(lines, m.theme.HelpKey.Render("Ctrl+C")+" → "+m.theme.HelpValue.Render("Quit"))
	lines = append(lines, "")
	lines = append(lines, m.theme.Highlight.Render("Repo"))
	lines = append(lines, m.theme.Primary.Render("github.com/yourname/crm-term"))
	lines = append(lines, m.theme.Faint.Render("Update server configuration coming soon."))
	lines = append(lines, "")

	switch m.settings.mode {
	case settingsViewing:
		lines = append(lines, m.theme.Secondary.Render("1. Update name"))
		lines = append(lines, m.theme.Secondary.Render("2. Update timezone"))
		lines = append(lines, m.theme.Faint.Render("3. Back"))
		lines = append(lines, "")
		lines = append(lines, m.theme.Accent.Render("> ")+m.menuInput.View())
	case settingsEditingName:
		lines = append(lines, m.theme.Secondary.Render("Enter new name:"))
		lines = append(lines, m.settings.input.View())
	case settingsEditingTimezone:
		lines = append(lines, m.theme.Secondary.Render("Enter timezone (e.g. America/New_York):"))
		lines = append(lines, m.settings.input.View())
	}
	if m.settings.err != "" {
		lines = append(lines, "", m.theme.Danger.Render(m.settings.err))
	}
	if m.infoMessage != "" {
		lines = append(lines, "", m.theme.Success.Render(m.infoMessage))
	}
	return strings.Join(lines, "\n") + "\n"
}
