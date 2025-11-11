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

**Main View:**
- `n` - Create new goal
- `e` or `Enter` - Edit selected goal
- `r` - Refresh goal list
- `1-5` - Switch between horizons (monthly, quarterly, yearly, short-term, long-term)
- `?` - Toggle help
- `q` - Quit
- `Esc` - Back to main view

**Goal Creation Form:**
- `TAB` - Switch between Title and Period fields
- `c` or `e` - Enable editing of Period field (when focused on it)
- `Enter` - Create goal and return to list
- `Shift+Enter` - Create goal and open in editor
- `Esc` - Cancel and return to list

## Creating Goals

### Interactive Goal Creation

When you press `n` to create a new goal, an interactive form appears with two fields:

1. **Title**: The name of your goal
2. **Period**: The time period for the goal (pre-filled with the next logical period)

#### Workflow:

1. **Enter Goal Title**: Start typing your goal title
2. **Switch to Period Field** (optional): Press `TAB` to move to the period field
3. **Edit Period** (optional): Press `c` or `e` to enable editing, then modify the period as needed
4. **Create Goal**:
   - Press `Enter` to create the goal and return to the list
   - Press `Shift+Enter` to create the goal and immediately open it in your configured editor

The period field is automatically populated based on your horizon and fiscal year configuration:
- **Monthly**: Next month (e.g., `2025-12`)
- **Quarterly**: Next quarter with fiscal year (e.g., `2026-Q3` if December 2025 with June fiscal year start)
- **Yearly**: Next fiscal year (e.g., `2026`)
- **Short-term**: Current year to 3 years out (e.g., `2025-2028`)
- **Long-term**: Current year to 10 years out (e.g., `2025-2035`)

**Note:** For quarterly goals, the year represents the fiscal year, not the calendar year. See the Fiscal Year and Quarter Configuration section for details.

You can edit the period to target a different time frame if needed.

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

## Fiscal Year and Quarter Configuration

The goal system supports configurable fiscal year boundaries to accommodate different organizational calendars.

### Configurable Fiscal Year Start

Users can configure the fiscal year start month in `config.toml`:

```toml
[goals]
year-start = "June"
```

Supported values are: "January", "February", "March", "April", "May", "June", 
"July", "August", "September", "October", "November", "December"

If not configured, the system defaults to "January" as the fiscal year start month.

### Quarterly Goal Creation

When creating quarterly goals, the system automatically assumes the next quarter as the default period. This is calculated based on the configured fiscal year start date.

**Important:** The year component in quarterly periods represents the **fiscal year**, not the calendar year. This follows common business practices where quarters are labeled with the fiscal year they belong to.

**Example:** If fiscal year starts in "June":
- Fiscal year 2026 runs from June 2025 to May 2026
- Quarters are: Q1 (Jun-Aug), Q2 (Sep-Nov), Q3 (Dec-Feb), Q4 (Mar-May)
- December 2025 is labeled as `2026-Q3` (fiscal year 2026, quarter 3)
- April 2025 is labeled as `2025-Q4` (fiscal year 2025, quarter 4)

**Concrete examples:**
- Current date: April 2025 → Current quarter: 2025-Q4 → Next goal: `2025-Q1` (June-Aug 2025)
- Current date: July 2025 → Current quarter: 2026-Q1 → Next goal: `2026-Q2` (Sep-Nov 2025)
- Current date: December 2025 → Current quarter: 2026-Q3 → Next goal: `2026-Q4` (Mar-May 2026)

### Yearly Goal Creation

Yearly goals respect the configured fiscal year start month as the year boundary.

**Example:** If fiscal year starts in "June":
- Fiscal year starts in June (not January)
- Current date is April 2025 → In fiscal year 2024 (started June 2024) → Next goal created for 2025
- Current date is July 2025 → In fiscal year 2025 (started June 2025) → Next goal created for 2026

This approach provides more natural goal creation for users with non-calendar fiscal years.
