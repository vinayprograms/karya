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
