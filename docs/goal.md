# Goal Command

The `goal` command is used for managing goals organized by horizons. It supports five distinct time horizons:

- **Monthly** - goals for the current month
- **Quarterly** - goals for the current quarter  
- **Yearly** - goals for the current year
- **Short-term** (3 years) - goals with a 3-year timeframe
- **Long-term** (10 years) - goals with a 10-year timeframe

## Features

- Interactive TUI for goal management
- Multi-form workflow for different time horizons
- Markdown-based goal storage with structured format
- Goal details and progress tracking
- Directory structure organized by horizon and period

## Directory Structure

Goals are stored in `$KARYA_DIR/.goals/` with the following structure:

```
$KARYA_DIR/.goals/
├── monthly/
│   └── 2025-11/
│       └── my_goal.md
├── quarterly/
│   └── 2025-Q1/
│       └── my_goal.md
├── yearly/
│   └── 2025/
│       └── my_goal.md
├── short-term/
│   └── 2025-2027/
│       └── my_goal.md
├── long-term/
│   └── 2025-2035/
│       └── my_goal.md
```

## Usage

### Interactive TUI

Run `goal` to start the interactive goal management TUI:

```bash
goal
```

### Key Commands in TUI

- `n` - Create new goal
- `e` - Edit selected goal
- `r` - Refresh goal list
- `?` - Toggle help
- `q` - Quit
- `esc` - Back to main view

## Goal File Format

Each goal is stored as a Markdown file with the following structure:

```markdown
# Goal Title

## Goal Details

## Progress

## Notes
```

## Development

The goal command is built using Go with:

- Terminal UI built with [bubbletea](https://github.com/charmbracelet/bubbletea)
- Markdown parsing using standard library
- File system operations with `os` and `path/filepath`

## Implementation Details

The command implements a multi-form TUI workflow:

1. **Horizon Tabs**: Different time horizons are represented as tabs at the top
2. **Flat List**: For long-term horizons (10 years), goals are displayed in a flat list
3. **Tree View**: For shorter horizons (monthly, quarterly, yearly), a tree view shows the relationship from long-term to specific period
4. **Month Display**: Month numbers are replaced with actual month names in tree view

The implementation ensures that title is displayed at the top with static positioning, and a tab selection system is provided for navigation between different horizons.
