package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vinayprograms/karya/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	editor := cfg.GeneralConfig.EDITOR
	if editor == "" {
		editor = "vim"
	}

	// Determine the inbox file path from config
	var inboxFile string
	if cfg.Directories.Karya != "" {
		// Use configured karya directory if set
		inboxFile = filepath.Join(cfg.Directories.Karya, "inbox.md")
	} else {
		// Default to project root
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Error getting home directory")
		}
		// In a real setup, we would look for KARYA_DIR environment variable
		inboxFile = filepath.Join(home, "inbox.md")
	}

	// Check if inbox file exists, if not, prompt for creation
	if _, err := os.Stat(inboxFile); os.IsNotExist(err) {
		fmt.Printf("Inbox file not found at '%s'. Create? [Y/n] ", inboxFile)
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)
		if input == "Y" || input == "" {
			// Create the file with default header
			if err := os.MkdirAll(filepath.Dir(inboxFile), 0755); err != nil {
				log.Fatalf("Failed to create directory: %v", err)
			}
			file, err := os.Create(inboxFile)
			if err != nil {
				log.Fatalf("Failed to create inbox file: %v", err)
			}
			defer file.Close()
			fmt.Fprintf(file, "# INBOX\n\n")
			fmt.Println("Inbox file created.")
		} else {
			fmt.Println("Cannot continue...Exiting")
			os.Exit(1)
		}
	}

	// Parse command line flags and arguments using flag package
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.Usage = func() {
		printHelp()
	}

	// Define flags
	help := flag.Bool("h", false, "Show help")
	helpLong := flag.Bool("help", false, "Show help")

	flag.Parse()

	// Check for help flags
	if *help || *helpLong {
		printHelp()
		return
	}

	// Check if there are remaining arguments
	args := flag.Args()

	if len(args) == 0 {
		// Open inbox file in editor immediately
		cmd := exec.Command(editor, inboxFile)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to open editor: %v", err)
		}
		return
	}

	// Handle subcommands - only support explicit add/a commands
	subcommand := args[0]
	if subcommand == "add" || subcommand == "a" {
		// Skip the subcommand and process the rest of the arguments as the task text
		if len(args) < 2 {
			// No task text provided, ask for it interactively using bubbletea
			p := tea.NewProgram(initialModel())
			if _, err := p.Run(); err != nil {
				log.Fatal(err)
			}
			return
		}
		taskText := strings.Join(args[1:], " ")
		// Add directly to file
		taskLine := fmt.Sprintf("TODO: %s", taskText)
		file, err := os.OpenFile(inboxFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error: failed to open inbox file: %v", err))
		}
		defer file.Close()

		if _, err := fmt.Fprintln(file, taskLine); err != nil {
			log.Fatal(fmt.Sprintf("Error: failed to write to inbox file: %v", err))
		}
		fmt.Printf("Added '%s' to inbox\n", taskText)
	} else {
		// Reject direct task arguments to avoid user confusion
		fmt.Println("Error: direct task arguments not supported. Use 'inbox add \"task\"'")
		fmt.Println("For help, run: inbox --help")
		os.Exit(1)
	}
}

func printHelp() {
	help := `inbox - Manage your tasks in the inbox

USAGE:
    inbox [OPTIONS] [TASK]

OPTIONS:
    -h, --help, help    Show this help message

COMMANDS:
    (no command)        Open inbox in editor
    add [TASK]         Add a task to inbox
    a [TASK]           Add a task to inbox (short form)
    -h, --help         Show help

EXAMPLES:
    inbox                          # Open inbox in editor
    inbox add "Write a blog post"  # Add task to inbox
    inbox a "Meeting notes"        # Add task to inbox (short form)

ENVIRONMENT VARIABLES:
    EDITOR             Editor to use (default: vim)
    KARYA_DIR          Root directory for karya files
`
	fmt.Print(help)
}

// Model for the bubbletea input
type model struct {
	textInput textinput.Model
	quitting  bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter task description"
	ti.Focus()
	ti.Width = 50

	return model{
		textInput: ti,
		quitting:  false,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if m.textInput.Value() != "" {
				// Save the value to inbox and quit
				addToInbox(m.textInput.Value())
			}
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf(
		"Enter task description:\n\n%s\n\nPress Ctrl+C to quit",
		m.textInput.View(),
	)
}

func addToInbox(taskText string) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Determine the inbox file path from config
	var inboxFile string
	if cfg.Directories.Karya != "" {
		// Use configured karya directory if set
		inboxFile = filepath.Join(cfg.Directories.Karya, "inbox.md")
	} else {
		// Default to project root
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Error getting home directory")
		}
		// In a real setup, we would look for KARYA_DIR environment variable
		inboxFile = filepath.Join(home, "inbox.md")
	}

	taskLine := fmt.Sprintf("TODO: %s", taskText)
	file, err := os.OpenFile(inboxFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error: failed to open inbox file: %v", err))
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, taskLine); err != nil {
		log.Fatal(fmt.Sprintf("Error: failed to write to inbox file: %v", err))
	}
	fmt.Printf("Added '%s' to inbox\n", taskText)
}
