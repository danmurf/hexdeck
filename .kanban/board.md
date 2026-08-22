# Board — hexdeck
Updated: 2026-08-22T10:36:51Z · 0 backlog · 3 todo · 15 done

## backlog

## todo
- T-16 Visual polish: web board and board.svg look like a modern SaaS tool
  Make the web view (hexdeck web) genuinely beautiful — modern SaaS board aesthetics: clean typography, spaced cards, smooth hover/transition states, badge styling, empty-column placeholders, subtle shadows. Then apply the same design language to the generated board.svg (docs/board.svg + repo board.svg): same visual language so both surfaces look like the same product. Reference the popular-web-designs skill for proven SaaS patterns. Benchmark: Linear or Vercel-level polish.
- T-17 Web board cards should not carry the comment form — comments belong to the ticket detail view
  T-12 moved comments off the board into the ticket view, but the web page still renders an Add-comment form on every collapsed board card. The form (and the comments) belong inside the expanded card detail — the web ticket view. Collapsed cards: id, title, badges only.
- T-18 Web ticket modal: opening a ticket shows the full ticket view (Jira/Linear style)
  Clicking a ticket card opens a full-size modal: description, status, labels, links, claim, comments, and a comment form. Ticket history too. Collapsed board cards stay id/title/badges only.

## done
- T-1 Migrate the build tracker into the board
  PROGRESS.md's phase table becomes tickets. The board is the tracker.
- T-2 Run the build worker against the board — claimed by hermes
  The worker creates, moves, and comments on tickets as it works. Code and ops land in the same commit.
- T-3 Dogfood acceptance: board.md answers where the project is up to — claimed by hermes
  A human reads board.md and can answer the question without opening anything else.
- T-4 Cold-start test: a fresh agent uses the board in one attempt — claimed by hermes
  A fresh agent with zero context, given only the repo, creates a ticket, moves it, and comments correctly within one attempt.
- T-5 V1.1: web view, MCP, snapshots
  Only if V1 earns it. board.svg CI render + README embed, local web view, MCP server, snapshot checkpointing.
- T-6 Coverage badge is dishonest — the E2E tests run the CLI as a subprocess, which go tool cover cannot see (measures 0%). Library alone is 85.6%; the badge's 44.7% is a measurement artifact. Fix: build the CLI with -cover, run E2E with GOCOVERDIR, merge via go tool covdata so subprocess coverage counts. Target ≥80% honest.
  Coverage measurement fix
- T-7 Docs overhaul: current-state documentation, not build history. Diataxis framework (the standard): tutorials, how-to guides, reference, explanation. Strip process narrative (chunk logs, phase history, cold-start report) from README and docs; document what the app IS: how it is built, how to use, how it works. README: quick start, real example, key concepts. Mermaid diagrams with correct fences so GitHub renders them. Simplified technical English throughout. T-7
  Docs reassessment
- T-8 Phase 5 chunk 1: board.svg CI render + README embed
  CI re-renders the demo board's SVG and fails if the committed image drifted. The README embeds the image at the repo root, so GitHub shows the live board on the homepage.
- T-9 Phase 5 chunk 2: local web view
  hexdeck web serves the board in the browser: drag tickets between columns, click to comment, changes panel (diff, suggested message, commit).
- T-10 Phase 5 chunk 3: MCP server
  hexdeck mcp serves the board as an MCP server over stdio: agents ask the board questions without the CLI. Read-only tools: board_show, board_show_ticket, board_log, board_next.
- T-11 Default columns: backlog → todo → done — claimed by danmurf-hermes
  The default flow: plan lots of work in backlog, bring items into todo when ready to pick up, move to done when finished. 'in-progress' becomes opt-in for work that spans multiple PRs. Update InitBoard defaults, the primer, the demo board, docs, and the BUILD-SPEC decision.
- T-12 Comments live on the ticket, not the board view — claimed by danmurf-hermes
  The board shows ticket id and title only. Comments are for the ticket view: hexdeck show <ticket> and the web ticket detail. Remove comment counts and inline comments from board.md; keep them in ticketText.
- T-13 Ticket relationships: blocks, related-to — claimed by danmurf-hermes
  Agents need to know what can run in parallel and what must come first. Add the ability to link tickets: A blocks B, A relates to B. Rendered on the ticket view and considered by pick.
- T-14 Labels on tickets — claimed by danmurf-hermes
  A small set of labels per ticket (e.g. feature, bug, docs, infra), shown on the board card and filterable, to help agents scan and group work.
- T-15 Makefile: common dev tasks in one place
  A Makefile with the standard targets: build, test, vet, fmt, render-check, coverage. So contributors (human or agent) run one command instead of remembering the go incantations. Keep it short — it should not wrap everything, just the common paths.
