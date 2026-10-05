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
| Kelos Tasks | `kubectl … get tasks.kelos.dev` + pods `-l kelos.dev/component=task` | `running`, `blocked` (the `wait-for-blockers` init container is running), `done` for 1 h after completion, then hidden | Failed |
| GitHub PRs | `gh api graphql` search `is:pr is:open owner:<owner> archived:false`, first 50 | `draft`, `conflicts`, `checks failing`, `checks running`, `changes requested`, `review`; a `kelos:<variant>` label → detail `[variant] title` | conflicts, checks failing, review |
| Vikunja | tasks with label id 1 (`in progress`) and `done = false`, paginated; the newest comment per task | `reply from <user>`, `waiting on agent` (operator spoke last), `in progress` (no comments) | the newest comment is not the operator's |

- **Newest comment:** comments are paginated oldest first, so fetch with
  `per_page=1` and read `x-pagination-total-pages` (case-insensitive header),
  then fetch that last page. Element `[-1]` of an unpaged response is wrong on
  long threads.
- **Staleness:** a needs-you PR or agent reply untouched for 30 days shows as
  `stale` and is not flagged (abandoned experiment PRs would stay red forever).
- **Vikunja token:** `VIKUNJA_API_TOKEN`, else
  `kubectl -n vikunja get secret agent-claude -o jsonpath={.data.token}`
  (base64). Never print tokens.

## TUI

Bubble Tea v2 stack: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`,
`charm.land/lipgloss/v2` (keep the three on the same major).

- Keys: ↑/↓ and k/j move, pgup/pgdn/home/end, enter opens the row's target,
  `r` refreshes now, `q`/ctrl+c quits. Auto-refresh every `--interval`
  (default 30s). While a refresh runs, show it in the status line and keep the
  previous rows. Keep the selection across refreshes by row identity
  (source+name), not by index. Handle window resize.
- Layout: header with time and `N need you · N busy · N idle`; columns mark,
  SOURCE, NAME, STATE, AGE, DETAIL (DETAIL takes the remaining width, truncated
  with `…`); footer with key help and the last action or error. Needs-you rows
  are red with `●`, busy green, the rest dim. Sort: needs you, then busy, then
  the rest, each most recent first.
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
| Kelos console URL | `https://kelos.spe.quebec` |
| Stale days | `30` |
| Refresh interval | `30s` |

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
