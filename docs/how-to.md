# How-to guides

One task, one guide. Each guide shows the commands and the result.

## Start a board in a repo

```
cd your-project
hexdeck init --as your-name
```

`init` creates `.kanban/` with the config, the first op, the rendered
board files, and a README that is the whole manual. It also appends one
line to `AGENTS.md` so agents find the board.

Options:

- `--name <board>` — the board name. Default: the repo dir name.
- `--prefix <prefix>` — the ticket id prefix. Default: `T`. Use `--prefix HDX` and tickets are `HDX-1`, `HDX-2`, and so on.

## Track work with tickets

```
hexdeck create "Fix login bug" -d "The login form rejects valid passwords."
hexdeck move T-1 todo
hexdeck comment T-1 "reproduced it"
hexdeck move T-1 done
```

A ticket starts in `backlog`. Move it to `todo` when it is ready to
pick up, and to `done` when it is finished. Comment at milestones worth
remembering — comments are part of the ticket's history.

Every command stages the op and the board files and prints a suggested
commit message. Commit them together with your code — the commit is the
evidence. Or pass `--commit` and the CLI commits for you.

## Claim and release tickets

```
hexdeck pick --as your-name
```

`pick` claims the next `todo` ticket. The default flow has no
`in-progress` column — the claim alone marks the pick, and the ticket
stays in `todo`. The board shows the claim. A claim is a cooperative
lock, not a security boundary — it tells others who is working on the
ticket.

A ticket whose blocker is not in `done` is not pickable — `pick` skips
it and takes the next pickable ticket.

A board that wants an `in-progress` column adds it to
`.kanban/config.json`; `pick` then moves the ticket there. The column is
opt-in for work that spans multiple PRs.

```
hexdeck release T-2 --as your-name
```

`release` clears the claim. The ticket stays in its column.

A claim older than the claim timeout is stale. The board marks it
`(stale claim)`, and `pick` takes the ticket anyway.

## Link tickets

```
hexdeck link T-1 blocks T-3
hexdeck link T-1 related T-4
```

`blocks` says the ticket must come first: T-3 cannot be picked until
T-1 is in `done`. `related` says the tickets are connected but neither
must come first. `pick` considers the links — a blocked ticket is
skipped.

The ticket view shows the links on both sides:

```
$ hexdeck show T-1
T-1 Fix the auth service
status: todo
blocks: T-3
related: T-4
created: 2026-08-21T10:23:50Z
```

Remove a link with `--remove`:

```
hexdeck link T-1 blocks T-3 --remove
```

A ticket can never link to itself.

## Label tickets

```
hexdeck label T-1 feature
hexdeck label T-1 docs
```

A label is one word, at most 20 characters — the small set the board
is meant to hold: `feature`, `bug`, `docs`, `infra`. The board card
shows the labels in brackets after the title, so a scan of the board
groups the work:

```
$ hexdeck show
## todo
- T-1 Fix the auth service [feature]
- T-2 Write the tutorial [docs]
```

The ticket view shows them on a `labels:` line. Remove a label with
`--remove`:

```
hexdeck label T-1 feature --remove
```

## Render the board image

```
hexdeck render --svg
```

`render` rebuilds `board.md` and `board.json` from the ops. `--svg`
also rebuilds `board.svg` — the board image for the README. The image
is deterministic: same ops, same bytes, always.

## Show the board on GitHub

GitHub does not run JavaScript in READMEs, so the board image is the
way to show the board on the repo homepage.

```
hexdeck render --svg
cp .kanban/board.svg board.svg
```

Commit both files. Add the image to the README:

```markdown
![Board](board.svg)
```

CI keeps it honest:

```yaml
- name: board.svg honesty
  run: |
    go build -o hexdeck ./cmd/hexdeck
    ./hexdeck render --svg
    cmp .kanban/board.svg board.svg
    git diff --exit-code -- .kanban/board.svg
```

The image is a projection. CI re-renders it and fails if it drifted —
the board on the homepage is always the projection of the ops.

## Check the board in CI

```
hexdeck render --check
```

`render --check` re-renders the board from the ops and compares it to
the committed files. It fails if they drifted. Add it to your CI
workflow:

```yaml
- name: hexdeck render --check
  run: |
    go build -o hexdeck ./cmd/hexdeck
    ./hexdeck render --check
```

A hand-edited or stale projection fails the build. The board is always
honest.

## Use the web view

```
hexdeck web
```

Opens `http://127.0.0.1:8080` in your browser. The page shows the
board. Drag a ticket to another column to move it. Click a card to
open the ticket — the description, links, comments, and history — and
comment from there.

Every change is an op, staged in git, and listed in the changes panel
on the right — with the staged diff and the suggested commit message.
Edit the message if you want, then press Commit. The changes land in
one commit.

The web view writes through the same path as the CLI, so the two can
never disagree about the board. It binds to 127.0.0.1 only — it is a
local tool, not a service.

## Ask the board from an agent harness

```
hexdeck mcp
```

`hexdeck mcp` serves the board as an MCP server over stdio. Point your
agent harness at it and the agent can ask the board questions without
the CLI:

- `board_show` — the whole board as markdown.
- `board_show_ticket` — one ticket.
- `board_log` — the op timeline, with filters.
- `board_next` — the next todo ticket to pick.

The server is read-only: every tool answers from the projection, and
nothing writes to the board. Stdout carries the protocol; the board
dir is printed to stderr.

## Write ops by hand

The CLI is the preferred way. If it is unavailable, write the op file
yourself. Create `.kanban/ops/<seq>-<uuid>.json`:

```json
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "backlog", "to": "todo" } }
```

One op per file. Never modify an op after it is committed. Then run
`hexdeck render` to rebuild the board files.

## Use a custom ticket prefix

```
hexdeck init --prefix HDX
```

Tickets are `HDX-1`, `HDX-2`, and so on. The prefix lives in
`.kanban/config.json` (`ticketPrefix`). Tickets sort numerically within
a column, whatever the prefix — `HDX-2` comes before `HDX-10`.
