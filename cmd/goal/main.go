package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/goal"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	errMsg error
)

// Karya directory with goals
var goalsDir string

// ColorScheme for the goal TUI
type ColorScheme struct {
	primaryColor   lipgloss.Style
	secondaryColor lipgloss.Style
	accentColor    lipgloss.Style
	selectorStyle  lipgloss.Style
	errorStyle     lipgloss.Style
}

var colors ColorScheme

func InitializeColors(cfg *config.Config) {
	colors = ColorScheme{
		primaryColor:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		secondaryColor: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		accentColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		selectorStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		errorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
	}
}

// Goal represents a single goal with metadata
type Goal struct {
	ID        string
	Title     string
	Period    string
	Horizon   goal.Horizon
	CreatedAt time.Time
}

// GoalItem represents an item in a list
type GoalItem struct {
	goal Goal
}

func (i GoalItem) FilterValue() string {
	return i.goal.Title
}

func (i GoalItem) renderWithSelection(isSelected bool, horizon goal.Horizon) string {
	var parts []string
	periodStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	// For monthly goals, show 3 columns: Year, Month, Goal
	// Year: 4 chars + 2 padding = 6
	// Month: "September" (9 chars) + 2 padding = 11
	if horizon == goal.HorizonMonthly {
		yearMonthSplit := strings.Split(i.goal.Period, "-")
		if len(yearMonthSplit) >= 2 {
			year := yearMonthSplit[0]
			month := yearMonthSplit[1]
			monthName := getMonthName(month)
			
			parts = append(parts, " "+periodStyle.Render(year)+" ")
			parts = append(parts, " "+periodStyle.Render(fmt.Sprintf("%-9s", monthName))+" ")
		}
	} else if horizon == goal.HorizonQuarterly {
		// For quarterly goals, show 3 columns: Year, Quarter, Goal
		// Year: 4 chars + 2 padding = 6
		// Quarter: "Q1" (2 chars) + 2 padding = 4
		// Expected format: 2025-Q1
		periodParts := strings.Split(i.goal.Period, "-")
		if len(periodParts) >= 2 {
			year := periodParts[0]
			quarter := periodParts[1]
			
			parts = append(parts, " "+periodStyle.Render(year)+" ")
			parts = append(parts, " "+periodStyle.Render(quarter)+" ")
		}
	} else if horizon == goal.HorizonYearly {
		// For yearly goals, show 2 columns: Year, Goal
		// Year: 4 chars + 2 padding = 6
		parts = append(parts, " "+periodStyle.Render(i.goal.Period)+" ")
	} else if horizon == goal.HorizonShortTerm || horizon == goal.HorizonLongTerm {
		// For short-term and long-term goals, show 2 columns: Year Range, Goal
		// Expected format: 2025-2027 or 2025-2035
		// Range: "2025-2027" (9 chars) + 2 padding = 11
		parts = append(parts, " "+periodStyle.Render(fmt.Sprintf("%-9s", i.goal.Period))+" ")
	}
	
	if isSelected {
		indicator := colors.selectorStyle.Render("█ ")
		parts = append(parts, indicator+i.goal.Title)
	} else {
		parts = append(parts, "  "+i.goal.Title)
	}

	return strings.Join(parts, " ")
}

func (i GoalItem) Title() string {
	return i.goal.Title
}

func (i GoalItem) Description() string {
	return ""
}

// Key map for goal TUI
type KeyMap struct {
	Enter     key.Binding
	Quit      key.Binding
	Help      key.Binding
	Refresh   key.Binding
	NewGoal   key.Binding
	EditGoal  key.Binding
	Back      key.Binding
	Tab1      key.Binding
	Tab2      key.Binding
	Tab3      key.Binding
	Tab4      key.Binding
	Tab5      key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Quit, k.Help, k.Refresh, k.NewGoal}
}

func (k KeyMap) FullHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Quit, k.Help, k.Refresh, k.NewGoal, k.EditGoal, k.Back, k.Tab1, k.Tab2, k.Tab3, k.Tab4, k.Tab5}
}

var keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "edit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	NewGoal: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new goal"),
	),
	EditGoal: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit goal"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Tab1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "monthly"),
	),
	Tab2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "quarterly"),
	),
	Tab3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "yearly"),
	),
	Tab4: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "short-term"),
	),
	Tab5: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "long-term"),
	),
}

