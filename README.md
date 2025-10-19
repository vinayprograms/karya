# Karya

A fast, concurrent task and note management toolkit written in Go. Karya helps you manage tasks, notes, and projects using a simple markdown-based system with powerful CLI tools.

The term "karya" means "work" or "creation" in several Indian languages, reflecting the tool's purpose of helping you organize and create your work efficiently (interesting tidbit - "karya" and "karma" share the same root word).

## Features

- **Fast & Concurrent**: Multi-threaded file processing handles hundreds of files efficiently
- **Task Management**: Track TODOs, tasks, and project status across multiple projects
- **Live File Monitoring**: Task list automatically updates when files change (no refresh needed)
- **Zettelkasten Support**: Built-in support for zettelkasten note-taking methodology
- **Git Integration**: Automatic git commits for notes and zettels using go-git
- **Beautiful TUI**: Interactive terminal UI powered by Bubble Tea and Glamour
- **Flexible Configuration**: TOML-based config with environment variable support

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

## Configuration

Karya uses a TOML configuration file located at `~/.config/karya/config.toml`.

### Example Configuration

```toml
# General settings (must be before [sections])
editor = "nvim"
show_completed = true
structured = true
verbose = false
color_mode = "dark" # "dark" or "light"

# Directory paths
[directories]
projects = "$HOME/Documents/projects"
zettelkasten = "$HOME/Documents/zet"
karya = "$HOME/Documents/karya"

# Customize task keywords
[keywords]
active = [
    "TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING",
    "CALL", "EMAIL", "MESSAGE", "FOLLOWUP", "REVIEW",
    "CHECKIN", "CHECKOUT", "RESEARCH", "READING", "WRITING",
    "DRAFT", "FINALIZE", "SUBMIT", "PRESENTATION"
]
inprogress = [
    "DOING", "INPROGRESS", "WIP", "WORKING", "STARTED",
    "WAITING", "DEFERRED", "DELEGATED"
]
completed = [
    "ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED"
]

# Customize TUI colors (optional)
[colors]
# Use color names, ANSI numbers (0-15), or hex colors (#RRGGBB)
tag = "bright-magenta"
tag-bg = "black"
# project = "green"
# active = "yellow"
# inprogress = "cyan"
# completed = "gray"
# date = "yellow"
# past-date = "red"
# today-bg = "yellow"
```

**Note:** Root-level settings must be defined before any `[section]` declarations in TOML format.

### Configuration Options

- **`editor`** - Text editor to use (default: vim). Supports vim, nvim, emacs, nano, code, etc.
- **`show_completed`** - Show completed tasks in TUI (default: false)
- **`structured`** - Use zettelkasten structure for notes (default: true)
  - `true`: Scans `project/notes/zettelID/README.md` files
  - `false`: Scans all `.md` files in project directory tree
- **`verbose`** - Show additional details like Zettel ID column (default: false)
- **`color_mode`** - Terminal color mode: "dark" or "light" (default: dark)

### Environment Variables

Environment variables take precedence over the config file.

- `PROJECTS` - Project root directory (required)
- `ZETTELKASTEN` - Zettelkasten directory
- `KARYA` - Karya inbox directory
- `EDITOR` - Text editor (default: vim)
- `SHOW_COMPLETED` - Show completed tasks (true/false)
- `STRUCTURED` - Use zettelkasten structure (true/false)
- `VERBOSE` - Show additional details like Zettel ID (true/false)
- `KARYA_COLOR_MODE` - Color mode: "light" or "dark"

**Note:** Command-line flags take precedence over environment variables and config file settings.

### Color Customization

You can customize TUI colors using three methods:

1. **Color names**: `"red"`, `"green"`, `"bright-magenta"`, etc.
2. **ANSI numbers**: `"0"` through `"15"` for 16-color palette
3. **Hex colors**: `"#E8F4F8"` for full RGB range

**For full configuration options**, see `config.toml.example` in the repository.

## Commands

Karya provides a suite of commands for different workflows. Below is a quick overview. For detailed documentation, see the [docs](./docs) folder.

### Core Commands

- **[`todo`](./docs/todo.md)** - Task management with interactive TUI, live file monitoring, and powerful filtering
- **[`zet`](./docs/zet.md)** - Zettelkasten notes with git integration and markdown rendering
- **[`note`](./docs/note.md)** - Project-specific notes (wrapper around `zet`)

### Quick Reference

```bash
# Task Management
todo                    # Interactive TUI with live monitoring
todo ls                 # List all tasks
todo projects           # Show project summary

# Zettelkasten
zet new "Note Title"    # Create new zettel
zet ls                  # List all zettels
zet ? "search"          # Search content

# Project Notes
note myproject new "Title"  # Create project note
note myproject ls           # List project notes
```

## Directory Structure

Karya expects the following directory structure:

```text
$PROJECTS/
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

$ZETTELKASTEN/
├── 20240115120000/
│   └── README.md
├── 20240116130000/
│   └── README.md
└── README.md (index)
```

## Performance

The task processing system uses an adaptive worker pool calculation to maximize the speed of collecting content from across the directory tree. The system starts with file discovery to find all matching files. It then performs dynamic worker allocation by calculating the optimal worker count based on available CPU cores, the total file count to process, and resource constraints (with a minimum of 1 and maximum equal to CPU count). The worker pool uses buffered channels for communication between the main process and workers. Results are aggregated from workers, while ensuring that all workers complete before proceeding. This provides optimal performance for both small projects (few files) and large workspaces (thousands of files).

## Libraries Used

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
5. **Update Documentation**: Document new features or changes in the appropriate docs file

## Documentation

- [Task Management (`todo`)](./docs/todo.md)
- [Zettelkasten (`zet`)](./docs/zet.md)
- [Project Notes (`note`)](./docs/note.md)

## License

[Add your license here]

## Acknowledgments

- [Charm](https://charm.sh/) for excellent TUI libraries

## Support

- **Issues**: [GitHub Issues](https://github.com/vinayprograms/karya/issues)
- **Discussions**: [GitHub Discussions](https://github.com/vinayprograms/karya/discussions)
- **Documentation**: See the [docs](./docs) folder

## Roadmap

- [ ] Add shell completion (bash, zsh, fish)
- [ ] Web UI for task visualization
- [ ] Mobile app integration
- [ ] Cloud sync support
- [ ] Task dependencies and workflows
- [ ] Time tracking integration
- [ ] Export to various formats (JSON, CSV, etc.)
