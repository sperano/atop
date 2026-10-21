# atop

A `top` for coding agents—hence **agent top**, or `atop`: which agent needs
you, who is running what, who is idle. One table, refreshed every 30 seconds,
over five sources: local Claude Code sessions, Kelos Sessions and Tasks, open
GitHub PRs, and in-progress Vikunja tasks.

```
atop · 2026-10-14 21:55:04 · 2 need you · 3 busy · 2 idle

  TYPE          ORIGIN   NAME                  STATE                AGE DETAIL
● vikunja       vikunja  #182                  reply from claude    48m [claude-opus] CLI: launch specific VMs …
  vikunja       vikunja  #589                  PR open               2h [claude-fable] improve mcp server, let's …
● pr            github   └─ puckdb#14          awaiting your review  2h [claude-fable] Propose new MCP server tools
  vikunja       vikunja  #840                  conflict fix          5m [claude-fable] we need a refresh
  pr            github   └─ atop#3             conflict fix          5m [claude-fable] Show what each agent is doing
  kelos task    github      └─ pr-conflict-9f3 conflict fix          5m https://github.com/sperano/atop/pull/3
  vikunja       vikunja  #612                  implementing         12m [gpt-sol] CLI: list VMs
  kelos task    vikunja  └─ vikunja-cocovm-612 implementing         12m vikunja #612
  local claude  local    atop-da               busy                  3m ~/code/atop tmux main:@5.%6
  vikunja       vikunja  #701                  idle                  1d [claude-opus] write the docs
  kelos session kelos    eric-hollingsworth    idle                  3h hollingsworth
```

Rows that need you come first and are marked `●`; failures and errors are
red, blocked work and conflicts are yellow, active agents are green, and
inactive informational rows are dim. Each group is ordered most recent first.

Related rows form a tree. An agent PR (branch `kelos/vikunja-<id>`) sits
under its Vikunja task, and a Kelos Task sits under the PR it produced or
follows up, else under its Vikunja task. A row whose parent is not shown
stays at the top level. A tree is ranked by its most urgent row, so a PR
awaiting review lifts its task with it. A source that
cannot be read (no kubeconfig, `gh` logged out, Vikunja down) shows as one
`error` row that needs you; the other sources still render.

## Install

```bash
go install github.com/sperano/atop@latest
```

## Requirements

- macOS or Linux.
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
| h | hide the selected row; with hidden rows shown, unhide it |
| H | show or stop showing hidden rows |
| r | refresh now |
| q, ctrl+c | quit |

The table refreshes every `--interval`. While a refresh runs, the status line
says so and the previous rows stay. The selection follows its row across
refreshes. `atop --once` prints one plain-text snapshot, without the hidden
rows, and exits. Colour is off when `NO_COLOR` is set or stdout is not a
terminal.

## Hiding rows

`h` hides the selected row: an abandoned PR, a task you are not going to
look at today. It stays hidden across refreshes and restarts until you
unhide it, whatever its state, so a hidden row that starts needing you
stays hidden. Hidden rows are left out of the header counts, which end with
`N hidden`. `H` lists them again, dim and marked `-`, and `h` on one unhides
it.

Only the row itself is hidden. Rows nested under it (its PR, the Kelos Task
working on it) move to the top level. A source's `error` row cannot be
hidden, since that would also hide the next failure.

A row is remembered by its source and name in `--hidden-file`, a JSON list
you may edit by hand. atop re-reads the file on every refresh and every `h`,
so several atops share it and edits show up at the next refresh. Give an
atop run with other settings (another `--github-owner`, another kubectl
context) its own `--hidden-file`: it would prune the entries it cannot see. An entry is
forgotten once its source is read in full without it (the PR was merged, the
task is done), so a later row with the same name is not hidden by mistake; a
source that failed, or a PR search that hit its 50-PR limit, forgets nothing.
While the file cannot be parsed, atop leaves it alone, says so in the footer
and keeps hidden rows for the current run only; `--once` warns and hides
nothing.

## Types and origins