// Model represents the TUI state
type Model struct {
	goalManager *goal.GoalManager
	currentHorizon goal.Horizon
	goalLists   map[goal.Horizon]*list.Model
	quitting    bool
	editor      string
	creatingGoal bool
	goalTitle   string
	goalPeriod  string // The period string (e.g., "2025-11", "2025-Q1", "2025")
	focusedField int   // 0 = title, 1 = period
	editingPeriod bool // Whether period field is being edited
	creatingFeedback string
	cfg         *config.Config
	termWidth   int
	termHeight  int
	openAfterCreate bool // Whether to open in editor after creating
}

// NewModel creates a new TUI model
func NewModel() (Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return Model{}, err
	}

	karyaDir := cfg.Directories.Karya
	if karyaDir == "" {
		if cfg.Directories.Projects != "" {
			karyaDir = cfg.Directories.Projects
		} else {
			home, _ := os.UserHomeDir()
			karyaDir = filepath.Join(home, ".karya")
		}
	}
	goalsDir = filepath.Join(karyaDir, ".goals")

	goalManager := goal.NewGoalManager(goalsDir)
	editor := cfg.GeneralConfig.EDITOR
	if editor == "" {
		editor = "vim"
	}

	if err := os.MkdirAll(goalsDir, 0755); err != nil {
		return Model{}, err
	}

	goalLists := make(map[goal.Horizon]*list.Model)
	
	for _, hor := range []goal.Horizon{
		goal.HorizonMonthly,
		goal.HorizonQuarterly,
		goal.HorizonYearly,
		goal.HorizonShortTerm,
		goal.HorizonLongTerm,
	} {
		listModel := NewGoalList(hor, goalManager)
		goalLists[hor] = &listModel
	}

	// Initialize colors from config
	InitializeColors(cfg)

	return Model{
		goalManager:      goalManager,
		currentHorizon:   goal.HorizonMonthly,
		goalLists:        goalLists,
		editor:           editor,
		creatingGoal:     false,
		creatingFeedback: "",
		cfg:              cfg,
		termWidth:        0,
		termHeight:       0,
	}, nil
}

// goalDelegate is a custom delegate for rendering goal items
type goalDelegate struct {
	list.DefaultDelegate
	horizon goal.Horizon
}

func (d goalDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	goalItem, ok := item.(GoalItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	content := goalItem.renderWithSelection(isSelected, d.horizon)
	fmt.Fprint(w, content)
}

// NewGoalList initializes a goal list for a specific horizon
func NewGoalList(horizon goal.Horizon, goalManager *goal.GoalManager) list.Model {
	goals, err := goalManager.ListGoalsByHorizon(horizon)
	if err != nil {
		goals = make(map[string][]string)
	}

	var items []list.Item
	for period, goalTitles := range goals {
		for _, title := range goalTitles {
			goal := Goal{
				ID:      title,
				Title:   title,
				Period:  period,
				Horizon: horizon,
			}
			items = append(items, GoalItem{goal: goal})
		}
	}
	
	// Sort items for monthly goals by year and month
	if horizon == goal.HorizonMonthly {
		sortMonthlyGoals(items)
	}

	// Use custom delegate
	delegate := goalDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
		horizon:        horizon,
	}
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	listModel := list.New(items, delegate, 0, 0)
	listModel.Title = "Goals"
	listModel.SetShowStatusBar(false)
	listModel.SetShowTitle(true)
	listModel.SetFilteringEnabled(false)
	listModel.KeyMap.Quit.SetKeys("q", "ctrl+c")
	listModel.KeyMap.AcceptWhileFiltering.SetEnabled(false)
	listModel.KeyMap.CancelWhileFiltering.SetEnabled(false)
	listModel.KeyMap.ClearFilter.SetEnabled(false)
	listModel.KeyMap.NextPage.SetKeys("pgdown", "ctrl+f", "ctrl+d")
	listModel.KeyMap.PrevPage.SetKeys("pgup", "ctrl+b", "ctrl+u")

	listModel.AdditionalShortHelpKeys = func() []key.Binding {
		return keys.ShortHelp()
	}
	listModel.AdditionalFullHelpKeys = func() []key.Binding {
		return keys.FullHelp()
	}

	return listModel
}

