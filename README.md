# atop

A `top` for coding agents: which agent needs you, who is running what, who is
idle. One table, refreshed every 30 seconds, over five sources: local Claude
Code sessions, Kelos Sessions and Tasks, open GitHub PRs, and in-progress
Vikunja tasks.

```
atop · 2026-10-04 21:55:04 · 3 need you · 2 busy · 4 idle

  SOURCE        NAME                  STATE              AGE DETAIL
● vikunja       #625                  reply from claude  11m general idea — Moved to a standalone Go …
● pr            puckdb#14             review              2h [claude] Propose new MCP server tools
  kelos task    vikunja-cocovm-182    running            29m vikunja #182
  local claude  atop-da               busy                3m ~/code/atop tmux main:@5.%6
  kelos session eric-hollingsworth    idle                3h hollingsworth
```

Rows that need you come first, red and marked `●`; then busy rows, green;
then the rest, dim. Each group is ordered most recent first. A source that
cannot be read (no kubeconfig, `gh` logged out, Vikunja down) shows as one
`error` row that needs you; the other sources still render.

## Install

```bash
go install github.com/sperano/atop@latest
```

## Requirements

- `kubectl` with a current context that can read the Kelos namespace (and the
  Vikunja token Secret, unless `VIKUNJA_API_TOKEN` is set).
- `gh` logged in (`gh auth status`).
- `tmux`, to jump to a local Claude Code session with enter.

## Keys

| Key | Action |
| --- | --- |
| ↑/↓, k/j | move |
| pgup/pgdn, home/end | move by a page, to the first or last row |
| enter | open the row's target (see below) |
| r | refresh now |
| q, ctrl+c | quit |

The table refreshes every `--interval`. While a refresh runs, the status line
says so and the previous rows stay. The selection follows its row across
refreshes. `atop --once` prints one plain-text snapshot and exits. Colour is
off when `NO_COLOR` is set or stdout is not a terminal.

## Sources

| Source | Shows | Needs you when | Enter opens |
| --- | --- | --- | --- |
| `local claude` | Claude Code sessions on this machine: `busy`, `idle`, or another raw status | the status is neither busy nor idle | `tmux switch-client` to its pane, when atop runs inside tmux |
| `kelos session` | Kelos Sessions: `busy`/`idle` from the `Active` condition, else the lowercased phase | the phase is `Failed` | the Kelos console |
| `kelos task` | Kelos Tasks: `running`, `blocked` (waiting on its blockers), `done` for an hour after completion, then hidden | the Task failed | its PR, else its Vikunja task, else its logs in the console |
| `pr` | open PRs of the owner's non-archived repos (first 50): `draft`, `conflicts`, `checks failing`, `checks running`, `changes requested`, `review`; a `kelos:<variant>` label prefixes the title with `[variant]` | conflicts, failing checks, or awaiting review | the PR |
| `vikunja` | tasks labelled in progress and not done: `reply from <user>`, `waiting on agent` (you spoke last), `in progress` (no comments) | the newest comment is not yours | the task |

A PR or agent reply that would need you but has been untouched for
`--stale-days` shows as `stale` and is not flagged, so abandoned experiments do
not stay red forever.

Local sessions are read from `~/.claude/sessions/<pid>.json`, skipping those
whose process has exited. That directory is a Claude Code internal, not a
documented interface, and may change without notice.

## Configuration

Every flag defaults to an environment variable, then to a built-in value.

| Flag | Environment | Default |
| --- | --- | --- |
| `--namespace` | `ATOP_NAMESPACE` | `kelos-agents` |
| `--github-owner` | `ATOP_GITHUB_OWNER` | `sperano` |
| `--vikunja-url` | `VIKUNJA_URL` | `https://vikunja.spe.quebec/api/v1` |
| `--token-namespace` | `ATOP_TOKEN_NAMESPACE` | `vikunja` |
| `--token-secret` | `ATOP_TOKEN_SECRET` | `agent-claude` |
| `--token-key` | `ATOP_TOKEN_KEY` | `token` |
| `--operator` | `ATOP_OPERATOR` | `eric` |
| `--in-progress-label` | `ATOP_IN_PROGRESS_LABEL` | `1` |
| `--console-url` | `ATOP_CONSOLE_URL` | `https://kelos.spe.quebec` |
| `--stale-days` | `ATOP_STALE_DAYS` | `30` |
| `--interval` | `ATOP_INTERVAL` | `30s` |
| `--once` | | off |

The Vikunja token comes from `VIKUNJA_API_TOKEN`, else from the
`token-namespace`/`token-secret` Secret's `token-key` via kubectl.
`--operator` is your Vikunja username: a newest comment by anyone else is a
reply waiting on you.

## License

GPL-3.0. See [LICENSE](LICENSE).
