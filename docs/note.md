# note - Project Notes

Wrapper around `zet` for project-specific notes. The `note` command provides the same zettelkasten functionality as `zet`, but scoped to individual projects.

## Usage

```bash
# Create new note for a project
note myproject new
note myproject n

# Edit existing note
note myproject edit 20240115120000
note myproject e 20240115120000

# List all notes in project
note myproject ls

# Show note content (rendered markdown)
note myproject show 20240115120000

# Edit last note
note myproject last

# Count notes in project
note myproject count

# Search content in project notes
note myproject ? "search term"

# Search titles in project notes
note myproject t? "title search"

# Find TODOs in project notes
note myproject todo
note myproject d
```

## Directory Structure

Project notes are stored within the project directory:

```text
$PROJECTS/
├── myproject/
│   ├── goals.md
│   ├── thots.md
│   └── notes/
│       ├── 20240115120000/
│       │   └── README.md
│       ├── 20240116130000/
│       │   └── README.md
│       └── 20240117140000/
│           └── README.md
└── anotherproject/
    └── notes/
        └── ...
```

Each project note:
- Has a unique timestamp-based ID (YYYYMMDDhhmmss format)
- Lives in its own directory under `project/notes/`
- Contains a single `README.md` file with the note's content

## Relationship to `zet` command

The `note` command is essentially `zet` scoped to a project directory:

- **`zet`**: Personal zettelkasten in `$ZETTELKASTEN/`
- **`note`**: Project-specific notes in `$PROJECTS/projectname/notes/`

Both use the same zettelkasten structure and methodology, but serve different purposes:

- Use `zet` for personal knowledge management and permanent notes
- Use `note` for project-specific, context-bound documentation

## Git Integration

Like `zet`, the `note` command automatically commits changes to git when you create or edit notes. This provides version control for all project notes.

## Configuration

### Environment Variables

```bash
PROJECTS="$HOME/Documents/projects"  # Location of your projects
EDITOR="nvim"                        # Editor to use for editing notes
```

### Config File

In `~/.config/karya/config.toml`:

```toml
editor = "nvim"

[directories]
projects = "$HOME/Documents/projects"
```

## Use Cases

Project notes are ideal for:

- **Meeting notes**: Document discussions and decisions
- **Research findings**: Track investigation results
- **Design documents**: Capture architecture and design decisions
- **Bug investigations**: Document debugging process and findings
- **Sprint planning**: Record sprint goals and tasks
- **Retrospectives**: Capture lessons learned
- **Technical specs**: Write detailed specifications

## Tips

- Use `note myproject last` to quickly continue working on your most recent project note
- Project notes can reference zettels from your main zettelkasten using IDs
- The `note myproject todo` command helps track action items specific to that project
- Keep project notes focused on project-specific information; use `zet` for general knowledge
- Use the same linking conventions as your main zettelkasten for consistency
