# Agent PM Tool — Build Spec (v3.0)

**Status:** DRAFT v3.0 (Aug 20 2026) — the buildable core, scoped from the v2.1 product spec. For review.
**Author:** Hermes
**Part of:** the Agent PM Tool idea · supersedes the build-level detail of the product spec (v2.1 — still the product vision)
**Research base:** the git-native comparison notes · the event-sourcing mechanics notes · new-entrant sweep Aug 20 2026 (below)

---

## 1. What this is

A **kanban board stored in git, built for agents**. Tickets, columns, comments, and a progress timeline — all plain files in the repo. Agents and humans read and write the same files; the board is always a projection of them. No database, no server, no lock-in.

**What changed since v2.1 (Aug 13):** the field moved. Four new entrants appeared in the last week, and they settle the open design questions:

| Tool | Model | What it proves |
|---|---|---|
| **faru** (fluado/faru) | Cards = markdown files in folders, YAML frontmatter, 3 columns, auto-commit/push, agent dispatch drivers, cron "kata" | "Any tool that can write files can create cards" — the zero-tooling agent story works; setup-prompt bootstrap works |
| **kanban-md** (antopolskiy/kanban-md, Go) | Task files + YAML frontmatter, claims (cooperative locking w/ expiry), `pick --claim` atomic claim-and-move, `--compact` token-efficient output, installable agent SKILL.md files, TUI | The most mature agent-first design: claims + compact output + skills are the "easy for agents" answer |
| **a5c-ai/kanban** (TS) | **One JSON op file per event** in `.kanban/ops/`, seq + opId ordering, state rebuilt by replay, LWW conflict resolution | The op-file event log works in practice — unique filenames = zero merge conflicts, replay is ~100 lines |
| **Vibe Kanban** (BloopAI, 24k★) | Worktree orchestration + auto status from PRs | **Sunsetting as a product** (continues as OSS) — the orchestration lane is crowded and brutal; the git-native board lane is not |

**The decision:** v2.1's storage call stands (git-native, Bruno model), but the buildable core is **the op-file event log** (a5c's shape, refined) — not the DSL ledger, not card folders. Rationale: comments and moves are the high-frequency concurrent writes, and they're both appends; one-file-per-op makes every concurrent write conflict-free by construction. The DSL from v2.1 dies — JSON ops are simpler for agents to write correctly and simpler to parse.

**Scope discipline:** this spec covers the V1 core — create tickets, move tickets, comment, track progress, human-readable board. The v2.1 grand vision (evidence-gated done, Q&A decisions, milestones, agent-run detection, web UI) is deferred with pointers (§7).

---

## 2. Design principles (mapped to the requirements)

1. **Easy for agents cold** → a `README.md` inside the board dir is the entire manual (~40 lines, full text in §3.4). An agent that has never seen the system reads it once and can do everything. A CLI makes the common path one command; the README shows the raw file format for agents without the CLI.
2. **Easy to build a projection** → the projection is a pure function: read ops → sort → fold (§4). Deterministic: same ops, same board, always. ~100 lines in any language.
3. **Human-readable state** → a generated `board.md` (committed) shows the whole board in plain markdown, readable in GitHub, any editor, or a PR diff. `board.json` is the machine view.
4. **Agents can create / move / comment / track** → four op types + a timeline command. That's the whole write surface.
5. **Zero-conflict concurrency** → every event is its own file. Two agents can never touch the same file (verified Aug 13: same-position appends to a shared file ALWAYS conflict; unique files never do).
6. **The same-commit rule** (project decision, Aug 13) → an agent that changes code updates the board in the SAME commit. The commit IS the evidence.

### 2.5 The domain language (glossary)

The ubiquitous language. Code, docs, CLI, and the primer use these exact words — nothing else.

