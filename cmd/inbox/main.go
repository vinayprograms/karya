package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/vinayprograms/karya/internal/colors"
	"github.com/vinayprograms/karya/internal/config"
	editorpkg "github.com/vinayprograms/karya/internal/editor"
)

type options struct {
	command string
	title   []string
	stdin   bool
	quiet   bool
	json    bool
	raw     bool
	keyword string
}

type receipt struct {
	Path      string     `json:"path"`
	Tasks     []string   `json:"tasks"`
	Selectors []selector `json:"selectors"`
}

type selector struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Expected string `json:"expected"`
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, isatty.IsTerminal(os.Stdin.Fd())); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parse(args []string) (options, error) {
	selected := options{command: "edit"}
	if len(args) == 0 {
		return selected, nil
	}
	selected.command = args[0]
	args = args[1:]
	switch selected.command {
	case "-h", "--help", "help":
		selected.command = "help"
		if len(args) == 1 && slices.Contains([]string{"add", "a", "edit", "show", "colors"}, args[0]) {
			args = nil
		}
		if len(args) != 0 {
			return selected, errors.New("usage: inbox help [add|edit|show|colors]")
		}
	case "show":
		flags := flag.NewFlagSet("show", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.BoolVar(&selected.raw, "raw", false, "print original bytes without colors")
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				selected.command = "help"
				return selected, nil
			}
			return selected, err
		}
		if flags.NArg() != 0 {
			return selected, errors.New("show accepts no operands")
		}
	case "colors", "edit":
		if len(args) == 1 && slices.Contains([]string{"-h", "--help"}, args[0]) {
			selected.command = "help"
		} else if len(args) != 0 {
			return selected, fmt.Errorf("%s accepts no arguments", selected.command)
		}
	case "add", "a":
		selected.command = "add"
		flags := flag.NewFlagSet("add", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.BoolVar(&selected.stdin, "stdin", false, "read tasks from stdin")
		flags.BoolVar(&selected.quiet, "quiet", false, "suppress success output")
		flags.BoolVar(&selected.json, "json", false, "print JSON results")
		flags.StringVar(&selected.keyword, "keyword", "", "fallback task keyword")
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				selected.command = "help"
				return selected, nil
			}
			return selected, err
		}
		selected.title = flags.Args()
		if selected.quiet && selected.json {
			return selected, errors.New("--quiet and --json cannot be combined")
		}
		if selected.stdin && len(selected.title) != 0 {
			return selected, errors.New("--stdin cannot be combined with a title")
		}
		flags.Visit(func(value *flag.Flag) {
			if value.Name == "keyword" && selected.keyword == "" {
				selected.keyword = " "
			}
		})
	default:
		return selected, fmt.Errorf("unknown command %q; use inbox help", selected.command)
	}
	return selected, nil
}

