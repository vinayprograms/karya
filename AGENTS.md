# AGENTS.md

Karya is a suite of Go CLI/TUI tools for markdown-based task and note management (`todo`, `note`, `zet`, `goal`, `agenda`, `inbox`). Each binary also doubles as an MCP server.

## Commands

```bash
make build            # builds all cmd/* into bin/
go test ./...         # full test suite (fast, no external deps)
go test ./internal/task/   # focused package tests
make install          # go install all commands
make install-plugins  # symlink vim plugin into ~/.vim/pack
```

No lint config in repo; standard `go vet`/`gofmt` apply.

## Architecture

- `cmd/<name>/main.go` — one main per tool. These are large (todo's main.go is ~2800 lines) and contain the entire Bubble Tea TUI: models, key handling, rendering, fsnotify live-reload. Shared logic lives in `internal/`.
- `internal/task/` — the core: task parsing, hierarchy (parent/child via indentation), schedule/agenda views, clocking, date/status pickers, JIRA sync, and the todo MCP server (`mcp.go`).
- `internal/note/`, `internal/zet/`, `internal/goal/` — per-tool domain logic + their MCP servers.
- `internal/parallel/` — generic worker-pool (`parallel.Process[T,R]`) used for concurrent markdown file scanning; results are unordered.
- `internal/config/` — TOML config from `~/.config/karya/config.toml`, env vars (`PROJECTS`, `ZETTELKASTEN`, `KARYA`, `EDITOR`, ...) override the file; defaults applied last. Task keywords (active/inprogress/completed/someday/routines) are config-driven — never hardcode keyword lists.
- `internal/git/` — go-git auto-commit for notes/zettels; SSH key resolution via `~/.ssh/config`.
- `internal/jira/` — Atlassian MCP client (OAuth) for pulling assigned tickets.
- Each tool's MCP mode is entered via the `mcp` subcommand (e.g., `todo mcp`), wired in each `cmd/*/main.go`.

## Task format (the domain model)

Tasks are lines in markdown files: `KEYWORD: [id] Title ^ref #tag @s:date @d:date >> assignee`.
- `[id]` optional identifier, `^id` dependency references (circular deps detected and flagged via `Task.InCycle`).
- `@date`/`@s:` scheduled, `@d:` due date; `>>` assignees (comma-separated).
- Hierarchy comes from indentation; parents cannot be completed with pending children (`task.ErrPendingChildren`).
- `Task.RawTitle` preserves the unparsed line remainder for exact in-file line rewriting — status updates edit source files in place, so keep `RawTitle`/`LineNum`/`FilePath` intact when modifying tasks. Task keys include `LineNum` to disambiguate duplicate titles.

## Conventions & gotchas

- Tests use stdlib `testing` only, table-driven; tests exist next to code and also in `cmd/*` (e.g., `cmd/todo/todo_test.go`).
- Config structs use TOML tags; colors accept ANSI names, numbers, hex, or gogh theme names resolved in `internal/config`.
- MCP tool args/results are structs with both `json` and `jsonschema` tags (see `internal/note/mcp.go` for the pattern).
- Rendering uses lipgloss; `internal/task/markdown.go` does regex-based inline markdown styling (not glamour) for task titles.
- `parallel.Process` returns only non-nil results in nondeterministic order — sort afterward if order matters.
- `docs/*.md` document user-facing behavior per tool; update them when changing CLI behavior.