| Term | Meaning |
|---|---|
| **Board** | The kanban view of a project's work: columns of tickets. |
| **Ticket** | One unit of work. Has a title, a description, a column, comments, and a claim. |
| **Column** | A stage of work. Default: `todo`, `in-progress`, `review`, `done`. |
| **Op** | One event. One JSON file in `.kanban/ops/`. The only way the board changes. Ops are never edited or deleted. |
| **Actor** | The writer of an op — an agent or a human. Each actor has a stable name. |
| **Claim** | A mark on a ticket saying one actor is working on it. Claims expire. |
| **Projection** | A view built from the ops: `board.md`, `board.json`, `board.svg`. Always disposable — rebuildable from the ops alone. |
| **Replay** | Reading all ops in order and applying them to build a projection. |
| **Snapshot** | A saved state that makes replay faster (V1.1). Never the source of truth. |
| **Compaction** | Moving old ops to cold storage and replacing them with one summary op (V2). Git history is never rewritten. |
| **Same-commit rule** | Board changes land in the same commit as the code they describe. The commit is the evidence. |
| **Board dir** | The `.kanban/` folder in a repo. Holds the ops, config, and generated views. |
| **Primer** | The README inside the board dir. Teaches any agent the system. |

### 2.6 Domain-driven design (project decision, Aug 20 2026)

- The code speaks the domain language. The glossary above is the contract: code, docs, CLI, and primer all use it.
- One bounded context in V1: **the board**. Everything in the tool serves it.
- The **Ticket** is the aggregate: ops about a ticket are applied in order to build its state.
- The language is the point. Structure follows the language as the codebase grows — no architecture ceremony in V1.

---

## 3. The format

### 3.1 Directory layout

```
.kanban/
  README.md        # the primer — everything an agent needs (full text §3.4)
  config.json      # board config: columns, claim timeout, board name
  ops/             # append-only event log — ONE JSON FILE PER EVENT
    0000000000000001-3f2a9c1b.json
    0000000000000002-8d41e0aa.json
    ...
  board.md         # GENERATED — human-readable board (committed, never hand-edited)
  board.json       # GENERATED — machine-readable projection (committed, never hand-edited)
```

### 3.2 The op (one event = one file)

Filename: `%016d-seq-<opId>.json` — zero-padded seq sorts lexicographically = numerically.

```json
{
  "schema": 1,
  "opId": "3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d",
  "seq": 42,
  "ts": "2026-08-20T14:03:00Z",
  "actor": "claude-a",
  "type": "ticket.moved",
  "ticket": "T-12",
  "payload": { "from": "todo", "to": "in-progress" }
}
```

**Rules:**
- One op per file. Ops are **never edited or deleted** — corrections are new ops.
- `seq` = (highest seq seen in `ops/`) + 1. Collisions across writers are EXPECTED and harmless: replay sorts by `(seq, opId)`, so the order is always deterministic. `ts` is display-only, never used for ordering (wall clocks from different machines can't be trusted).
- `actor` = writer id (agent name or human name), stable per writer.

### 3.3 Op types (V1 — the complete list)

| Type | Payload | Meaning |
|---|---|---|
| `board.created` | `{ "name": "..." }` | once, at init |
| `ticket.created` | `{ "title": "...", "description": "..." }` | new ticket; `ticket` field = its id |
| `ticket.moved` | `{ "from": "todo", "to": "in-progress" }` | column change |
| `ticket.updated` | `{ "title"?: "...", "description"?: "..." }` | rare — title/description edits |
| `comment.added` | `{ "text": "..." }` | a comment on the ticket |
| `ticket.claimed` | `{ "by": "claude-a" }` | cooperative lock (see §3.6) |
| `ticket.released` | `{ "by": "claude-a" }` | release the lock |
| `ticket.archived` | `{}` | hide from the default board (V1.1) |

**Ticket ids:** sequential `T-1`, `T-2`, … assigned by the CLI (next = max existing + 1). The prefix is **configurable** — `ticketPrefix` in `config.json` (default `T`). A board owner can pick any prefix (e.g. `HDX-1`, `KAN-1`), Jira-style. Hand-writing agents check `board show` for the next number. If two `ticket.created` ops land with the same id, the projection keeps the first and renders a warning on the second — visible, not fatal.

### 3.4 README.md — the primer (full text, ships with `board init`)

````markdown
# Board — how to use it

This repo tracks work in `.kanban/`. Everything is plain files in git.

## The one rule
Every change to the board is an **op** — a small JSON file appended to
`.kanban/ops/`. Never edit or delete existing ops. The board is always
rebuilt from the ops, so the ops are the truth.

## Where the project is up to
Read board.md — the committed board view. No CLI needed.

## Commands (preferred)
board create "Title" [-d "description"]   # new ticket
board move T-12 in-progress               # change column
board comment T-12 "text"                 # add a comment
board show                                # print the board (compact)
board show T-12                           # print one ticket
board log --since 2d                      # what happened recently
board pick --as <your-name>               # claim the next todo ticket

## Writing ops by hand (if the CLI is unavailable)
Create `.kanban/ops/<seq>-<uuid>.json`:
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "todo", "to": "in-progress" } }

