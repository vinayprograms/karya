# Karya Refactor Plan

Findings from a full-codebase review, organized around three questions:
duplicated-but-diverging logic, over-complex functions, and alignment with
the "one word per exported name; call sites read like prose" philosophy
(as practiced in `agentkit`: `llm.New`, `memory.Store`, `store.Recall(...)`).

---

## 1. Duplicated logic that has already diverged

These are ranked by divergence risk — the copies are not just repeated, they
already behave differently.

### 1.1 Editor launching (~10 copies, actively diverging) — HIGH
- `cmd/todo/main.go:1702` (`openEditorCmd`), `cmd/note/main.go:1179-1310` (4 copies),
  `cmd/zet/main.go:731-832` (3 copies), `cmd/goal/main.go:870-925` (2 copies),
  `cmd/agenda/main.go:2007`.
- Divergence: todo reads `os.Getenv("EDITOR")` directly, bypassing config's
  editor + vim fallback that every other command uses; todo alone supports
  line-number navigation (`+N`, `code -g file:line`); zet's search-term switch
  handles nano/code/subl, note's copy dropped them; zet's post-edit hook calls
  `UpdatePinboard`, note's near-identical copy doesn't.
- Fix: new `internal/editor` package. One entry point:
  `editor.Open(cfg, file, editor.At{Line: n, Search: term})`.

### 1.2 ColorScheme struct + InitializeColors (5 independent copies) — HIGH
- todo:33-75, note:32-64, zet:31-63, goal:32-43, agenda's own `colors` struct.
- Divergence: entirely different field sets; **note's version has a live bug** —
  it initializes `selectorStyle`/`navStyle`/`errorStyle` from the zero-value
  global, so those styles are always empty.
- Fix: build all styles once in `internal/colors` from `config.ColorScheme`;
  cmds consume `colors.Scheme(cfg)`.

### 1.3 fsnotify watcher setup + waitForFileChange (4 copies each) — HIGH
- `waitForFileChange`: todo:519, note:707, zet:222, agenda:186. Identical 100ms
  debounce, but todo logs watcher errors while agenda silently drops them.