TYPE says what a row represents. ORIGIN says where its work began: `local`
for local Claude sessions, `kelos` for Kelos Sessions, `github` for PRs,
and `vikunja` for Vikunja tasks. Kelos Tasks use the explicit
`kelos.dev/origin` label, whose supported values are `github`, `vikunja`,
and `wiki`; a missing or unrecognized value is shown as `unknown` rather
than inferred from a Task name or prompt.

## Sources

| Type | Shows | Needs you when | Enter opens |
| --- | --- | --- | --- |
| `local claude` | Claude Code sessions on this machine: `busy`, `idle`, or another raw status | the status is neither busy nor idle | `tmux switch-client` to its pane, when atop runs inside tmux |
| `kelos session` | Kelos Sessions: `busy`/`idle` from the `Active` condition, else the lowercased phase | the phase is `Failed` | the Kelos console |
| `kelos task` | Kelos Tasks. While one runs, its state is what it does: `implementing`, `discussing`, `conflict fix`, `cubic fix` (a review bot's findings; the bot's name comes from the Task), `pr follow-up`, `task follow-up`; a plumbing Task shows its Kelos role (`merge-close`, `pr-open`, `conflict-reconcile`). Otherwise `blocked` (waiting on its blockers), `waiting` (queued behind the branch lock), `pending`, `done` for an hour after completion, then hidden; the activity then starts DETAIL | the Task failed | its PR, else its Vikunja task, else its logs in the console |
| `pr` | open PRs of the owner's non-archived repos (first 50): `draft`, `conflicts`, `checks failing`, `checks running`, `changes requested`, `awaiting your review`; a `kelos:<variant>` label prefixes the title with `[variant]` | conflicts, failing checks, or awaiting review | the PR |
| `vikunja` | tasks labelled in progress and not done, the title prefixed with the agent user they are assigned to: `PR open` (the task carries the `pr open` label Kelos adds while its PR is open), `reply from <user>` (someone else spoke last), `idle` (you spoke last, or nobody did, and no agent is running) | the newest comment is not yours and the PR is not open | the task |

An agent working on a row speaks for it: while a Kelos Task runs on a Vikunja
task or on a PR, that row takes the Task's state (`conflict fix`, `cubic fix`,
`implementing`, ...) and is not flagged, since nothing is waiting on you. A
Task that is `blocked`, `waiting` or `pending` does the same without being
busy. Enter still opens the row's own target. When the Kelos Task source
fails, rows keep their own state. The header's busy count is agents only
(local sessions, Kelos Sessions and Tasks), so a Task speaking for its PR
and its Vikunja task counts once.

A PR or agent reply that would need you but has been untouched for
`--stale-days` shows as `stale` and is not flagged, so abandoned experiments do
not stay at the top forever.

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
| `--pr-open-label` | `ATOP_PR_OPEN_LABEL` | `pr open` |
| `--console-url` | `ATOP_CONSOLE_URL` | `https://kelos.spe.quebec` |
| `--stale-days` | `ATOP_STALE_DAYS` | `30` |
| `--interval` | `ATOP_INTERVAL` | `30s` |
| `--hidden-file` | `ATOP_HIDDEN_FILE` | `<user config dir>/atop/hidden.json` (`~/.config` on Linux, `~/Library/Application Support` on macOS) |
| `--once` | | off |

The Vikunja token comes from `VIKUNJA_API_TOKEN`, else from the
`token-namespace`/`token-secret` Secret's `token-key` via kubectl.
`--operator` is your Vikunja username: a newest comment by anyone else is a
reply waiting on you. `--pr-open-label` is the title of the label Kelos adds
to a task while its PR is open; empty turns `PR open` off.

Kelos Task activities come from the Task itself: its
`vikunja.spe.quebec/role` label (`pr-conflict`, `pr-followup`,
`task-followup`), its name (`vikunja-discuss-*`), and, for a PR follow-up,
the `Review bot:` line and the `<request>` of its prompt, which also names
the PR it works on (`PR: <url>`). A Task atop does not know shows `running`.

## License

GPL-3.0. See [LICENSE](LICENSE).