Op types: ticket.created, ticket.moved, ticket.updated,
comment.added, ticket.claimed, ticket.released, ticket.archived.

Ticket ids are <prefix>-<number>, prefix from config.json
(default T, e.g. T-12).

## Columns
todo → in-progress → review → done   (see config.json)

## Rules
- One op per file. Never modify an op after it's committed.
- Commit ops with your code (same commit) or use `board ... --commit`.
- `git pull --rebase` before appending ops.
- A ticket is done only when moved to done. No other signal counts.
````

### 3.4.1 How agents discover the board (cold start)

An agent that has never heard of the tool must find it, learn it, and use it correctly. Three mechanisms, layered:

1. **The AGENTS.md hook (discovery).** `board init` appends one line to the repo's `AGENTS.md` (or `CLAUDE.md`): *"Work is tracked in `.kanban/` — read `.kanban/README.md` before touching the board."* Every major harness (Claude Code, Codex, Cursor) auto-reads AGENTS.md at session start, so the agent is pointed at the primer before it does anything.
2. **The README primer (the manual).** §3.4 — commands, raw file format, rules. ~40 lines, the whole system.
3. **`board show` (state awareness).** The agent's first action prints the current board in one compact command — it sees the columns, tickets, and claims immediately. Optional: `board context` generates a markdown board summary for embedding in AGENTS.md (kanban-md's pattern), so the board state is in the agent's context before it even runs a command.
4. **Installable skills (optional, kanban-md's pattern).** A SKILL.md that installs into the agent's skills directory and auto-triggers on task-related work — a decision tree + command reference the harness injects automatically. Ships with the CLI; `board skill install`.
5. **Enforcement (the backstop).** The Aug 13 finding: agents ignore convention files ("91 of 100 config files have a smell"; "make the environment enforce it"). The CI check (`board render --check`) makes drift visible structurally, not voluntarily.

**Acceptance test (Phase 4):** a fresh agent with zero context, given only the repo, must create a ticket, move it, and comment correctly within one attempt.

### 3.5 config.json

```json
{
  "schema": 1,
  "board": "my-project",
  "columns": ["todo", "in-progress", "review", "done"],
  "ticketPrefix": "T",
  "claimTimeout": "4h",
  "autoPush": false
}
```

### 3.6 Claims (multi-agent safety)

`board pick --as claude-a` atomically finds the next unclaimed `todo` ticket and appends `ticket.claimed` + `ticket.moved` (to `in-progress`). A claim older than `claimTimeout` is **stale**: `pick` skips it, and the projection marks it `(stale claim)`. This is kanban-md's proven pattern — cooperative locking, not a security boundary. Two agents can never pick the same ticket because each pick is its own op file; the projection resolves any race deterministically (first claim by `(seq, opId)` wins, second renders a warning).

---

## 4. The projection

The board is a **pure function** of the ops. This is the "easy to build a projection" requirement, made exact.

### 4.1 Algorithm

```
project(board_dir) -> BoardState:
  ops = parse every *.json in ops/            # skip unparseable files, warn
  sort ops by (seq asc, opId asc)             # deterministic, never file order
  state = empty board (columns from config.json)
  for op in ops: apply(op, state)
  return state

apply(op, state):
  board.created   -> state.name = payload.name
  ticket.created  -> state.tickets[op.ticket] = { title, description,
                     status: first column, comments: [], created: op.ts,
                     claimed_by: null, archived: false }
  ticket.moved    -> state.tickets[op.ticket].status = payload.to
  ticket.updated  -> merge payload fields into the ticket
  comment.added   -> state.tickets[op.ticket].comments += { ts, actor, text }
  ticket.claimed  -> state.tickets[op.ticket].claimed_by = payload.by
  ticket.released -> state.tickets[op.ticket].claimed_by = null
  ticket.archived -> state.tickets[op.ticket].archived = true
```

**Guarantees:** same ops → same board, always. No timestamps in the fold (only in display). No file order dependence. No hidden state.

### 4.2 Rendering

`board render` runs the projection and writes two files. The board has three views over the same projection — text, image, interactive:

**`board.md`** — the text view (committed; `Updated:` line uses the max op ts, so rendering is deterministic). For agents, PR diffs, and any editor:

```markdown
# Board — my-project
Updated: 2026-08-20T14:03:00Z · 3 todo · 2 in-progress · 1 review · 5 done

## todo
- T-14 Fix login race condition
- T-15 Add sound settings

## in-progress
- T-12 Add payment webhook — claimed by claude-a · 2 comments
- T-13 Refactor level loader — claimed by codex-1

## review
- T-11 Juice pass 2

## done
- T-10 Munitions (4 types)
```

**`board.json`** — the machine view: the full `BoardState` (tickets with comments, claims, transitions). For UIs, MCP, and agents that want structure.

**The interactive view (project decision, Aug 20 2026) — a JS plugin, mermaid-style.** A `hexdeck` code block in any markdown renders a live kanban board wherever JavaScript runs: docs sites (VitePress, Docusaurus, MkDocs), Obsidian, VS Code, the local web view. The block holds a pointer to the state (`.kanban/`), or an inlined snapshot (`hexdeck render --embed`) so the markdown is self-contained. The plugin reads the ops, runs the same projection, and renders the board.

**Why the static image still exists:** GitHub does not run JavaScript in READMEs. Mermaid works there only because GitHub built it in — a new tool cannot get that. So on GitHub the board is a committed image: CI runs `board render --svg` and the README embeds `![Board](board.svg)`. Mermaid solves the same problem the same way — its CLI (`mmdc`) renders to SVG for places where JavaScript cannot run.

**The SVG is deterministic** (project decision, Aug 20 2026): same ops → same bytes, always. No timestamps inside (the `Updated:` line uses the max op ts, like `board.md`), stable ordering, no external fonts. Determinism is what makes it a safe committed artifact — CI can verify it with `render --check`, and it never produces noisy diffs.

### 4.3 Checkpoints, snapshots, compaction

- **Why projections are committed at all: visibility, not speed.** GitHub and PR diffs cannot run the CLI, so the committed `board.md`/`board.svg` is how the board appears there. They are disposable — with zero projections, every view can be regenerated from the ops alone. The speed optimization is the snapshot (V1.1), not the committed files.
- **V1: replay everything, every time.** a5c does exactly this; it's fine to thousands of ops (a few ms).
- **V1.1:** `snapshot.json` (state + last processed opId) — replay from snapshot + delta. Disposable, rebuildable, never the source of truth.
- **V2:** compaction — archive old ops to cold storage, replace with a `Compacted` summary op. **Never rewrite git history** (breaks hashes). Full mechanics in Agent PM Tool - Event Sourcing Mechanics (vault).

### 4.3.1 Why the snapshot can never conflict

- **The snapshot is never committed.** It is a local, gitignored cache — disposable, rebuildable, per-machine. It exists only to make replay faster on that machine. Never shared → never merged → never conflicts. (Fowler: "an application state is purely derivable from the event log, you can cache it anywhere".)
- **The committed projections CAN conflict** — if two writers append ops and re-render at the same time, the ops merge cleanly (unique files) but the projections are same-file edits. The resolution is mechanical:
  1. Merge the ops — always clean.
  2. Re-run `board render` — regenerates the projections from the merged ops.
  3. Commit. The conflict is resolved by regeneration, never by hand-editing.
- **Belt-and-braces:** a custom git merge driver in `.gitattributes` can regenerate the projections automatically on merge (the package-lock.json pattern). Optional — the manual re-render is one command.
- **CI catches drift:** `board render --check` fails if the committed projections don't match the ops.

---

## 5. The CLI

Name: **hexdeck** (project decision, Aug 20 2026 — deck of cards, hex = git's language). Binary: `hexdeck` (alias `hd`). Single static binary, no runtime deps.

### 5.1 Human UX

- **Create tickets:** `board create "Fix login bug"` in the terminal. One command, staged, suggested commit message printed.
- **Visualize the board:** three views over the same projection — `board show` (terminal), `board.md` (text — readable anywhere, diffable in PRs), and the interactive JS plugin (mermaid-style `hexdeck` block, V1.1). On GitHub, where JavaScript cannot run, the board is a CI-rendered `board.svg` in the README (V1.1).
- **Review & commit:** every change is a staged git change with a suggested message; the human reviews the diff and commits — the same path agents use, so history is uniform.
- **V1.1:** local web view — drag tickets between columns, click to comment, with the changes panel (diff → suggested message → commit, the GitHub Desktop pattern from v2.1 §5.3).
- **Optional middle ground:** a TUI (`board tui`) — keyboard-driven board in the terminal, kanban-md's proven pattern. Cheap to add if the CLI table feels thin.

| Command | Effect |
|---|---|
| `board init` | create `.kanban/` (README, config, `board.created` op), stage, suggest commit |
| `board create "Title" [-d "desc"]` | append `ticket.created`, stage, print ticket id |
| `board move T-12 in-progress` | append `ticket.moved`, stage |
| `board comment T-12 "text"` | append `comment.added`, stage |
| `board show` | print the board — compact, one line per ticket (token-efficient for agents) |
| `board show T-12` | print one ticket: fields + comments + history |
| `board show --json` | print `board.json` |
| `board log [--since 2d] [--ticket T-12] [--actor claude-a]` | print the event timeline |
| `board pick --as claude-a` | claim + move the next todo ticket (atomic: two ops) |
| `board release T-12 --as claude-a` | append `ticket.released` |
| `board render` | rebuild `board.md` + `board.json` from ops |
| `board render --svg` | rebuild `board.svg` — the deterministic board image for the README |
| `board render --check` | exit 1 if committed `board.md`/`board.json` don't match the ops (for CI) |

**Git behaviour (the same-commit rule):**
- Every command writes the op file and **stages it**, then prints the suggested commit message (`board: move T-12 → in-progress`). Nothing is committed implicitly.
- `--commit` commits immediately with the suggested message (for board-only work).
- Agents doing code work run `board move ...` (staged), then commit code + op together — the commit IS the evidence.
- Before appending, the CLI runs `git pull --rebase` (skipped if `--no-pull`).

---

## 6. Use cases — the different ways this could be used

1. **Agent fleet mission control (core).** A solo dev runs 3–5 agents in worktrees; the board shows who's doing what, what's stuck, what's done. `board show` is the "where is my project up to" answer.
2. **Single agent, long project.** One agent, many sessions, context resets. The board survives every reset; a fresh agent reads README + `board show` and picks up.
3. **Human-only kanban.** A personal task list in any repo. Works with zero agents — it's just a kanban in git.
4. **Cross-project board repo.** A dedicated repo holding the board for several projects (faru's pattern) — board ops in one repo, code elsewhere. `board` works from any directory with `--dir`.
5. **Public progress display.** CI runs `board render --svg`; README embeds `![Board](board.svg)`. GitHub shows the live board on the repo homepage (V1). Docs sites and Obsidian embed the interactive JS plugin (V1.1).
6. **MCP integration (V1.1).** Expose the board as an MCP server — any agent asks "what's the status?", "any open tickets?", "what happened since Tuesday?" without the CLI.
7. **Team mode (V2).** Multiple humans + agents; `review` column becomes a real gate; assignees become first-class.
8. **Evidence-gated done (V2).** The v2.1 wedge: `done` requires proof (commit/test evidence) — the op log already has the shape; add `evidence.attached` ops and a CI gate.
9. **Dogfood.** Replaces hand-maintained progress files in personal repos — the board IS the progress file, generated instead of hand-maintained.

---

## 7. What's in / what's out (vs v2.1)

| v2.1 feature | V1 (this spec) | Later |
|---|---|---|
| Tickets, columns, moves | ✅ | |
| Comments | ✅ | |
| Progress timeline (`board log`) | ✅ | |
| Claims / pick (multi-agent) | ✅ | |
| Human-readable board (`board.md`) | ✅ | |
| Same-commit rule | ✅ | |
| README primer for agents | ✅ | |
| DSL ledger (Bru-style) | ❌ replaced by JSON ops | — |
| Evidence-gated done (commit/test linkage) | ❌ | V2 |
| Q&A decisions channel | ❌ | V2 |
| Milestones + roll-up | ❌ | V2 |
| Agent-run detection (sessions) | ❌ | V2 |
| Web UI + changes panel | ❌ | V1.1 (web view) |
| MCP server | ❌ | V1.1 |
| board.svg + README embed | ✅ | |
| JS plugin (mermaid-style interactive board) | ❌ | V1.1 |
| Snapshots / compaction | ❌ | V1.1 / V2 |
| Hash chain (tamper-evidence) | ❌ | V2 |

---

## 8. Build plan (agent-run-sized phases)

Each phase = one agent run, TDD, commit at the end. Stack: **Go** (recommended — single static binary, zero deps, kanban-md proves the pattern; alternative: Node/TS, a5c proves it — open question §11).

### 8.0 Quality bar (project decision, Aug 20 2026 — applies to every phase, every language)

**Code:**
- **Idiomatic.** Go: stdlib-first, effective-Go conventions, `gofmt` clean, no cleverness, no unnecessary dependencies. TS: strict mode, idiomatic React, no any-casts, no dead code.
- **Well tested.** TDD per phase. Go: table-driven tests + golden files for the projection. TS: Vitest. Concurrency paths get the race detector / merge-matrix scenarios.
- **Clean.** Small functions, single responsibility, no commented-out code, no TODOs that outlive their phase. Every phase ends with lint + tests + build green.
- **CI gates (Phase 3.5):** lint → test → build on every push; `board render --check` once the board exists.

**Docs (human-facing):**
- **Simplified technical English** — short sentences, active voice, plain words. No jargon where a plain word works. A new engineer (or agent) reads it once and understands.
- Written **as work goes along**, not at the end: README + `docs/architecture.md` + `docs/components.md`.
- **A feature ships with its doc line in the same commit** — no undocumented features, ever.

**Phase 0 — Decisions (human, not an agent run).** Answer §11. Confirm stack.

**Phase 1 — Core library.** Go module: op schema + parse, sort `(seq, opId)`, fold, render `board.md` + `board.json` + `board.svg`. TDD with golden files: fixture ops dirs → exact expected `board.md`/`board.json`/`board.svg` (byte-for-byte — the SVG is deterministic). Acceptance: `go test ./...` green; golden tests cover every op type + seq-collision + duplicate-ticket-id + unparseable-op cases.

**Phase 2 — CLI.** All commands in §5 + git staging/`--commit`/pull-before-append. E2E tests in temp repos (init → create → move → comment → show → log → render). Acceptance: full command matrix green; `board render --check` passes on a fresh board.

**Phase 3 — Concurrency hardening.** Re-run the 18-scenario merge matrix (two writers appending concurrently, stale checkouts, seq collisions) against the real tool. Claim expiry + stale-claim rendering. Acceptance: zero conflicts in every scenario; projection identical on both sides after merge.

**Phase 3.5 — CI pipeline (GitHub Actions).** The quality bar (§8.0) promises lint → test → build gates on every push, and Phase 4's acceptance needs `board render --check` in CI. This phase builds the pipeline while the core is complete but before dogfood relies on it. Chunks:
1. `ci.yml`: gofmt check, `go vet`, `go test ./...` (with `-race`), `go build`. Runs on every push and PR. Acceptance: green on the repo's own history; a deliberately broken change fails the run.
2. Wire `board render --check` into the workflow — the CI-honesty job. Acceptance: a drifted committed board fails CI.
3. README badges: CI passing (shields.io GitHub Actions badge — no account needed) and code coverage (`go test -coverprofile`; codecov.io for a public repo, or a shields endpoint — decided in-phase). Acceptance: both badges render on the README with real values.

**Phase 4 — Dogfood on a real project.** Install the binary, `board init` in a real repo, migrate its existing progress notes into tickets, run one real worker phase against the board (worker creates/moves/comments as it works). The `board render --check` job from Phase 3.5 is already in CI — dogfood verifies it fires on real drift. Acceptance: a human reads `board.md` and can answer "where is the project up to" without opening anything else; worker's commits show code + ops together.

**Phase 5 — V1.1 (only if V1 earns it).** board.svg CI render + README embed; local web view; MCP server; snapshot checkpointing.

---

## 9. Verification & acceptance (the whole build)

1. **Golden projection tests** — fixture ops → exact board.md/board.json (Phase 1).
2. **Merge matrix** — concurrent appends never conflict (Phase 3, the Aug 13 recipe).
3. **Cold-start test** — a fresh agent with zero context, given only the repo + README, must create a ticket, move it, and comment correctly within one attempt (Phase 4, run against a real agent).
4. **Dogfood** — a real project runs a phase through the board; the human reviews the result (Phase 4).
5. **CI honesty** — `board render --check` fails if the committed board drifts from the ops (Phase 3.5 pipeline, verified in Phase 4).

---

## 10. Risks & pitfalls

| Risk | Mitigation |
|---|---|
| Agents hand-write ops with wrong seq | Harmless — `(seq, opId)` tie-break keeps replay deterministic |
| Agents edit ops instead of appending | README rule + `render --check` in CI catches drift |
| Stale checkout → seq collisions | Expected and harmless; `pull --rebase` before append |
| Two agents pick the same ticket | Impossible to *win* twice — first claim by `(seq, opId)` wins, second renders a warning |
| Duplicate ticket ids (hand-written) | Projection keeps first, warns on second |
| `board.md` drift (hand-edited) | It's generated — `render --check` fails; README says never hand-edit |
| Op files bloat the repo | Fine to thousands of ops; snapshots V1.1, compaction V2 |
| Agents ignore the board entirely | Same-commit rule + CI check make it structural, not voluntary (the Aug 13 finding: "make the environment enforce it") |

---

## 11. Open questions

- [x] **Stack** — DECIDED (Aug 20 2026): **Go** (spec recommendation; veto via PR comment).
- [x] **Board dir** — DECIDED (Aug 20 2026): `.kanban/` (hidden, out of the way).
- [x] **Columns** — DECIDED (Aug 20 2026): `todo / in-progress / review / done`.
- [x] **Claims in V1** — DECIDED (Aug 20 2026): yes — the multi-agent safety.
- [x] **Dogfood target** — DECIDED (Aug 20 2026): **hexdeck itself**. Once the CLI works (Phase 2), `hexdeck init` in the hexdeck repo, migrate PROGRESS.md's phase table into tickets, and the build worker uses the board instead of PROGRESS.md. The tool tracks its own build.
- [x] **Name** — DECIDED (Aug 20 2026): **hexdeck**. hexdeck.com free; no same-category collisions (Steam game + termux theme, both different fields); Companies House clean.

---

## Sources (new research, Aug 20 2026)

- github.com/fluado/faru (README fetched — card folders, YAML frontmatter, 3 columns, auto-commit, dispatch drivers, Dojo cron, setup-prompt bootstrap)
- github.com/antopolskiy/kanban-md (README + SKILL.md fetched — claims w/ expiry, `pick --claim`, `--compact` output, installable agent skills, TUI, config.yml statuses/WIP/classes)
- github.com/a5c-ai/kanban (source fetched — `ops.ts`/`state.ts`: one JSON op file per event, `%016d-seq-opId.json` naming, replay + LWW fold, retry-on-EEXIST append)
- vibekanban.com + BloopAI/vibe-kanban (24k★, sunsetting as product, continues OSS — market signal)
- Prior research: the git-native comparison notes · the event-sourcing mechanics notes (18 git merge experiments, Bruno internals, git-bug/Grite/git-native-issue, verify-before-done pattern)