func run(args []string, input io.Reader, output, diagnostics io.Writer, terminal bool) error {
	selected, err := parse(args)
	if err != nil {
		return err
	}
	if selected.command == "help" {
		_, err := io.WriteString(output, usage)
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if selected.command == "colors" {
		return colors.Print(cfg)
	}
	path, err := filepath.Abs(cfg.GetInboxFilePath())
	if err != nil {
		return err
	}
	if selected.command == "show" {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("inbox must be a regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if destination, ok := output.(interface{ Fd() uintptr }); ok && !selected.raw && isatty.IsTerminal(destination.Fd()) {
			data = []byte(render(string(data), cfg, lipgloss.NewRenderer(output)))
		}
		count, err := output.Write(data)
		if err == nil && count != len(data) {
			return io.ErrShortWrite
		}
		return err
	}
	if selected.command == "edit" {
		if !terminal {
			return errors.New("editor requires a terminal; use inbox add to capture tasks")
		}
		return edit(cfg.GeneralConfig.EDITOR, path, input, output, diagnostics)
	}
	keywords := slices.Concat(cfg.Todo.Active, cfg.Todo.InProgress, cfg.Todo.Completed, cfg.Todo.Someday, cfg.Todo.Events)
	keyword := selected.keyword
	if keyword == "" {
		if len(cfg.Todo.Active) == 0 {
			return errors.New("no active task keyword configured")
		}
		keyword = cfg.Todo.Active[0]
	}
	if !slices.Contains(keywords, keyword) {
		return fmt.Errorf("unknown task keyword %q", keyword)
	}
	for _, configured := range keywords {
		if configured == "" || strings.ContainsFunc(configured, func(char rune) bool {
			return unicode.IsSpace(char) || unicode.IsControl(char) || char == ':'
		}) {
			return fmt.Errorf("invalid configured task keyword %q", configured)
		}
	}
	var source io.Reader = input
	if len(selected.title) != 0 {
		if !terminal {
			var probe [1]byte
			count, err := io.ReadFull(input, probe[:])
			if count != 0 {
				return errors.New("title arguments and stdin content cannot be combined")
			}
			if err != io.EOF {
				return fmt.Errorf("read stdin: %w", err)
			}
		}
		title := strings.Join(selected.title, " ")
		if strings.ContainsAny(title, "\r\n") {
			return errors.New("title must be a single line; use --stdin for batches")
		}
		source = strings.NewReader(title)
	} else if terminal && !selected.stdin {
		program := tea.NewProgram(initial(), tea.WithInput(input), tea.WithOutput(diagnostics))
		final, err := program.Run()
		if err != nil {
			return err
		}
		captured := final.(model)
		if !captured.accepted {
			return nil
		}
		source = strings.NewReader(captured.input.Value())
	}
	tasks, err := capture(source, keyword, keywords)
	if err != nil {
		return err
	}
	line, err := store(path, tasks)
	if err != nil {
		return err
	}
	if selected.quiet {
		return nil
	}
	if selected.json {
		selectors := make([]selector, len(tasks))
		for index, task := range tasks {
			selectors[index] = selector{Path: path, Line: line + index, Expected: task}
		}
		err = json.NewEncoder(output).Encode(receipt{Path: path, Tasks: tasks, Selectors: selectors})
	} else {
		_, err = fmt.Fprintf(output, "Added %d task(s) to %s\n", len(tasks), path)
	}
	if err != nil {
		return fmt.Errorf("tasks saved but result output failed: %w", err)
	}
	return nil
}

func render(content string, cfg *config.Config, renderer *lipgloss.Renderer) string {
	styles := colors.Styles(cfg)
	paint := func(style lipgloss.Style, text string) string {
		return style.Renderer(renderer).Render(text)
	}
	tokens := regexp.MustCompile(`>>[^\r\n]*|#[\p{L}\p{N}_:./-]+|@(?:s:|d:)?[^\s]+`)
	var output strings.Builder
	for segment := range strings.SplitAfterSeq(content, "\n") {
		line := strings.TrimRight(segment, "\r\n")
		ending := segment[len(line):]
		base := styles.Task
		keywordStyle := styles.Task
		start, end := -1, -1
		prefix := len(line) - len(strings.TrimLeft(line, " \t"))
		candidate := line[prefix:]
		if strings.HasPrefix(candidate, "- ") || strings.HasPrefix(candidate, "* ") {
			prefix += 2
			candidate = line[prefix:]
		}
		if keyword, _, found := strings.Cut(candidate, ":"); found {
			switch {
			case slices.Contains(cfg.Todo.Events, keyword):
				keywordStyle = styles.Events
			case slices.Contains(cfg.Todo.Completed, keyword):
				keywordStyle, base = styles.Completed, styles.CompletedTask
			case slices.Contains(cfg.Todo.InProgress, keyword):
				keywordStyle = styles.InProgress
			case slices.Contains(cfg.Todo.Active, keyword):
				keywordStyle = styles.Active
			case slices.Contains(cfg.Todo.Someday, keyword):
				keywordStyle = styles.Someday
			default:
				keyword = ""
			}
			if keyword != "" {
				start, end = prefix, prefix+len(keyword)+1
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			output.WriteString(paint(styles.Project, line))
			output.WriteString(ending)
			continue
		}
		position := 0
		if start >= 0 {
			output.WriteString(line[:start])
			output.WriteString(paint(keywordStyle, line[start:end]))
			position = end
		}
		remainder := line[position:]
		cursor := 0
		for _, match := range tokens.FindAllStringIndex(remainder, -1) {
			output.WriteString(paint(base, remainder[cursor:match[0]]))
			token := remainder[match[0]:match[1]]
			style := styles.Tag
			switch {
			case strings.HasPrefix(token, ">>"):
				style = styles.Assignee
			case strings.HasPrefix(token, "@"):
				style = styles.Date
				date := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(token, "@"), "s:"), "d:")
				if len(date) >= 10 {
					if parsed, err := time.Parse("2006-01-02", date[:10]); err == nil {
						today := time.Now().Format("2006-01-02")
						if parsed.Format("2006-01-02") == today {
							style = styles.TodayDate
						} else if date[:10] < today {
							style = styles.PastDate
						}
					}
				}
			case strings.HasPrefix(token, "#"):
				for _, special := range cfg.Todo.SpecialTags {
					name := strings.TrimPrefix(token, "#")
					if name == special || strings.HasPrefix(name, special+":") {
						style = styles.SpecialTag
						break
					}
				}
			}
			output.WriteString(paint(style, token))
			cursor = match[1]
		}
		output.WriteString(paint(base, remainder[cursor:]))
		output.WriteString(ending)
	}
	return output.String()
}

