# vigie

A `top` for coding agents, originally named `atop` for **agent top**, now called
`vigie`: which agent needs you, who is running what, who is idle. One table,
refreshed every two minutes, covers five sources: local Claude Code sessions,
Kelos Sessions and Tasks, open GitHub PRs, and in-progress Vikunja tasks.

```
vigie · 2026-10-14 21:55:04 · 2 need you · 3 busy · 2 idle

  TYPE          ORIGIN   NAME                               STATE                  AGE DETAIL
● vikunja       vikunja  #182                               reply from claude      48m [claude-opus] CLI: launch specific VMs …
  vikunja       vikunja  #589                               PR open                 2h [claude-fable] improve mcp server, let's …
● pr            github   └─ puckdb#14                       awaiting your review    2h [claude-fable] Propose new MCP server tools
  vikunja       vikunja  #840                               conflict fix            5m [claude-fable] we need a refresh
  pr            github   └─ vigie#3                         conflict fix            5m [claude-fable] Show what each agent is doing
  kelos task    github      └─ pr-conflict-9f3              conflict fix            5m https://github.com/sperano/vigie/pull/3
  vikunja       vikunja  #612                               implementing           12m [gpt-sol] CLI: list VMs
  kelos task    vikunja  └─ vikunja-cocovm-612              implementing           12m vikunja #612
  local claude  local    vigie-da                           busy                    3m ~/code/vigie tmux main:@5.%6
  vikunja       vikunja  #701                               idle                    1d [claude-opus] write the docs
  kelos session kelos    eric-hollingsworth                 idle                    3h hollingsworth
```

Rows that need you come first and are marked `●`; failures and errors are
red, blocked work and conflicts are yellow, active agents are green, and
inactive informational rows are dim. Attention-only rows with no failure,
block, or active agent state use normal text. Each group is ordered most recent
first.

Related rows form a tree. An agent PR (branch `kelos/vikunja-<id>`) sits
under its Vikunja task, and a Kelos Task sits under the PR it produced or
follows up, else under its Vikunja task. A row whose parent is not shown
stays at the top level. A tree is ranked by its most urgent row, so a PR
awaiting review lifts its task with it. A source that
cannot be read (no kubeconfig, no GitHub token, Vikunja down) shows as one
`error` row that needs you; the other sources still render.

## Install

```bash
go install github.com/sperano/vigie@latest
```

## Requirements

- macOS or Linux.
- Access to the cluster: in a pod, its service account; elsewhere a
  kubeconfig (`KUBECONFIG`, else `~/.kube/config`, current context), as
  `kubectl` would use it. It needs `get` and `list` on Kelos Sessions, Tasks
  and pods in the Kelos namespace, and `get` on the Vikunja token Secret
  unless `VIKUNJA_API_TOKEN` is set. `kubectl` itself is not needed.
- A GitHub token (see Configuration). `gh` is needed only to supply it on a
  laptop.
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
refreshes. `vigie --once` prints one plain-text snapshot, without the hidden
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
you may edit by hand. `vigie` re-reads the file on every refresh and every `h`,
so several processes share it and edits show up at the next refresh. Give a
`vigie` run with other settings (another `--github-owner`, another kubeconfig
context) its own `--hidden-file`: it would prune the entries it cannot see. An entry is
forgotten once its source is read in full without it (the PR was merged, the
task is done), so a later row with the same name is not hidden by mistake; a
source that failed, or a PR search that hit its 50-PR limit, forgets nothing.
While the file cannot be parsed, vigie leaves it alone, says so in the footer
and keeps hidden rows for the current run only; `--once` warns and hides
nothing.

The default file is `<user config dir>/vigie/hidden.json`. If it does not yet
exist, `vigie` reads the former `atop/hidden.json` and writes the migrated set
to the new path on the first change.

## Server

`vigie serve` runs an HTTP server instead of the TUI. It refreshes every
`--interval` in the background, keeps the last snapshot, and serves it as
JSON, already nested, sorted and counted as the TUI draws it:

| Endpoint | Does |
| --- | --- |
| `GET /api/v1/snapshot` | the snapshot: `rows` (source, name, origin, depth, guides, mark, state, since, age, detail, target URL, severity, needs you, busy, hidden), `counts`, `fetched_at` (null before the first refresh), `refreshing`, and `error`, the last hidden-row update's error |
| `GET /api/v1/events` | Server-Sent Events: a `snapshot` event with the whole snapshot on connection, when a refresh starts or ends, and when a row is hidden or unhidden; a `: keepalive` comment every 15 s. A client that falls behind gets the latest snapshot, not every one |
| `POST /api/v1/refresh` | starts a refresh, or joins the running one (never two at once); answers `202` with the snapshot at once, and the new one arrives on the event stream. Within 10 s of the last refresh's end it starts none and answers `429` with `Retry-After` |
| `PUT /api/v1/hidden/{source}/{name}` | hides a row of the last refresh: `204`; `404` for an unknown row, `409` for a source's error row, `500` when the hidden file cannot be read or written |
| `DELETE /api/v1/hidden/{source}/{name}` | unhides a row: `204` |
| `GET /healthz` | `200` while the process is up |
| `GET /readyz` | `200` once the first refresh is done, else `503` |

`show_hidden=true` on the snapshot, events and refresh lists the hidden rows
too, marked `-` with `hidden` set, as `H` does. Path segments are
URL-encoded (`/api/v1/hidden/pr/vigie%233`). Each refresh is cut off after a
minute. SIGTERM or an interrupt ends the event streams and shuts down
gracefully.

