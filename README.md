# Karya

A fast, concurrent task and note management toolkit written in Go. Karya helps you manage tasks, notes, and projects using a simple markdown-based system with powerful CLI tools.

## Features

- **Fast & Concurrent**: Multi-threaded file processing handles 2000+ files efficiently
- **Task Management**: Track TODOs, tasks, and project status across multiple projects
- **Live File Monitoring**: Task list automatically updates when files change (no refresh needed)
- **Zettelkasten Support**: Built-in support for zettelkasten note-taking methodology
- **Git Integration**: Automatic git commits for notes and zettels using go-git
- **Beautiful TUI**: Interactive terminal UI powered by Bubble Tea and Glamour
- **Flexible Configuration**: TOML-based config with environment variable support
- **Multiple Tools**: Suite of commands for different workflows

## Installation

### Prerequisites

- Go 1.24 or later
- Git (optional, for version control features)

### Build from Source

```bash
git clone https://github.com/vinayprograms/karya.git
cd karya
make build
```

Binaries will be available in the `bin/` directory.

### Install with `go install`

You can install individual commands directly using `go install`:

```bash
# Install specific command
go install github.com/vinayprograms/karya/cmd/todo@latest
go install github.com/vinayprograms/karya/cmd/zet@latest
go install github.com/vinayprograms/karya/cmd/note@latest

# Or install all commands at once
go install github.com/vinayprograms/karya/cmd/...@latest
```

Binaries will be installed to `$GOPATH/bin` (typically `$HOME/go/bin`).

**For Private Repository:** Configure Git and Go for private repository access (one-time setup):

```bash
# Tell Git to use SSH for GitHub (uses your existing ~/.ssh/config)
git config --global url."git@github.com:".insteadOf "https://github.com/"

# Tell Go to skip checksum verification for private modules
export GOPRIVATE=github.com/vinayprograms/karya
# Add this to your ~/.bashrc or ~/.zshrc to make it permanent:
echo 'export GOPRIVATE=github.com/vinayprograms/karya' >> ~/.bashrc  # or ~/.zshrc
```

This is required because `go install` uses HTTPS by default and tries to verify modules through the public checksum database. After running these commands, all `go` commands will automatically use SSH (and your existing SSH keys) for GitHub authentication and skip public verification.

