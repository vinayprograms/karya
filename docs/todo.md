# todo - Task Management

Manage tasks across projects with support for tags, dates, assignees, and **task relationships**. Features powerful field-specific filtering, an interactive TUI, and **live file monitoring** that automatically updates the task list when files change.

## Task Format

Tasks in markdown files follow this format:

```markdown
TODO: Implement feature X #urgent @2025-01-15 >> john
TODO: [auth-01] Implement authentication #backend
TODO: [api-02] Build REST API ^auth-01 #backend
TODO: Review PR @s:2025-01-16 @d:2025-01-18 >> alice
DONE: Fix bug Y
TASK: Meeting notes #meeting @2025-01-20
```

### Field Definitions

- **`[id]`** - Optional unique identifier. Must appear right after the keyword. Used to create references between tasks.
- **`^id`** - Reference to another task by its ID. Creates a dependency relationship. Multiple references can be specified.
- **`#tag`** - Any text prefixed by "#" is treated as a tag.
  - Special tags can be listed in `config.toml` which will be highlighted with a different color. Special tags can also be followed by a ":<additional text>" to provide extra context. For example "#priority:high".
- **`@date`** - Any text prefixed by "@" is treated as a date. By default, this is the scheduled date.
  - To specify due dates, use the `@d:` prefix.
  - To specify scheduled dates explicitly, use the `@s:` prefix.
- **`>> assignee`** - Any text prefixed by ">>" is treated as an assignee. Multiple assignees must be separated by commas.

## Task Relationships

Tasks can reference other tasks using the `^id` syntax:

```markdown
TODO: [build-01] Build project
TODO: [test-01] Run unit tests ^build-01
TODO: [deploy-01] Deploy to production ^test-01 ^build-01
```

In this example:
- `test-01` depends on `build-01`
- `deploy-01` depends on both `test-01` and `build-01`

### Circular Dependency Detection

The tool automatically detects circular dependencies and highlights them in the TUI with a special indicator (⟲). For example:

```markdown
TODO: [a] Task A ^b
TODO: [b] Task B ^a  # Creates a cycle: a → b → a
```

Both tasks will be highlighted with the cycle indicator to help you identify and resolve the circular dependency.

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

- **Active Tasks**: TODO, TASK, NOTE, REMINDER, EMAIL, MESSAGE, FOLLOWUP, REVIEW, CHECKIN, CHECKOUT, RESEARCH, READING, WRITING, DRAFT, EDITING, FINALIZE, SUBMIT, PRESENTATION, WAITING, DEFERRED, DELEGATED
- **In-Progress Tasks**: DOING, INPROGRESS, STARTED, WORKING, WIP
- **Completed Tasks**: ARCHIVED, CANCELED, DELETED, DONE, COMPLETED, CLOSED

## Events

Configure time-sensitive items separately from work tasks:

```toml
[todo]
events = ["MEETING", "EVENT", "CALL"]
```

These are also the defaults. Set `events = []` to disable event classification, or add your own keywords for deadlines, launches, appointments and reminders. Events take precedence if a keyword also appears in a task-state category.

Events appear on their scheduled or due day in `agenda`, never as overdue work. Press Enter in agenda to open the source file and add indented notes below the event. Events do not cycle through TODO/DONE, support clock tracking, or generate logbook entries. They are excluded from the todo TUI, `todo list`, and MCP work-task lists and counts.

Agenda refreshes recurring events on startup and every 15 minutes while running. Past-day dates advance to today or the next recurrence, preserving the keyword, times, recurrence modifiers and notes. Today's event stays visible all day. For events, all recurrence modes advance from the stored date because there is no completion action. One-off events are left unchanged. Refresh pauses while the external editor owns the TUI and retries after one minute while the date picker is open.

The former routines view and `r` toggle have been removed. Migrate `routines` configuration to `events`; unlike routines, events include one-off and undated items. Undated events are available through MCP but have no agenda day.

Todo MCP provides `list_events` (optional project and filter), `schedule_event` (exact title or `raw_title`), and `refresh_events`. `get_keywords` includes Events. The old `show_routines` arguments and `routine_count` result have been removed. Existing task inspection tools can still inspect event notes; task-status and clock operations reject events.

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
