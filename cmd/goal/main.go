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
	creatingFeedback string
	cfg         *config.Config
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
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.creatingGoal = false
				m.goalTitle = ""
				m.creatingFeedback = ""
				return m, nil
			case "backspace":
				if len(m.goalTitle) > 0 {
					m.goalTitle = m.goalTitle[:len(m.goalTitle)-1]
				}
			case "enter":
				if m.goalTitle != "" {
					return m, m.createGoal(m.currentHorizon, m.goalTitle)
				} else {
					m.creatingGoal = false
					m.goalTitle = ""
					m.creatingFeedback = ""
				}
			default:
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
					m.goalTitle += string(msg.Runes[0])
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
			}
			return m, nil
		case "n":
			m.creatingGoal = true
			m.goalTitle = ""
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
		// Close the creation form
		m.creatingGoal = false
		m.goalTitle = ""
		m.creatingFeedback = ""
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
			Width(60)

		titleStyle := colors.primaryColor.Copy().Bold(true)
		title := titleStyle.Render(fmt.Sprintf("New %s Goal", m.currentHorizon))

		prompt := fmt.Sprintf("Title: %s", m.goalTitle)
		
		var content string
		if m.creatingFeedback != "" {
			feedback := lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Render(m.creatingFeedback)
			content = dialogBox.Render(title + "\n\n" + prompt + "\n\n" + feedback + "\n\n" + 
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Enter to confirm • Esc to cancel"))
		} else {
			content = dialogBox.Render(title + "\n\n" + prompt + "\n\n" + 
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Enter to confirm • Esc to cancel"))
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
	horizon goal.Horizon
}

func (m *Model) createGoal(horizon goal.Horizon, title string) tea.Cmd {
	return func() tea.Msg {
		var currentPeriod string
		switch horizon {
		case goal.HorizonMonthly:
			currentPeriod = "2025-11"
		case goal.HorizonQuarterly:
			currentPeriod = "2025-Q1" 
		case goal.HorizonYearly:
			currentPeriod = "2025"
		case goal.HorizonShortTerm:
			currentPeriod = "2025-2027"
		case goal.HorizonLongTerm:
			currentPeriod = "2025-2035"
		}

		err := m.goalManager.CreateGoal(horizon, currentPeriod, title)
		if err != nil {
			return errMsg(err)
		}

		return goalCreatedMsg{horizon: horizon}
	}
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