// sortMonthlyGoals sorts monthly goal items by year and month
func sortMonthlyGoals(items []list.Item) {
	monthNameToNum := map[string]int{
		"January": 1, "February": 2, "March": 3, "April": 4,
		"May": 5, "June": 6, "July": 7, "August": 8,
		"September": 9, "October": 10, "November": 11, "December": 12,
	}
	
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			gi1, ok1 := items[i].(GoalItem)
			gi2, ok2 := items[j].(GoalItem)
			if !ok1 || !ok2 {
				continue
			}
			
			period1 := strings.Split(gi1.goal.Period, "-")
			period2 := strings.Split(gi2.goal.Period, "-")
			
			if len(period1) < 2 || len(period2) < 2 {
				continue
			}
			
			year1, month1 := period1[0], period1[1]
			year2, month2 := period2[0], period2[1]
			
			monthNum1 := monthNameToNum[getMonthName(month1)]
			monthNum2 := monthNameToNum[getMonthName(month2)]
			
			// Sort by year first, then by month number
			if year1 > year2 || (year1 == year2 && monthNum1 > monthNum2) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}

		if m.creatingGoal {
			// Handle special keys first (non-printable)
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.creatingGoal = false
				m.goalTitle = ""
				m.goalPeriod = ""
				m.focusedField = 0
				m.editingPeriod = false
				m.openAfterCreate = false
				m.creatingFeedback = ""
				return m, nil
			case "tab":
				// Cycle between title and period fields
				m.focusedField = (m.focusedField + 1) % 2
				return m, nil
			case "backspace":
				if m.focusedField == 0 {
					// Editing title
					if len(m.goalTitle) > 0 {
						m.goalTitle = m.goalTitle[:len(m.goalTitle)-1]
					}
				} else if m.editingPeriod {
					// Editing period
					if len(m.goalPeriod) > 0 {
						m.goalPeriod = m.goalPeriod[:len(m.goalPeriod)-1]
					}
				}
				return m, nil
			case "shift+enter":
				// Create goal and open in editor
				if m.goalTitle != "" {
					m.openAfterCreate = true
					return m, m.createGoal(m.currentHorizon, m.goalTitle, m.goalPeriod)
				}
				return m, nil
			case "enter":
				// Create goal
				if m.goalTitle != "" {
					m.openAfterCreate = false
					return m, m.createGoal(m.currentHorizon, m.goalTitle, m.goalPeriod)
				} else {
					m.creatingGoal = false
					m.goalTitle = ""
					m.goalPeriod = ""
					m.focusedField = 0
					m.editingPeriod = false
					m.creatingFeedback = ""
				}
				return m, nil
			}
			
			// Handle single character keys
			// Check if this is "c" or "e" pressed on period field to enable editing
			if m.focusedField == 1 && !m.editingPeriod && (msg.String() == "c" || msg.String() == "e") {
				m.editingPeriod = true
				return m, nil
			}
			
			// Handle normal text input for both fields
			if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
				if m.focusedField == 0 {
					// Editing title - accept all printable characters
					m.goalTitle += string(msg.Runes[0])
				} else if m.editingPeriod {
					// Editing period - accept all printable characters
					m.goalPeriod += string(msg.Runes[0])
				}
			}
			
			return m, nil
		}

		switch msg.String() {
		case "r":
			for _, hor := range []goal.Horizon{
				goal.HorizonMonthly,
				goal.HorizonQuarterly,
				goal.HorizonYearly,
				goal.HorizonShortTerm,
				goal.HorizonLongTerm,
			} {
				*m.goalLists[hor] = NewGoalList(hor, m.goalManager)
				// Restore the terminal dimensions to the refreshed list
				if m.termWidth > 0 && m.termHeight > 0 {
					m.goalLists[hor].SetWidth(m.termWidth)
					m.goalLists[hor].SetHeight(m.termHeight - 3)
				}
			}
			return m, nil
		case "n":
			m.creatingGoal = true
			m.goalTitle = ""
			m.goalPeriod = m.getDefaultPeriod(m.currentHorizon)
			m.focusedField = 0 // Start with title field
			m.editingPeriod = false
			m.openAfterCreate = false
			m.creatingFeedback = ""
			return m, nil
		case "e", "enter":
			return m, m.editGoal()
		case "1":
			m.currentHorizon = goal.HorizonMonthly
			return m, nil
		case "2":
			m.currentHorizon = goal.HorizonQuarterly
			return m, nil
		case "3":
			m.currentHorizon = goal.HorizonYearly
			return m, nil
		case "4":
			m.currentHorizon = goal.HorizonShortTerm
			return m, nil
		case "5":
			m.currentHorizon = goal.HorizonLongTerm
			return m, nil
		}

	case tea.WindowSizeMsg:
		// Store terminal dimensions
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		
		// Update all lists with new dimensions
		for _, hor := range []goal.Horizon{
			goal.HorizonMonthly,
			goal.HorizonQuarterly,
			goal.HorizonYearly,
			goal.HorizonShortTerm,
			goal.HorizonLongTerm,
		} {
			m.goalLists[hor].SetWidth(msg.Width)
			// Reserve space for: tabs (1) + 2 spacing lines (2) = 3 lines
			// The list title and help are handled internally by the list component
			m.goalLists[hor].SetHeight(msg.Height - 3)
		}
		return m, nil

	case goalCreatedMsg:
		// Refresh the goal list for the horizon
		*m.goalLists[msg.horizon] = NewGoalList(msg.horizon, m.goalManager)
		// Restore the terminal dimensions to the refreshed list
		if m.termWidth > 0 && m.termHeight > 0 {
			m.goalLists[msg.horizon].SetWidth(m.termWidth)
			m.goalLists[msg.horizon].SetHeight(m.termHeight - 3)
		}
		// Close the creation form
		m.creatingGoal = false
		m.goalTitle = ""
		m.goalPeriod = ""
		m.focusedField = 0
		m.editingPeriod = false
		m.openAfterCreate = false
		m.creatingFeedback = ""
		
		// If user pressed Shift+Enter, open the goal in editor
		if msg.openInEditor {
			return m, m.openGoalInEditor(msg.goalPath)
		}
		
		return m, nil

	case errMsg:
		m.creatingFeedback = fmt.Sprintf("Error: %v", msg)
		return m, nil
	}

	if !m.creatingGoal {
		newModel, cmd := m.goalLists[m.currentHorizon].Update(msg)
		m.goalLists[m.currentHorizon] = &newModel
		return m, cmd
	}

	return m, nil
}