Once the repository is made public, this configuration is no longer needed (but won't cause any issues if left in place).

### Install to System

```bash
# Copy binaries to your PATH
sudo cp bin/* /usr/local/bin/

# Or add bin directory to your PATH
export PATH="$PATH:$(pwd)/bin"
```

## Configuration

Karya uses a TOML configuration file located at `~/.config/karya/config.toml`.

**Quick Start:**

```toml
# Minimum required configuration
prjdir = "$HOME/Documents/projects"

# Optional settings
zetdir = "$HOME/Documents/zet"
editor = "nvim"
show_completed = false
structured = true
```

**Environment Variables:**

Environment variables take precedence over the config file.

- `PRJDIR` - Project root directory (required)
- `ZETDIR` - Zettelkasten directory
- `EDITOR` - Text editor (default: vim)
- `SHOW_COMPLETED` - Show completed tasks (true/false)
- `STRUCTURED` - Use zettelkasten structure (true/false)

**For full configuration options**, including custom keywords and advanced settings, see `config.toml.example` in the repository.

## Commands

### `todo` - Task Management

Manage tasks across projects with support for tags, dates, and assignees. Features powerful field-specific filtering, an interactive TUI, and **live file monitoring** that automatically updates the task list when files change.

```bash
# Interactive TUI mode (default)
todo

# List all tasks in plain text
todo ls

# List tasks for a specific project
todo ls myproject

# Show interactive TUI for specific project
todo myproject

# Show project summary table
todo projects

# Show project list (plain text)
todo pl
```

**Live File Monitoring:**

The interactive TUI automatically monitors your project directories for changes and updates the task list in real-time:

- **External edits**: Detects when files are modified by other tools/editors
- **New projects**: Automatically picks up newly created directories and files
- **New files**: Detects new markdown files added to existing projects
- **Works with filters**: Updates happen even when a custom filter is active

The monitoring works in both structured and unstructured modes

**Interactive Mode Keys:**

**Navigation:**

- `j/k` or `↑/↓` - Navigate tasks (vim-style)
- `g` / `G` - Jump to top / bottom
- `Ctrl+d` / `Ctrl+f` / `PgDn` - Page down (vim or emacs style)
- `Ctrl+u` / `Ctrl+b` / `PgUp` - Page up (vim or emacs style)

**Actions:**

- `/` - Start filtering
- `Enter` - Edit selected task / Exit filter mode
- `s` - Switch to structured mode (zettelkasten)
- `u` - Switch to unstructured mode (all .md files)
- `Esc` - Exit filter mode or clear filter
- `q` - Quit
- `Ctrl+c` - Quit

**Field-Specific Filtering:**

Press `/` in interactive mode to filter tasks by specific fields:

- `text` - Search across all fields
- `>> assignee` - Filter by assignee (e.g., `>> alice`)
- `#tag` - Filter by tag (e.g., `#urgent`)
- `@date` - Filter by scheduled date (e.g., `@2025-01-15`)
- `@s:date` - Explicitly filter by scheduled date (e.g., `@s:2025-01-15`)
- `@d:date` - Filter by due date (e.g., `@d:2025-01-20`)

**Examples:**

```bash
# In interactive mode, press '/' then type:
>> john          # Show tasks assigned to john
#urgent          # Show tasks tagged as urgent
@2025-01-15      # Show tasks scheduled for Jan 15
@d:2025-01-20    # Show tasks due on Jan 20
```

**Task Format in Markdown:**

```markdown
TODO: Implement feature X #urgent @2025-01-15 >> john
TODO: Review PR @s:2025-01-16 @d:2025-01-18 >> alice
DONE: Fix bug Y
TASK: Meeting notes #meeting @2025-01-20
```

**Date Prefixes in UI:**

- `S:` - Scheduled date (when work should start)
- `D:` - Due date (when work must be completed)

**Date Color Coding:**

- Past dates: Red (inverted)
- Today: Yellow (bold)
- Future dates: Standard

**Environment Variables:**

```bash
EDITOR="nvim"              # Editor to use (supports vim, nvim, emacs, nano, code)
SHOW_COMPLETED=true        # Show completed tasks (default: false)
STRUCTURED=true            # Use zettelkasten structure (default: true)
```

**Structured vs Unstructured Mode:**

- **Structured** (`STRUCTURED=true`): Scans `project/notes/zettelID/README.md` files
- **Unstructured** (`STRUCTURED=false`): Scans all `.md` files in project directory tree

**Supported Keywords:**

- Active: TODO, TASK, NOTE, REMINDER, EVENT, MEETING, CALL, EMAIL, MESSAGE, FOLLOWUP, REVIEW, CHECKIN, CHECKOUT, RESEARCH, READING, WRITING, DRAFT, EDITING, FINALIZE, SUBMIT, PRESENTATION, WAITING, DEFERRED, DELEGATED
- In-Progress: DOING, INPROGRESS, STARTED, WORKING, WIP
- Completed: ARCHIVED, CANCELED, DELETED, DONE, COMPLETED, CLOSED

### `zet` - Zettelkasten Notes

Manage your zettelkasten with git integration and markdown rendering.

```bash
# Interactive mode (TUI)
zet

# Create new zettel
zet new "My Note Title"
zet n "Quick note"

# Edit existing zettel
zet edit 20240115120000
zet e 20240115120000

# List all zettels
zet ls

# Show zettel content (rendered markdown)
zet show 20240115120000

# Edit last zettel
zet last

# Edit table of contents
zet toc

# Count zettels
zet count

# Search content
zet ? "search term"

# Search titles
zet t? "title search"

# Find TODOs
zet todo
zet d
```

### `note` - Project Notes

Wrapper around `zet` for project-specific notes.

```bash
# Create/edit notes for a project
note myproject new "Meeting notes"
note myproject edit 20240115120000
note myproject ls

# Count notes in project
note myproject count
```

### `inbox` - Quick Capture

Quickly capture ideas and tasks to your inbox.

```bash
# Open inbox in editor
inbox

# Add item to inbox
inbox "Remember to call John"
inbox "Buy groceries"
```

### `goal` - Project Goals

Manage project goals and objectives.

```bash
# Edit goals for a project
goal myproject
```

### `thot` - Quick Thoughts

Capture quick thoughts for a project.

```bash
# Open thoughts file in editor
thot myproject

# Add a quick thought
thot myproject "Idea for new feature"
```

### `prj` - Project Overview

View project summary with tasks and notes.

```bash
# Show all projects
prj list
prj ls

# Show specific project
prj myproject

# Watch mode (refreshes every 5 seconds)
prj show
```

### `tickle` - Deferred Tasks

Mark tasks to be reminded about later.

```bash
# Defer task until next Monday
tickle monday "Follow up with client"

# Defer with specific date
tickle 2024-01-20 "Review quarterly report"
```

### `think` - Weekly Review

Shortcut for weekly review tasks (defers to Friday).

```bash
think "Review this week's progress"
```

### `waitfor` - Waiting Tasks

Track tasks you're waiting on others to complete.

```bash
# Show all waiting tasks
waitfor

# Show tasks waiting on specific person
waitfor john

# Mark task as waiting
waitfor john 123
```

### `rnr` - Reading List

Manage your reading list with automatic title extraction.

```bash
# Open reading list
rnr

# Add URL to reading list (auto-extracts title)
rnr https://example.com/article
```

### `holiday` - Holiday Information

Fetch holiday information from holidata.net.

```bash
# Current year, system locale
holiday

# Specific year
holiday 2024

# Specific locale, region, and year
holiday en-US "" 2024
```

## Project Structure

```text
karya/
├── cmd/                    # Command implementations
│   ├── todo/              # Task management
│   ├── zet/               # Zettelkasten
│   ├── note/              # Project notes
│   ├── inbox/             # Quick capture
│   ├── goal/              # Project goals
│   ├── thot/              # Quick thoughts
│   ├── prj/               # Project overview
│   ├── tickle/            # Deferred tasks
│   ├── think/             # Weekly review
│   ├── waitfor/           # Waiting tasks
│   ├── rnr/               # Reading list
│   └── holiday/           # Holiday info
├── internal/              # Internal packages
│   ├── config/            # Configuration management
│   ├── task/              # Task processing (concurrent)
│   └── zet/               # Zettelkasten logic
├── .prior-art/            # Original bash scripts
├── bin/                   # Compiled binaries
├── config.toml.example    # Example configuration
└── Makefile              # Build automation
```

## Directory Structure

Karya expects the following directory structure:

```text
$PRJDIR/
├── project1/
│   ├── goals.md
│   ├── thots.md
│   └── notes/
│       ├── 20240115120000/
│       │   └── README.md
│       └── 20240116130000/
│           └── README.md
├── project2/
│   └── notes/
│       └── ...
└── ...

$ZETDIR/
├── 20240115120000/
│   └── README.md
├── 20240116130000/
│   └── README.md
└── README.md (index)
```

## Performance

Karya uses concurrent file processing with adaptive worker pools:

- **Dynamic worker calculation** - Automatically determines optimal worker count based on:
  - Number of CPU cores available
  - Total number of files to process
  - System resources
- Efficiently handles **2000+ markdown files**
- Scales across **22+ directory levels**
- Non-blocking I/O with goroutines and channels
- Adaptive performance for both small and large workloads

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./cmd/todo
go test ./internal/task

# Run with coverage
go test -cover ./...
```

### Building

```bash
# Build all commands
make build

# Build specific command
go build -o bin/todo ./cmd/todo

# Clean build artifacts
make clean
```

### Code Style

This project follows Go best practices:

- Test-Driven Development (TDD)
- Comprehensive test coverage
- Detailed inline documentation
- Idiomatic Go code
- No external CLI dependencies (except EDITOR)

## Architecture

### Concurrency Model

The task processing system uses an adaptive worker pool calculation to maximize the speed of collecting tasks from across the directory tree. The system starts with file discovery to find all matching files. It then performs dynamic worker allocation by calculating the optimal worker count based on available CPU cores, the total file count to process, and resource constraints (with a minimum of 1 and maximum equal to CPU count). The worker pool consists of goroutines that process files concurrently, using buffered channels for communication between the main process and workers. Results are aggregated from workers via channels, while ensuring that all workers complete before proceeding. This adaptive approach provides optimal performance for both small projects (few files) and large workspaces (thousands of files).

### Libraries Used

- **[BurntSushi/toml](https://github.com/BurntSushi/toml)**: TOML configuration parsing
- **[go-git/go-git](https://github.com/go-git/go-git)**: Git operations without CLI
- **[fsnotify/fsnotify](https://github.com/fsnotify/fsnotify)**: Cross-platform file system notifications
- **[charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)**: Terminal UI framework
- **[charmbracelet/bubbles](https://github.com/charmbracelet/bubbles)**: TUI components
- **[charmbracelet/glamour](https://github.com/charmbracelet/glamour)**: Markdown rendering
- **[charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)**: Terminal styling

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Write Tests First**: Follow TDD principles
2. **Document Your Code**: Add inline comments for complex logic
3. **Run Tests**: Ensure all tests pass before submitting PR
4. **Follow Go Style**: Use `gofmt` and follow Go conventions
5. **Update README**: Document new features or changes

### Development Workflow

```bash
# 1. Create feature branch
git checkout -b feature/my-feature

# 2. Write tests
# Edit cmd/mycommand/mycommand_test.go

# 3. Implement feature
# Edit cmd/mycommand/main.go

# 4. Run tests
go test ./cmd/mycommand

# 5. Build and test manually
make build
./bin/mycommand

# 6. Commit with descriptive message
git commit -m "Add feature X with Y functionality"

# 7. Push and create PR
git push origin feature/my-feature
```

## License

[Add your license here]

## Acknowledgments

- [Charm](https://charm.sh/) for excellent TUI libraries
- The Go community for best practices and patterns

## Support

- **Issues**: [GitHub Issues](https://github.com/yourusername/karya/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/karya/discussions)
- **Documentation**: This README and inline code comments

## Roadmap

- [ ] Add shell completion (bash, zsh, fish)
- [ ] Web UI for task visualization
- [ ] Mobile app integration
- [ ] Cloud sync support
- [ ] Plugin system
- [ ] Task dependencies and workflows
- [ ] Time tracking integration
- [ ] Export to various formats (JSON, CSV, etc.)

---