The server leaves out local Claude sessions: a pod has no
`~/.claude/sessions`, and a browser cannot switch tmux panes. It keeps its
own `--hidden-file`, with the same rules as the TUI's, except that it never
forgets hidden local sessions, which it does not read; in a container, point
it at a writable volume.

### Web UI

The server also serves a web UI at `/`: the TUI's table in a browser, live
from the event stream, with its header, columns, colours (light or dark, as
the system says), status line and keys (`↑/↓`, `k/j`, `pgup/pgdn`,
`home/end`, `enter`, `r`, `h`, `H`; no `q`). Every name with a target is a
link, so a middle-click opens it in a new tab; enter opens the selected
row's target in a new tab too. A lost stream is said in the status line and
reconnected, with the previous rows kept. Any path that is not an API path
or a file of the UI loads the UI.

`go build` and `go install` embed a placeholder page that says the UI was
not built. To embed the UI, build it first, then build with the `webdist`
tag:

```bash
(cd web && npm ci && npm run build)
go build -tags webdist .
```

For work on the UI, `npm run dev` in `web/` serves it with Vite and proxies
`/api` to a `vigie serve` on `localhost:8080`. Its checks are `npm run lint`,
`npm run check` and `npm test` (the component tests run in Chromium:
`npx playwright install chromium` once).

**The server has no authentication.** Anyone who reaches it can read every
row and hide any of them. Bind it with `--listen` (default `:8080`, every
interface) to an address only an authenticating proxy reaches, such as
`127.0.0.1:8080` behind a forward-auth proxy; an empty address is refused. Its event streams stay open:
give the proxy's route no request timeout. The web UI has no login of its
own either: the proxy's is the only one.

### Image

The `Dockerfile` builds an image of `vigie serve` with the UI embedded, for
`linux/amd64` and `linux/arm64`, without emulation (Node and Go run on the
build platform, Go cross-compiles):

```bash
docker buildx build --platform linux/amd64,linux/arm64 .
```

It is `scratch` with the binary, the CA bundle (GitHub and Vikunja are
reached over HTTPS) and the licenses of everything it ships under
`/licenses`. It runs as `65532:65532`, listens on `8080`, and keeps its hidden
rows in `/data/hidden.json` (`VIGIE_HIDDEN_FILE`): mount a writable volume on
`/data` to keep them across restarts. Probe `GET /healthz` and
`GET /readyz`. In a pod it reads Kelos (and the Vikunja token Secret, unless
`VIKUNJA_API_TOKEN` is set) with its service account, and takes the GitHub token from `GH_TOKEN` or
`GITHUB_TOKEN` (there is no `gh` in the image).

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
| `local claude` | Claude Code sessions on this machine: `busy`, `idle`, or another raw status | the status is neither busy nor idle | `tmux switch-client` to its pane, when vigie runs inside tmux |
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
The former `ATOP_*` name remains a fallback for each renamed `VIGIE_*`
variable when the new name is unset.

| Flag | Environment | Default |
| --- | --- | --- |
| `--namespace` | `VIGIE_NAMESPACE` | `kelos-agents` |
| `--github-owner` | `VIGIE_GITHUB_OWNER` | `sperano` |
| `--vikunja-url` | `VIKUNJA_URL` | `https://vikunja.spe.quebec/api/v1` |
| `--token-namespace` | `VIGIE_TOKEN_NAMESPACE` | `vikunja` |
| `--token-secret` | `VIGIE_TOKEN_SECRET` | `agent-claude` |
| `--token-key` | `VIGIE_TOKEN_KEY` | `token` |
| `--operator` | `VIGIE_OPERATOR` | `eric` |
| `--in-progress-label` | `VIGIE_IN_PROGRESS_LABEL` | `1` |
| `--pr-open-label` | `VIGIE_PR_OPEN_LABEL` | `pr open` |
| `--console-url` | `VIGIE_CONSOLE_URL` | `https://kelos.spe.quebec` |
| `--stale-days` | `VIGIE_STALE_DAYS` | `30` |
| `--interval` | `VIGIE_INTERVAL` | `2m` |
| `--hidden-file` | `VIGIE_HIDDEN_FILE` | `<user config dir>/vigie/hidden.json` (`~/.config` on Linux, `~/Library/Application Support` on macOS) |
| `--once` | | off |
| `--listen` (`vigie serve` only) | `VIGIE_LISTEN` | `:8080` |

The Vikunja token comes from `VIKUNJA_API_TOKEN`, else from the
`token-namespace`/`token-secret` Secret's `token-key`, read through the
Kubernetes API. Kelos is reached in-cluster when `KUBERNETES_SERVICE_HOST` is
set, else through the kubeconfig.

PRs are read from `https://api.github.com/graphql` with the first token
found: `GH_TOKEN`, else `GITHUB_TOKEN`, else the output of `gh auth token`.
To read check states on private repositories the token must be a GitHub App
installation token or a classic PAT: a fine-grained PAT cannot read them.
`--operator` is your Vikunja username: a newest comment by anyone else is a
reply waiting on you. `--pr-open-label` is the title of the label Kelos adds
to a task while its PR is open; empty turns `PR open` off.

Kelos Task activities come from the Task itself: its
`vikunja.spe.quebec/role` label (`pr-conflict`, `pr-followup`,
`task-followup`), its name (`vikunja-discuss-*`), and, for a PR follow-up,
the `Review bot:` line and the `<request>` of its prompt, which also names
the PR it works on (`PR: <url>`). A Task vigie does not know shows `running`.

## License

GPL-3.0. See [LICENSE](LICENSE).
