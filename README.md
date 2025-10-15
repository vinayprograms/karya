# Karya

A fast, concurrent task and note management toolkit written in Go. Karya helps you manage tasks, notes, and projects using a simple markdown-based system with powerful CLI tools.

## Features

- **Fast & Concurrent**: Multi-threaded file processing handles 2000+ files efficiently
- **Task Management**: Track TODOs, tasks, and project status across multiple projects
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
git clone https://github.com/yourusername/karya.git
cd karya
make build
```

Binaries will be available in the `bin/` directory.

### Install to System

```bash
# Copy binaries to your PATH
sudo cp bin/* /usr/local/bin/

# Or add bin directory to your PATH
export PATH="$PATH:$(pwd)/bin"
```

## Configuration

Create a configuration file at `~/.config/karya/config.toml`:

```toml
# Required: Root directory for projects
prjdir = "$HOME/Documents/projects"

# Optional: Zettelkasten directory (for zet command)
zetdir = "$HOME/Documents/zet"

# Optional: Editor (defaults to vim)
editor = "nvim"

# Optional: Karya directory for inbox (defaults to prjdir)
karya_dir = "$HOME/Documents/karya"
```

You can also use environment variables which take precedence over the config file:

```bash
export PRJDIR="$HOME/Documents/projects"
export ZETDIR="$HOME/Documents/zet"
export EDITOR="nvim"
export KARYA_DIR="$PRJDIR"
```

## Commands

### `todo` - Task Management

Manage tasks across projects with support for tags, dates, and assignees.

```bash
# Show pending tasks (default)
todo

# List all tasks (including completed)
todo ls

# List tasks for a specific project
todo myproject

# Show project summary
todo projects

# Show project list with counts
todo projlist
```

**Task Format in Markdown:**
```markdown
TODO: Implement feature X #urgent @2024-01-15 >> john
DONE: Fix bug Y
TASK: Review PR #123
```

**Supported Keywords:**
- Active: TODO, TASK, NOTE, REMINDER, EVENT, MEETING, CALL, EMAIL, MESSAGE, FOLLOWUP, REVIEW, CHECKIN, CHECKOUT, RESEARCH, READING, WRITING, DRAFT, EDITING, FINALIZE, SUBMIT, PRESENTATION, WAITING, DEFERRED, DELEGATED
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

```
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

```
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

Karya uses concurrent file processing with worker pools:

- **10 concurrent workers** by default
- Efficiently handles **2000+ markdown files**
- Scales across **22+ directory levels**
- Non-blocking I/O with goroutines and channels

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

The task processing system uses a worker pool pattern:

1. **File Discovery**: `filepath.Glob` finds all matching files
2. **Worker Pool**: 10 goroutines process files concurrently
3. **Channel Communication**: Files sent via channels to workers
4. **Result Aggregation**: Results collected from workers via channels
5. **Synchronization**: `sync.WaitGroup` ensures all workers complete

### Libraries Used

- **[BurntSushi/toml](https://github.com/BurntSushi/toml)**: TOML configuration parsing
- **[go-git/go-git](https://github.com/go-git/go-git)**: Git operations without CLI
- **[charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)**: Terminal UI framework
- **[charmbracelet/bubbles](https://github.com/charmbracelet/bubbles)**: TUI components
- **[charmbracelet/glamour](https://github.com/charmbracelet/glamour)**: Markdown rendering
- **[charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)**: Terminal styling

## Migration from Bash Scripts

If you're migrating from the original bash scripts in `.prior-art/`:

1. **Config Migration**: Convert `~/.gtdrc` to `~/.config/karya/config.toml`
2. **Same Directory Structure**: No changes needed to your project directories
3. **Compatible Task Format**: All task keywords and formats are preserved
4. **Faster Execution**: Go implementation is significantly faster
5. **No External Dependencies**: No need for `fzf`, `bat`, `grep`, etc.

### Example Migration

**Old `~/.gtdrc`:**
```bash
export PRJDIR="$HOME/Documents/projects"
export ZETDIR="$HOME/Documents/zet"
export EDITOR="vim"
```

**New `~/.config/karya/config.toml`:**
```toml
prjdir = "$HOME/Documents/projects"
zetdir = "$HOME/Documents/zet"
editor = "vim"
```

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

- Original bash scripts that inspired this project
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

**Built with ❤️ using Go and Charm libraries**