- `setupWatcher`/`updateWatcher`: todo:1852-1992, note:2055-2120 (two variants
  in one file), zet:1430, agenda:2180. Diverge in recursion depth (full walk vs
  1-level vs explicit dirs), hidden-dir skipping (note skips dot-dirs, agenda
  doesn't), and error handling.
- Fix: new `internal/watch` package: `watch.Dirs(paths) (*Watcher, error)` and
  a shared bubbletea `watch.Wait(w) tea.Cmd`.

### 1.4 Status-dispatch keyword loops (7+ sites, 3 behaviors) — HIGH
- `IsActive/IsInProgress/IsCompleted/IsSomeday/IsRoutine`
  (internal/task/task.go:44-128) are five copies of the same nil-check +
  keyword loop; `IsCompletedKeyword` (task.go:1132) is a sixth.
- The Active→InProgress→Someday→else-Completed dispatch is re-implemented at
  todo:134, todo:259, todo:1497, agenda:1591, agenda:2102, task/mcp.go:408 and
  :718 — with different check ordering and different "else" semantics.
- Fix: a single `func (t *Task) Status(cfg) Status` enum (partially exists as
  `Priority`, task.go:147) + `slices.Contains`. All render/dispatch sites
  switch on the enum.

### 1.5 Pending-children completion guard (agenda re-implements) — HIGH
- `task.go:1035` (canonical), todo wraps `task.HasActiveChildren` correctly,
  but **agenda:956 re-implements the child loop inline** while agenda:343 uses
  the helper. Inline copy will not track semantic changes.
- Fix: delete the inline loop; always call the `internal/task` helper.

### 1.6 Task-line location in files (3 implementations) — MEDIUM
- todo:1786 (`findTaskLine`), task.go:1071 (inside `UpdateTaskStatus`),
  clock.go:115-128 — all locate a task's source line via prefix-strip +
  keyword/title/ID match, with different match ordering.
- Fix: one `task.Locate(file, t) (line int, err error)` used by all three.

### 1.7 zet logic duplicated inside package task — MEDIUM
- `task.IsValidZettelID` (task.go:994) vs `zet.IsValidZettelID` (zet.go:316);
  `task.GetZettelTitle` (task.go:1007) vs `zet.GetZettelTitle` (zet.go:179);
  `task.SearchInFile` (task.go:904) vs `zet.SearchInFile` (zet.go:344).
- Fix: keep one copy in `zet`; `task` imports it (or extract a tiny shared
  package if the import direction is wrong).
  > [VINAY] Can these be pulled into a small, logically cohesive, small package? Making `task` package depend on `zet` package may introduce stong coupling that we don't need between tools.

### 1.8 git helpers duplicated inside package zet — MEDIUM
- `zet.GitInit`/`zet.GitInitAndCommit` (git.go:248,254) duplicate
  `git.Init`/`git.InitAndCommit`.
- Fix: delete the zet copies; call `internal/git`.

### 1.9 Duplicate row renderers within single files — MEDIUM
- todo: `renderWithSelection` (main.go:99-215) vs `Title()` (main.go:224-334)
  are ~110-line near-identical copies differing only in the selection glyph.
- note: `projectItem` (main.go:118) vs `matchedNoteItem` (main.go:173) repeat
  namespace-dimming/padding/truncation with divergent `…` handling.
- Fix: `Title()` delegates to `renderWithSelection(false)`; shared render
  helper for note items.

### 1.10 Zettel-browsing TUI duplicated across binaries — MEDIUM
- cmd/note re-implements cmd/zet's zettel list (models at note:622-699 vs
  zet's; TOC editing note:2394 vs zet:1034), with already-diverged post-edit
  behavior (pinboard/README updates).
- Fix: shared zettel-list bubbletea component (in `internal/zet` or a new
  `internal/tui`), embedded by both commands.

### 1.11 Lower-risk repetition — LOW
- Status-update + git-commit flow: todo:1624 vs agenda:907 (error messaging
  and clock-out behavior differ) → helper in `internal/task`.
- Project directory scanning (prjDir/{project}/notes layout re-derived at
  note:1313, note:2072, agenda:2190, todo:1903 with different hidden-dir
  rules) → `internal/project.Scan(cfg)`.
- `printHelp` env-var docs drifting across todo:2399, note:1839, zet:1055,
  inbox:177.
- MCP `case "mcp":` startup boilerplate in all four mains.
- `config.Load()` + exit preamble in every main (mixed `log.Fatal` vs
  `fmt.Fprintf`+`os.Exit`) → `config.MustLoad()`.
- Date styling: todo `getDateStyle` (main.go:336) vs agenda's inline today-
  deadline check duplicated twice within agenda (1578, 1617, 1850).

---

## 2. Over-complex functions to decompose

Prioritized; lengths approximate.

| # | Function | Location | Size | Extraction |
|---|----------|----------|------|------------|
| 1 | `(model).Update` | cmd/todo/main.go:553 | ~600 | Per-mode sub-updates: `updateFiltering`, `updateStatusPicker`, `updateDatePicker`, `updateDetailView`, `updateListKeys`. |
| 2 | `showInteractiveTUI` | cmd/todo/main.go:2493 | ~240 | `newModel(cfg, opts)` constructor; column-width trio (todo:1817-1850) → `task.ColumnWidths(tasks)`. |
| 3 | `showProjectList` | cmd/note/main.go:1414 | ~255 | Separate item building from program lifecycle; scanning → `internal/project`. |
| 4 | zettel-list `Update` ×2 | note:734, zet:250 | ~215 ea | Merge into the shared zettel-list component (see 1.10). |
| 5 | `(*DatePicker).View` | internal/task/datepicker.go:540 | ~200 | `renderTimeSection`, `renderRecurrenceSection`, `renderWarningSection`, `renderHelp`; calendar rendering → own file. |
| 6 | `main` ×3 | todo:2185, zet:854, note:2122 | ~200 ea | Subcommand dispatch table; shared bootstrap (config + colors + flags). |
| 7 | `(*MCPServer).registerTools` | internal/task/mcp.go:302 | ~104 | Tool-definition slice + loop; same pattern in note/zet/goal → `internal/mcputil`. |
| 8 | `ParseLine` | internal/task/task.go:339 | ~106 | `parse.go`: per-token extractors (tags, refs, dates, assignee); reuse from `SetTaskDate`, which currently re-implements token placement. |
| 9 | `(*DatePicker).updateTime` | datepicker.go:234 | ~107 | `adjustTimeField(focus, delta)` + `wrapClock(v, max)`; kills 4 copy-pasted inc/dec branches. |
| 10 | `ProcessFile` | task.go:209 | ~98 | Thin IO wrapper + `buildTaskTree(lines)` for the indent/parent-stack logic. |
| 11 | `(*MCPServer).updateTaskStatus` + siblings | mcp.go:574 | ~91 | `findTask(project, keyword, title, rawTitle)` helper — lookup logic is copy-pasted across `getTask`, `updateTaskStatus`, `scheduleTask`, `clockIn/Out`. |
| 12 | `SyncFromJira` | jira_sync.go:27 | ~81 | `diffTickets(existing, fetched)` returning add/update/remove sets, separate from file mutation. |
| 13 | `QueryClockTable` | clock.go:244 | ~81 | `aggregateClockEntries(entries, window)` separate from the file walk. |
| 14 | `(Model).Update`/`View` | cmd/goal/main.go:394/605 | ~186/105 | Extract the create-goal form into its own sub-model; period math (`getNextQuarterPeriod`, `getNextYear`) → `internal/goal/period.go`. |
| 15 | Filter mini-editors | todo:1151, note:1078, zet:611 | — | One shared text-input filter component. |

Structural theme: the `cmd/*/main.go` files own far too much (todo ≈2800
lines). Target end-state: each main is dispatch + wiring; TUI models,
rendering, watching, and editing live in `internal/`.

---

## 3. Naming: one word per export, prose at call sites

Philosophy (from agentkit): the package name is the namespace. `zet.CreateZettel`
stutters; `zet.Create` reads as prose. `Get` prefixes are non-idiomatic Go.
MCP wire types (`ListZettelsArgs` etc.) are kept as-is below — they mirror the
tool names on the wire and renaming them buys nothing at call sites.

### package `zet` (worst offender — nearly every export stutters)

| Current | Proposed |
|---|---|
| `CreateZettel` | `Create` |
| `ListZettels` | `List` |
| `ListZettelsFromIndex` | `FromIndex` (or fold into `List` with an option) |
| `GetLatestFromIndex` | `Latest` |
| `CountZettels` | `Count` |
| `GetZettelTitle` | `Title` |
| `SearchZettels` | `Search` |
| `SearchZettelTitles` | `Titles` |
| `FindTodos` | `Todos` |
| `UpdateReadme` | `Readme` (or keep verb: `WriteReadme`) |
| `DeleteZettel` | `Delete` |
| `IsValidZettelID` | `ValidID` |
| `FindMatchingZettels` | `Match` |
| `SearchInFile` | delete (use one shared search) |
| `GenerateZettelID` | `NewID` |
| `ReadZettelContent` | `Read` |
| `WriteZettelContent` | `Write` |
| `ListPinnedZettels` | `Pinned` |
| `UpdatePinboard` | `Pinboard` (or `WritePinboard`) |
| `GitCommit` | delete → use `git.CommitFile` |
| `GitDeleteZettel` | fold into `Delete` |
| `GetLastZettelID` | `LastID` |
| `GitInit`, `GitInitAndCommit` | delete → `internal/git` |

Call sites become: `zet.Create(dir, id, title)`, `zet.Search(dir, q)`,
`zet.Delete(dir, id)` — plain English.

### package `task`

| Current | Proposed |
|---|---|
| `ListTasks` | `List` |
| `FilterTasks` | `Filter` |
| `SearchTasks` | `Search` |
| `UpdateTaskStatus` | `SetStatus` |
| `SetTaskDate` | `SetDate` |
| `GetTaskByID` | `ByID` |
| `GetDependencies` / `GetDependents` | `Dependencies` / `Dependents` |
| `GetAllKeywords` / `GetAllKeywordsFlat` | `Keywords` / `KeywordList` |
| `IsCompletedKeyword` | fold into `Status` enum (see 1.4) |
| `ProcessFile` | `Parse` (it parses a file into tasks) |
| `ParseLine` | keep (or `Line`) |
| `StripLinePrefix` | `Strip` |
| `SortByPriority` | `Sort` |
| `SummarizeProjects` | `Summarize` |
| `PartitionRoutines` | `Partition` |
| `HasActiveChildren` | method: `(*Task).Blocked(cfg)` or `Pending` |
| `GroupWithChildren` | `Group` |
| `ReadRawBlock` | `Block` |
| `TruncateString` | `Truncate` |
| `RenderMarkdownDescription` | move to render package as `render.Markdown` |
| `QueryAgenda` | `Agenda` |
| `ParseSchedule` | `Schedule` is taken → `ParseSchedule` OK, or `schedule.Parse` if schedule gets its own package |
| `CompleteRecurringTask` | `Recur` (advance to next occurrence) |
| `SyncFromJira` | `Sync` (context is jira_sync.go; better: `jira.Sync`) |
| `UpdateTaskLabels` | `SetLabels` |
| `IsValidZettelID`, `GetZettelTitle`, `SearchInFile` | delete → use `zet` |
| Clock funcs: `ParseClockEntries`, `QueryClockTable`, `FindActiveClocks`, `IsClockActive`, `FormatDuration`, `RecordStateTransition`, `ParseCompletionEntries`, `ParseStateTransitions` | consider extracting `internal/clock`: `clock.Entries`, `clock.Table`, `clock.Active`, `clock.In`, `clock.Out`, `clock.Duration` |

Note: `task` is overloaded (parsing + scheduling + clocking + agenda + jira +
two TUI widgets). Splitting `clock`, `schedule`, and the pickers into sibling
packages makes single-word names natural instead of forced.

### package `goal`

| Current | Proposed |
|---|---|
| `GoalManager` / `NewGoalManager` | `Manager` / `NewManager` (or just functions on a dir, matching `zet`) |
| `(*Manager).CreateGoal` | `Create` |
| `(*Manager).ListGoals` | `List` |
| `(*Manager).ListGoalsByHorizon` | `ByHorizon` |
| `(*Manager).GetGoalPath` / `GetGoalPathForHorizon` / `GetHorizonPath` | `Path(horizon, name)` — one method, not three |

### package `config`

| Current | Proposed |
|---|---|
| `GeneralConfig` | `General` |
| `(*Config).GetInboxFilePath` | `Inbox` |
| `(*Config).JiraStatusToKeyword` | `Keyword(status)` |
| `DefaultJiraStatusMap` | `JiraDefaults` |

### package `git`

| Current | Proposed |
|---|---|
| `IsGitRepo` | `IsRepo` |
| `FindRepoRoot` | `Root` |
| `GetSignature` | `Signature` |
| `CommitFile` / `CommitFiles` | `Commit(files ...string)` — one variadic function |
| `SSHAuthForRepo` | `Auth` |

### package `jira`

| Current | Proposed |
|---|---|
| `(*Client).SearchIssues` | `Search` |
| `(*Client).GetIssue` | `Issue` |
| `(*Client).GetCurrentUser` | `Me` |
| `(*TokenStore).RunAuthFlow` | `Authorize` |
| `SanitizeContent` | `Sanitize` |

### package `parallel`

| Current | Proposed |
|---|---|
| `CalculateWorkers` | `Workers` |
| `ProcessWithErrors` / `ProcessWithErrorFunc` | `Try` / `TryFunc` (call site: `parallel.Try(items, fn)`) |

### package `colors`

| Current | Proposed |
|---|---|
| `ColorValue` | `Value` |
| `KeywordColor` | `Keyword` |

---

## Suggested execution order

1. **Kill diverged duplication first** (section 1.1-1.8) — introduces
   `internal/editor`, `internal/watch`, extended `internal/colors`, the
   `Status` enum, and de-dupes zet/git/task overlap. This shrinks the surface
   before renaming.
2. **Decompose** (section 2) — split the giant `Update`s and `main`s; extract
   `internal/clock`, shared zettel TUI, `internal/project`.
3. **Rename last** (section 3) — after extraction, most renames are mechanical
   `lsp_rename` sweeps; the AI-facing MCP wire types stay put.
4. Run `go test ./...` after every step (full suite currently green).
5. Housekeeping: `go mod tidy` (gorilla/websocket unused); remove stray
   `".git 2"` / `"editors 2"` directories.
