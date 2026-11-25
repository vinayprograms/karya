# Inbox capture

Inbox captures tasks; use `todo ls inbox`, `todo transition`, and other todo operations to manage them. No inbox MCP server is provided.

## Commands

```sh
inbox add "Write a proposal"
inbox a "Quick capture"
inbox add
inbox add < tasks.txt
inbox add --stdin --json < tasks.txt
inbox add --keyword NEXT --quiet "Prepare meeting"
inbox add -- "--literal title"
inbox show
inbox edit
inbox
inbox help
inbox add --help
inbox colors
```

`add` without a title uses single-task Bubble Tea input when stdin is a terminal, otherwise reads stdin. `--stdin` forces stream reading, including from a terminal until EOF. Output redirection alone does not select stream input. Interactive rendering goes to stderr; results go to stdout. Ctrl+C or Escape cancels without writes; `q` is ordinary text. Empty submission is an error.

Put flags before the title; `--` ends option parsing. Tokens after the first title argument are literal title text, not flags. A title containing line breaks is rejected; use stream input for batches. Unknown commands/flags and extra editor arguments fail without opening a UI.

## Batch semantics

Each nonblank line becomes one independent top-level task. Surrounding whitespace and indentation are removed; blank lines are skipped. Existing configured `KEYWORD:` prefixes and metadata are preserved. Plain lines use the first configured active keyword, or a validated `--keyword` fallback. The fallback never replaces an existing recognized status. Unknown prefixes are plain title text. Markdown bullets, hierarchy, and detail blocks are not imported specially.

The entire stream is read and validated before storage changes. Empty input, empty recognized task titles, invalid text, or input read failures reject the whole batch. Input line numbers identify invalid records. Disk failures can still leave partial writes; no transactional rollback or cross-process locking is promised.

Title arguments conflict with any actual bytes on redirected stdin, including whitespace. Empty stdin such as `/dev/null` is allowed; an open pipe may wait for input or EOF. Explicit `--stdin` with a title is always rejected immediately.

Valid capture automatically creates missing parent directories and initializes a missing inbox with `# INBOX`. Existing content is preserved and a missing final newline is repaired before append. Help, invalid input, cancellation, and editor commands do not initialize storage.

## Output

Default output confirms the number of captured tasks and destination. `--quiet` suppresses success output. `--json` emits an object with `path` (absolute inbox filename) and `tasks` (array of stored task-record strings), plus `selectors` in matching order. Each selector contains `path`, one-based `line`, and `expected` (the exact task record). Positions describe the capture-time file, not permanent IDs: subsequent edits or concurrent writers can invalidate them. Consumers must verify expected content before mutation; todo selector-based mutation remains separate work. These flags are mutually exclusive. Diagnostics go to stderr; failures exit nonzero. An output failure after saving is reported as such, so callers must not blindly retry.

`inbox show` uses configured colors when stdout is a terminal: headings, configured task keywords, task descriptions, tags/special tags, dates, and assignees are highlighted without removing Markdown syntax. Redirected or piped output remains byte-for-byte raw, independently of stdin. `inbox show --raw` disables styling even on a terminal. It never creates storage or launches an editor. Missing/unreadable files and output failures return an error; operands are rejected.

## Editor and configuration

Bare `inbox` and `inbox edit` require terminal stdin and an existing inbox. Missing storage returns guidance to capture a task first. Vim/Neovim/vi open at EOF in insert mode; a trailing blank line is prepared for editing. Other editors use the shared editor launcher. Editor specifications retain the shared launcher's whitespace-split argument behavior; shell quoting and paths containing spaces are not supported by that launcher.

Configuration is read from `~/.config/karya/config.toml`. `KARYA` overrides the inbox directory, `EDITOR` overrides the editor, and task keywords come from `[todo]`. The filename is `inbox.md`. Shared configuration supplies project/home fallbacks when no inbox directory is configured. Help works without valid configuration or storage. `KARYA_DIR` is not supported.

## Verification

```sh
go test ./cmd/inbox
go test ./...
go vet ./...
```
