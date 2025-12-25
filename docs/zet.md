# zet - Zettelkasten Notes

Manage your zettelkasten with git integration and markdown rendering. The `zet` command provides a complete toolkit for creating, editing, and organizing notes using the zettelkasten methodology.

## Zettelkasten Methodology

The zettelkasten method is a note-taking system that emphasizes:

1. **Atomic notes**: Each zettel contains one idea
2. **Linking**: Notes link to related notes
3. **Emergence**: Knowledge structures emerge from connections
4. **Permanent notes**: Notes are written for long-term value

### Best Practices

- **One idea per zettel**: Keep notes focused and atomic
- **Use descriptive titles**: Make titles searchable and meaningful
- **Link liberally**: Reference other zettels by ID
- **Write in your own words**: Don't just copy-paste; synthesize
- **Add context**: Include enough information to understand the note later
- **Use the TOC**: Maintain your table of contents as an entry point

### Linking Between Zettels

Reference other zettels in your notes:

```markdown
# Understanding Concurrency

See also: [[20240115120000]] for goroutine patterns
Related: [[20240116130000]] for channel usage
```
## Tool usage

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

### Table of Contents

```bash
# Edit the main index/table of contents
zet toc
```

The table of contents is stored in `$ZETTELKASTEN/README.md` and serves as the main entry point to your zettelkasten.

## Git Integration

The `zet` command automatically commits changes to git when you create or edit zettels.

## Directory Structure

Zettels are stored in the following structure:

```text
$ZETTELKASTEN/
├── 20240115120000/
│   └── README.md
├── 20240116130000/
│   └── README.md
├── 20240117140000/
│   └── README.md
└── README.md (table of contents)
```

Each zettel:
- Has a unique timestamp-based ID (YYYYMMDDHHmmss format)
- Lives in its own directory
- Contains a single `README.md` file with the note content

## Configuration

### Environment Variables

```bash
ZETTELKASTEN="$HOME/Documents/zet"  # Location of your zettelkasten
EDITOR="nvim"                        # Editor to use for editing zettels
```

### Config File

In `~/.config/karya/config.toml`:

```toml
editor = "nvim"

[directories]
zettelkasten = "$HOME/Documents/zet"
```

## Tips

- Use `zet last` to quickly continue working on your most recent note
- The `zet ?` search is great for finding notes when you remember content but not the ID
- Use `zet t?` when you remember part of the title
- Keep your TOC (`zet toc`) organized with categories or themes
- The `zet todo` command helps track action items across all your notes

## MCP Server Integration

The `zet mcp` command starts an MCP (Model Context Protocol) server that allows AI agents to interact with your zettelkasten. This enables AI assistants to create, read, search, and manage your notes programmatically.

### Starting the MCP Server

```bash
zet mcp
```

This starts a stdio-based MCP server that can be registered with AI agents like Claude Desktop, Cursor, or other MCP-compatible clients.

### Available Tools

The MCP server exposes the following tools:

| Tool | Description |
|------|-------------|
| `create_zettel` | Create a new zettel with an optional title |
| `list_zettels` | List all zettels (optionally with limit) |
| `get_zettel` | Get full content of a zettel by ID |
| `search_zettels` | Search across all zettel contents |
| `search_titles` | Search in zettel titles only |
| `count_zettels` | Get total count of zettels |
| `delete_zettel` | Delete a zettel by ID |
| `update_zettel` | Update a zettel's content |
| `get_last_zettel` | Get the most recently modified zettel |
| `find_todos` | Find all TODO items across zettels |

### Configuration Example (Claude Desktop)

Add to your Claude Desktop config (`~/.config/claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zet": {
      "command": "/path/to/zet",
      "args": ["mcp"]
    }
  }
}
```

### Example Usage

Once configured, you can ask your AI assistant to:
- "Create a new zettel about machine learning"
- "Search my zettels for notes about Go programming"
- "Show me my most recent zettel"
- "Find all TODO items in my zettelkasten"