func getMonthName(month string) string {
	months := map[string]string{
		"01": "January",
		"02": "February",
		"03": "March",
		"04": "April",
		"05": "May",
		"06": "June",
		"07": "July",
		"08": "August",
		"09": "September",
		"10": "October",
		"11": "November",
		"12": "December",
	}
	
	if name, exists := months[month]; exists {
		return name
	}
	
	return month
}



func (m Model) View() string {
	if m.quitting {
		return ""
	}

	horizons := []goal.Horizon{
		goal.HorizonMonthly,
		goal.HorizonQuarterly,
		goal.HorizonYearly,
		goal.HorizonShortTerm,
		goal.HorizonLongTerm,
	}

	var tabItems []string
	for i, hor := range horizons {
		tabNum := i + 1
		if hor == m.currentHorizon {
			activeTab := lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color(m.cfg.Colors.ProjectColor)).
				Render(fmt.Sprintf("%d.%s", tabNum, hor))
			tabItems = append(tabItems, activeTab)
		} else {
			inactiveTab := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(fmt.Sprintf("%d.%s", tabNum, hor))
			tabItems = append(tabItems, inactiveTab)
		}
	}
	tabs := strings.Join(tabItems, "  ")

	listView := m.goalLists[m.currentHorizon].View()
	
	// Split the list view to insert tabs after the title
	lines := strings.Split(listView, "\n")
	if len(lines) > 0 {
		// First line is the title, insert tabs after it
		result := []string{lines[0], "", tabs, ""}
		result = append(result, lines[1:]...)
		listView = strings.Join(result, "\n")
	}

	if m.creatingGoal {
		// Create modal dialog box
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(m.cfg.Colors.ProjectColor)).
			Padding(1, 2).
			Width(70)

		titleStyle := colors.primaryColor.Copy().Bold(true)
		title := titleStyle.Render(fmt.Sprintf("New %s Goal", m.currentHorizon))

		// Render title field with cursor/focus indicator
		titleLabel := "Title: "
		titleValue := m.goalTitle
		if m.focusedField == 0 {
			titleValue = titleValue + "█" // Show cursor
			titleLabel = lipgloss.NewStyle().Bold(true).Render(titleLabel)
		}
		titleLine := titleLabel + titleValue

		// Render period field with cursor/focus indicator
		periodLabel := "Period: "
		periodValue := m.goalPeriod
		var periodHint string
		if m.focusedField == 1 {
			if m.editingPeriod {
				periodValue = periodValue + "█" // Show cursor when editing
			} else {
				periodHint = lipgloss.NewStyle().
					Foreground(lipgloss.Color("8")).
					Render(" (press 'c' or 'e' to edit)")
			}
			periodLabel = lipgloss.NewStyle().Bold(true).Render(periodLabel)
		}
		periodLine := periodLabel + periodValue + periodHint

		// Help text
		helpText := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
			"TAB: switch fields • Enter: create • Shift+Enter: create and edit • Esc: cancel")
		
		var content string
		if m.creatingFeedback != "" {
			feedback := lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Render(m.creatingFeedback)
			content = dialogBox.Render(title + "\n\n" + titleLine + "\n" + periodLine + "\n\n" + feedback + "\n\n" + helpText)
		} else {
			content = dialogBox.Render(title + "\n\n" + titleLine + "\n" + periodLine + "\n\n" + helpText)
		}

		// Overlay the dialog on top of the list view
		return lipgloss.Place(
			lipgloss.Width(listView),
			lipgloss.Height(listView),
			lipgloss.Center,
			lipgloss.Center,
			content,
		)
	}

	return listView
}

