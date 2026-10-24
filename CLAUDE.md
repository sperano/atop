# atop — agent top

`atop` is a terminal UI that answers "which coding agent needs me, who is
running what, who is idle". It is a Go port of a Python prototype
(`just agents-top` in sperano/hollingsworth, never merged). Public repo,
GPL-3.0.

## Reference implementation

The prototype lives on branch `feat/agents-top` of sperano/hollingsworth
(closed PR #189). Read it before changing behaviour; port its test cases:

```bash
gh api repos/sperano/hollingsworth/contents/scripts/agents_top_sources.py?ref=feat/agents-top -q .content | base64 -d
gh api repos/sperano/hollingsworth/contents/scripts/agents_top.py?ref=feat/agents-top -q .content | base64 -d
# also agents_top_sources_test.py, agents_top_test.py
```

## Sources

Fetched concurrently. A failing source (no kubeconfig, `gh` logged out,
Vikunja down) becomes one `error` row flagged as needing you; the others still
render.

| Source | How | States | Needs you when |
| --- | --- | --- | --- |
| Local Claude Code sessions | `~/.claude/sessions/<pid>.json` (`pid`, `name`, `cwd`, `tmux`, `status` idle/busy, `statusUpdatedAt` ms); skip files whose pid is missing, ≤0 or dead (`kill(pid,0)`, EPERM = alive) | `busy`, `idle`, other raw status | status is neither busy nor idle |
| Kelos Sessions | `kubectl -n kelos-agents get sessions.kelos.dev -o json`; `Active` condition | `busy`, `idle`, lowercased phase if not Ready | phase Failed |
| Kelos Tasks | `kubectl … get tasks.kelos.dev` + pods `-l kelos.dev/component=task` | while Running, its activity: `implementing` (no role label, `vikunja.spe.quebec/task` label), `discussing` (name `vikunja-discuss-*`), `conflict fix` (role `pr-conflict`), `<bot> fix` (role `pr-followup` whose prompt names a `Review bot:` and whose `<request>` is not a `/` command; `cubic-dev-ai[bot]` → `cubic`), `pr follow-up`, `task follow-up`, any other role as is, else `running`; `blocked` (the `wait-for-blockers` init container is running), `waiting`, `pending`, `done` for 1 h after completion, then hidden; DETAIL starts with the activity when the state is a phase | Failed |
| GitHub PRs | `gh api graphql` search `is:pr is:open owner:<owner> archived:false`, first 50 | `draft`, `conflicts`, `checks failing`, `checks running`, `changes requested`, `review`; a `kelos:<variant>` label → detail `[variant] title` | conflicts, checks failing, review |
| Vikunja | tasks with label id 1 (`in progress`) and `done = false`, paginated; the newest comment per task; DETAIL is `[assignee] title` | `in review` (label titled `pr open`, whatever the comments), `reply from <user>`, `idle` (operator spoke last, or no comments) | the newest comment is not the operator's and the PR is not open |

- **Newest comment:** expand `comments` and `comment_count` on the task list.
  Vikunja caps expanded comments at 50 globally across the task page, so use
  the embedded last comment only when its task's count proves the slice is
  complete. Otherwise fetch one comment with `order_by=desc&per_page=1`.
  Cache by task `updated` and comment count across refreshes.
- **An agent speaks for its row** (`source.Enrich`, run by `FetchAll`): a
  Kelos Task that is not done or failed gives the Vikunja task and PR rows
  it links to (ParentRefs) its state, time and busy flag, and clears their
  needs-you; a running Task beats a queued one, then the most recent. Skipped
  when the Kelos Task source failed.
- **Task links:** a Task's PR is `status.results.pr`, else the `PR: <url>`
  line of its prompt when it has a role label (conflict, follow-up and
  plumbing prompts start with one; an implementation prompt is the owner's
  text and is not read). Its Vikunja task is the `vikunja.spe.quebec/task`
  label, else the `kelos/vikunja-<id>` branch in `spec.branch` or
  `status.results.branch`.
- **Staleness:** a needs-you PR or agent reply untouched for 30 days shows as
  `stale` and is not flagged (abandoned experiment PRs would stay red forever).
- **Vikunja token:** `VIKUNJA_API_TOKEN`, else
  `kubectl -n vikunja get secret agent-claude -o jsonpath={.data.token}`
  (base64). Never print tokens.

## TUI

Bubble Tea v2 stack: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`,
`charm.land/lipgloss/v2` (keep the three on the same major).

- Keys: ↑/↓ and k/j move, pgup/pgdn/home/end, enter opens the row's target,
  `r` refreshes now, `q`/ctrl+c quits, `h` hides the selected row (or
  unhides it while hidden rows are shown), `H` toggles showing hidden rows. Auto-refresh every `--interval`
  (default 2m). While a refresh runs, show it in the status line and keep the
  previous rows. Keep the selection across refreshes by row identity
  (source+name), not by index. Handle window resize.
- Layout: header with time and `N need you · N busy · N idle` (busy counts
  agent rows only: local, kelos session, kelos task; a Task speaking for its
  PR and Vikunja task is one busy agent); columns mark,
  SOURCE, NAME, STATE, AGE, DETAIL (DETAIL takes the remaining width, truncated
  with `…`); footer with key help and the last action or error. Needs-you rows
  are red with `●`, busy green, the rest dim. Sort: needs you, then busy, then
  the rest, each most recent first.
- Tree: a row that obviously belongs to another shown row is nested under it,
  with `├─`/`└─`/`│` guides before its NAME. Links, most specific first:
  - PR → Vikunja task: head branch `kelos/vikunja-<id>`.
  - Kelos Task → its PR (`status.results.pr`, else the prompt's `PR:` line),
    else the Vikunja task (label `vikunja.spe.quebec/task`, else
    `spec.branch` or `status.results.branch` `kelos/vikunja-<id>`).
  A row whose parents are not shown stays top-level. Each level is sorted by
  the rule above, a subtree ranking as its most urgent, most recent row (a
  failing PR lifts its Vikunja task). Colours and the needs-you and idle
  counts are per row; busy counts agents (see Layout).
- Hidden rows: kept by identity (source+name) in `--hidden-file`
  (`internal/hidden`), left out of the table, the header counts (which add
  `N hidden`) and `--once`; `H` lists them dim with mark `-`. Only the row is
  hidden: its children move to the top level. A source's error row cannot be
  hidden. The file is re-read and updated (load, change, save if changed) on
  every refresh and every `h`, so several atops share it; an unparsable file
  is never overwritten. An entry is pruned when its source was read in full
  (no error row, PR search under its limit) and no longer lists it.
- Ages use the largest unit: `59s`, `1h`, `3d`.
- `--once` prints one plain-text snapshot and exits. No colour when `NO_COLOR`
  is set or stdout is not a TTY.

### What enter opens

| Row | Target |
| --- | --- |
| vikunja | `<vikunja web>/tasks/<id>` (the web base is the API URL without `/api/v1`) |
| pr | the PR URL |
| kelos task | its PR (`status.results.pr`); else the Vikunja task in label `vikunja.spe.quebec/task`; else `<console>/api/resources/tasks/<ns>/<name>/logs` (the console has no per-task page) |
| kelos session | the console base URL |
| local claude | `tmux switch-client -t <pane>` when `$TMUX` is set (the stored value looks like `hollingsworth:@5.%6`); otherwise footer "not in tmux" |
| error | nothing |

Open URLs with `open` on darwin and `xdg-open` elsewhere, using exec without a
shell.

## Configuration

Flags with environment-variable defaults. Nothing secret or personal is
hard-coded beyond these defaults:

| Setting | Default |
| --- | --- |
| Kelos namespace | `kelos-agents` |
| GitHub owner | `sperano` |
| Vikunja API URL | `https://vikunja.spe.quebec/api/v1` |
| Vikunja token Secret (namespace/name/key) | `vikunja` / `agent-claude` / `token` |
| Operator Vikunja username | `eric` |
| In-progress label id | `1` |
| PR-open label title | `pr open` |
| Kelos console URL | `https://kelos.spe.quebec` |
| Stale days | `30` |
| Refresh interval | `2m` |
| Hidden-rows file | `<os.UserConfigDir>/atop/hidden.json` |

## Layout and rules

- Module `github.com/sperano/atop`, main package at the repo root, so that
  `go install github.com/sperano/atop@latest` works. Suggested packages:
  `internal/source` (row, local, kelos, github, vikunja, an exec helper with
  context timeouts), `internal/ui` (model, view, keys, open), `internal/config`.
- Inject command runners, HTTP clients, the clock and the opener through small
  interfaces or func fields, so that parsers, fetchers and the model can be
  tested without a cluster.
- Use named constants, not magic numbers. Keep files under ~500 lines and
  functions under ~50. Tests go in sibling `_test.go` files and are
  table-driven. Cover:
  - every parser and state mapping;
  - staleness, sorting and age formatting;
  - enter-target selection;
  - newest-comment pagination (`httptest`);
  - model key handling: moves, clamping, selection kept across refresh, enter
    calling the injected opener.
- Gate before every PR: `gofmt -l .` is empty, `go vet ./...`,
  `go test ./...`, `staticcheck ./...`. CI (`.github/workflows/ci.yml`) runs
  the same on push and PR.
- The README covers install, keys, the sources table (what each shows, when
  it is flagged, what enter opens), configuration, requirements (a kubectl
  context, `gh` logged in), and a note that `~/.claude/sessions` is a Claude
  Code internal, not a documented interface.
- Before a PR: review with the `idiomatic-go` agent, then `pr-challenger`.
- Work in a git worktree under `.claude/worktrees/<branch>`; never on `main`.
  End every task with a PR.
- Before calling the work done, run `atop --once` against the live sources and
  confirm that rows come from all five.