func capture(input io.Reader, fallback string, keywords []string) ([]string, error) {
	reader := bufio.NewReader(input)
	var tasks []string
	for number := 1; ; number++ {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read input line %d: %w", number, err)
		}
		line = strings.TrimSpace(line)
		if line != "" {
			if !utf8.ValidString(line) || strings.ContainsFunc(line, func(char rune) bool { return unicode.IsControl(char) && char != '\t' }) {
				return nil, fmt.Errorf("input line %d contains invalid text", number)
			}
			keyword, title, recognized := strings.Cut(line, ":")
			if recognized && slices.Contains(keywords, keyword) {
				if strings.TrimSpace(title) == "" {
					return nil, fmt.Errorf("input line %d has an empty task title", number)
				}
			} else {
				line = fallback + ": " + line
			}
			tasks = append(tasks, line)
		}
		if err == io.EOF {
			break
		}
	}
	if len(tasks) == 0 {
		return nil, errors.New("no tasks supplied")
	}
	return tasks, nil
}

func store(path string, tasks []string) (line int, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND, 0644)
	prefix := "# INBOX\n\n"
	if errors.Is(err, os.ErrExist) {
		prefix = ""
		file, err = os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	}
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("inbox must be a regular file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		prefix = "\n"
	}
	line = strings.Count(string(data), "\n") + strings.Count(prefix, "\n") + 1
	payload := prefix + strings.Join(tasks, "\n") + "\n"
	count, err := io.WriteString(file, payload)
	if err != nil {
		return 0, fmt.Errorf("append inbox (some data may have been written): %w", err)
	}
	if count != len(payload) {
		return 0, io.ErrShortWrite
	}
	return line, nil
}

func edit(spec, path string, input io.Reader, output, diagnostics io.Writer) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("inbox does not exist; use inbox add to capture a task first")
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("inbox must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(spec) == "" {
		return errors.New("editor must not be empty")
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n\n") {
		padding := "\n"
		if data[len(data)-1] != '\n' {
			padding += "\n"
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(file, padding)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return err
		}
	}
	command := editorpkg.Command(spec, path, editorpkg.At{})
	if slices.Contains([]string{"vim", "nvim", "vi"}, filepath.Base(command.Args[0])) {
		command.Args = append(command.Args[:len(command.Args)-1], "+$", "+startinsert!", path)
	}
	command.Stdin, command.Stdout, command.Stderr = input, output, diagnostics
	return command.Run()
}

const usage = `inbox - Capture tasks into your inbox

Usage:
  inbox                         Open existing inbox in editor (terminal required)
  inbox show [--raw]            Color inbox on terminal; raw when redirected
  inbox edit                    Open existing inbox in editor
  inbox add [OPTIONS] [TITLE]    Capture a task, or read one task per stdin line
  inbox a [OPTIONS] [TITLE]      Alias for add
  inbox colors                  Print configured colors
  inbox help [COMMAND]          Show help (-h and --help also work)

Capture options (before TITLE; -- ends options):
  --stdin            Force stream input; cannot be combined with TITLE
  --keyword KEYWORD  Fallback keyword (default: first configured active keyword)
  --quiet            Suppress success output
  --json             Print inbox path, task records, and file/line selectors as JSON

Without TITLE, add uses stdin when redirected, otherwise interactive capture.
Each nonblank input line becomes a top-level task. Configured KEYWORD: prefixes
are preserved; plain lines use the fallback keyword. The entire input is
validated before writing. Empty input is an error; Ctrl+C cancels interactive
capture. TITLE conflicts with nonempty redirected stdin. --quiet and --json
are mutually exclusive. Capture creates missing storage only after validation.

Examples:
  inbox add "Write a blog post"
  inbox add --keyword TODO -- "Title starting with -"
  inbox add --json < tasks.txt
  inbox add --stdin < tasks.txt

Configuration: ~/.config/karya/config.toml
Environment: KARYA selects the inbox directory; EDITOR selects the editor.
Use todo ls inbox and other todo commands to manage captured tasks.
`

type model struct {
	input    textinput.Model
	quitting bool
	accepted bool
}

func initial() model {
	input := textinput.New()
	input.Placeholder = "Enter task description"
	input.Focus()
	input.Width = 50
	input.CharLimit = 0
	return model{input: input}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.accepted = true
			m.quitting = true
			return m, tea.Quit
		}
	}
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	return m, command
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("Enter task description:\n\n%s\n\nPress Ctrl+C to quit", m.input.View())
}