type goalCreatedMsg struct {
	horizon   goal.Horizon
	goalPath  string
	openInEditor bool
}

func (m *Model) createGoal(horizon goal.Horizon, title string, period string) tea.Cmd {
	return func() tea.Msg {
		// Use the provided period directly
		err := m.goalManager.CreateGoal(horizon, period, title)
		if err != nil {
			return errMsg(err)
		}

		// Get the path to the created goal
		goalPath := m.goalManager.GetGoalPathForHorizon(horizon, period, title)

		return goalCreatedMsg{
			horizon:      horizon,
			goalPath:     goalPath,
			openInEditor: m.openAfterCreate,
		}
	}
}

// getNextQuarterPeriod returns the next quarter in "YYYY-QN" format
// based on the configured fiscal year start month
// The year component represents the fiscal year, not calendar year
func getNextQuarterPeriod(yearStart string) string {
	// Map month names to numbers for easier calculation
	monthMap := map[string]int{
		"January":   1, "February":  2, "March":     3,
		"April":     4, "May":       5, "June":      6,
		"July":      7, "August":    8, "September": 9,
		"October":  10, "November": 11, "December": 12,
	}

	startMonth, ok := monthMap[yearStart]
	if !ok {
		startMonth = 1 // Default to January
	}

	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())
	
	// First, determine the current fiscal year
	// If we're before the fiscal year start month, we're in the previous fiscal year
	var fiscalYear int
	if currentMonth < startMonth {
		fiscalYear = currentYear
	} else {
		// We're at or after the fiscal year start, so we're in next fiscal year
		fiscalYear = currentYear + 1
	}
	
	// Calculate which quarter (0-3) we're currently in based on the start month
	// In the sequence:
	// Q1: startMonth to (startMonth + 2) % 12
	// Q2: startMonth+3 to (startMonth + 5) % 12
	// Q3: startMonth+6 to (startMonth + 8) % 12
	// Q4: startMonth+9 to (startMonth + 11) % 12
	
	// Get offset from start month and compute (with proper cycle wraparound)  
	monthOffset := (currentMonth - startMonth + 12) % 12
	quarterIndex := monthOffset / 3 // 0 for Q1, 1 for Q2, 2 for Q3, 3 for Q4
	
	// Convert to 1-based quarter numbers
	quarter := quarterIndex + 1
	
	// Get the next quarter in sequence
	nextQuarter := quarter + 1
	nextFiscalYear := fiscalYear
	
	if nextQuarter > 4 {
		// Wrapped to next fiscal year
		nextQuarter = 1
		nextFiscalYear++
	}
	
	// Return the full period string with fiscal year
	return fmt.Sprintf("%d-Q%d", nextFiscalYear, nextQuarter)
}

