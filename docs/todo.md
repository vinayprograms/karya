# todo - Task Management

Manage tasks across projects with support for tags, dates, and assignees. Features powerful field-specific filtering, an interactive TUI, and **live file monitoring** that automatically updates the task list when files change.

## Task Format

Tasks in markdown files follow this format:

```markdown
TODO: Implement feature X #urgent @2025-01-15 >> john
TODO: Review PR @s:2025-01-16 @d:2025-01-18 >> alice
DONE: Fix bug Y
TASK: Meeting notes #meeting @2025-01-20
```

### Date Prefixes in UI

- `S:` - Scheduled date (when work should start)
- `D:` - Due date (when work must be completed)

## Tool usage

```bash
# Interactive TUI mode (default)
todo

# Show verbose output with Zettel ID column
todo -v
todo --verbose

# List all tasks in plain text
todo ls

# List tasks with verbose output
todo -v ls

# List tasks for a specific project
todo ls myproject

# Show interactive TUI for specific project with verbose output
todo -v myproject

# Show project summary table
todo projects

# Show project list (plain text)
todo pl
```

## Live File Monitoring

The interactive TUI automatically monitors your project directories for changes and updates the task list in real-time:

- **External edits**: Detects when files are modified by other tools/editors
- **New projects/files**: Automatically picks up newly created directories and new markdown files added to existing projects
- **Works with filters**: Updates happen even when a custom filter is active

The monitoring works in both structured and unstructured modes.

## Interactive Mode

### Navigation Keys

- `j/k` or `↑/↓` - Navigate tasks (vim-style)
- `g` / `G` - Jump to top / bottom
- `Ctrl+d` / `Ctrl+f` / `PgDn` - Page down (vim or emacs style)
- `Ctrl+u` / `Ctrl+b` / `PgUp` - Page up (vim or emacs style)

### Action Keys

- `/` - Start filtering
- `Enter` - Edit selected task / Exit filter mode
- `s` - Switch to structured mode (zettelkasten)
- `u` - Switch to unstructured mode (all .md files)
- `Esc` - Exit filter mode or clear filter
- `q` - Quit
- `Ctrl+c` - Quit

## Field-Specific Filtering

Press `/` in interactive mode to filter tasks by specific fields:

- `text` - Search across all fields
- `>> assignee` - Filter by assignee (e.g., `>> alice`)
- `#tag` - Filter by tag (e.g., `#urgent`)
- `@date` - Filter by scheduled date (e.g., `@2025-01-15`)
- `@s:date` - Explicitly filter by scheduled date (e.g., `@s:2025-01-15`)
- `@d:date` - Filter by due date (e.g., `@d:2025-01-20`)

### Filter Examples

```bash
# In interactive mode, press '/' then type:
>> john          # Show tasks assigned to john
#urgent          # Show tasks tagged as urgent
@2025-01-15      # Show tasks scheduled for Jan 15
@d:2025-01-20    # Show tasks due on Jan 20
```


### Date Color Coding

- Past dates: Red (inverted)
- Today: Yellow (bold)
- Future dates: Standard

## Supported Keywords

These are the default task keywords that are automatically recognized by the tool when `config.toml` doesn't specify custom keywords.

- **Active Tasks**: TODO, TASK, NOTE, REMINDER, EVENT, MEETING, CALL, EMAIL, MESSAGE, FOLLOWUP, REVIEW, CHECKIN, CHECKOUT, RESEARCH, READING, WRITING, DRAFT, EDITING, FINALIZE, SUBMIT, PRESENTATION, WAITING, DEFERRED, DELEGATED
- **In-Progress Tasks**: DOING, INPROGRESS, STARTED, WORKING, WIP
- **Completed Tasks**: ARCHIVED, CANCELED, DELETED, DONE, COMPLETED, CLOSED

## Configuration

### Environment Variables

```bash
EDITOR="nvim"              # Editor to use (supports vim, nvim, emacs, nano, code)
SHOW_COMPLETED=true        # Show completed tasks (default: false)
STRUCTURED=true            # Use zettelkasten structure (default: true)
VERBOSE=true               # Show additional details like Zettel ID (default: false)
                           # Note: -v/--verbose flag takes precedence
```

### Command-Line Options

- `-v, --verbose` - Show additional details like Zettel ID column

### Structured vs Unstructured Mode

- **Structured** (`STRUCTURED=true`): Scans `project/notes/zettelID/README.md` files
- **Unstructured** (`STRUCTURED=false`): Scans all `.md` files within the configured project root directory hierarchy.

You can toggle between modes in the interactive TUI using the `s` and `u` keys.

## Tips

- Use the verbose flag (`-v`) to see which zettel contains each task
- Combine filters to narrow down tasks (e.g., filter by assignee and tag)
- The live monitoring feature means you never need to restart the TUI when files change
- Use structured mode for zettelkasten-based workflows, unstructured for simpler setups