// getNextYear returns the next fiscal year based on the configured fiscal year start month
// For example, if year start is June and current date is April 2025,
// we are in fiscal year 2024 (which started in June 2024), so next year is 2025
// If current date is July 2025, we are in fiscal year 2025 (started June 2025), so next year is 2026
func getNextYear(yearStart string) int {
	// Map month names to numbers
	monthMap := map[string]int{
		"January":   1, "February":  2, "March":     3,
		"April":     4, "May":       5, "June":      6,
		"July":      7, "August":    8, "September": 9,
		"October":  10, "November": 11, "December": 12,
	}

	startMonth, ok := monthMap[yearStart]
	if !ok {
		startMonth = 1 // Default to January
	}

	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())
	
	// Determine the fiscal year we're currently in
	// If the quarter/year starts in June (month 6):
	// - Months Jan-May (1-5) belong to the previous fiscal year
	// - Months Jun-Dec (6-12) belong to the current fiscal year
	
	var fiscalYear int
	if currentMonth < startMonth {
		// We're before the fiscal year start, so we're in the previous fiscal year
		fiscalYear = currentYear - 1
	} else {
		// We're at or after the fiscal year start, so we're in the current fiscal year
		fiscalYear = currentYear
	}
	
	// The next fiscal year is simply fiscalYear + 1
	return fiscalYear + 1
}

// getDefaultPeriod returns the default period string for the given horizon
func (m *Model) getDefaultPeriod(horizon goal.Horizon) string {
	// Get the configured year start month
	yearStart := "January"
	if m.cfg.Goals.YearStart != "" {
		yearStart = m.cfg.Goals.YearStart
	}
	
	switch horizon {
	case goal.HorizonMonthly:
		// Next month
		now := time.Now()
		nextMonth := now.AddDate(0, 1, 0)
		return fmt.Sprintf("%d-%02d", nextMonth.Year(), nextMonth.Month())
	case goal.HorizonQuarterly:
		// Next quarter
		return getNextQuarterPeriod(yearStart)
	case goal.HorizonYearly:
		// Next year
		nextYear := getNextYear(yearStart)
		return fmt.Sprintf("%d", nextYear)
	case goal.HorizonShortTerm:
		// 3 years from now
		now := time.Now()
		endYear := now.Year() + 3
		return fmt.Sprintf("%d-%d", now.Year(), endYear)
	case goal.HorizonLongTerm:
		// 10 years from now
		now := time.Now()
		endYear := now.Year() + 10
		return fmt.Sprintf("%d-%d", now.Year(), endYear)
	}
	return ""
}

// openGoalInEditor opens the specified goal file in the configured editor
func (m *Model) openGoalInEditor(goalPath string) tea.Cmd {
	editor := m.editor
	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]
	editorArgs = append(editorArgs, goalPath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg(err)
		}
		return nil
	})
}

func (m *Model) editGoal() tea.Cmd {
	item := m.goalLists[m.currentHorizon].SelectedItem()
	if item == nil {
		return nil
	}

	if goalItem, ok := item.(GoalItem); ok {
		goalPath := m.goalManager.GetGoalPathForHorizon(m.currentHorizon, goalItem.goal.Period, goalItem.goal.Title)
		
		editor := m.editor
		if strings.HasPrefix(editor, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				editor = filepath.Join(home, editor[2:])
			}
		}

		editorParts := strings.Fields(editor)
		editorCmd := editorParts[0]
		editorArgs := editorParts[1:]
		editorArgs = append(editorArgs, goalPath)

		c := exec.Command(editorCmd, editorArgs...)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return errMsg(err)
			}
			return nil
		})
	}

	return nil
}

func main() {
	model, err := NewModel()
	if err != nil {
		log.Fatal(err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
