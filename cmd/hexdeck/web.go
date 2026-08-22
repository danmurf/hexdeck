package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/danmurf/hexdeck"
)

// webPage is the whole web view: one HTML file with the board, the
// drag-and-drop, the comment form, and the changes panel. It is a
// render — the web view of the board — so it is deterministic and
// pinned by a golden test. The page talks to the API endpoints below;
// it never touches the board files itself.
var webPage = []byte(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>hexdeck</title>
<style>
:root { --bg:#f7f8f8; --card:#ffffff; --border:#e6e8eb; --text:#1f2328; --meta:#6b7280; --accent:#5e6ad2; --accent-soft:#eef0fe; --amber:#fff8e5; --amber-text:#92600a; }
* { box-sizing: border-box; }
body { margin:0; font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; background:var(--bg); color:var(--text); }
header { display:flex; align-items:baseline; gap:12px; padding:14px 24px; background:var(--card); border-bottom:1px solid var(--border); }
header h1 { font-size:16px; margin:0; letter-spacing:-.01em; }
header h1::before { content:""; display:inline-block; width:10px; height:10px; border-radius:3px; background:var(--accent); margin-right:8px; }
header .updated { font-size:12px; color:var(--meta); }
main { display:flex; gap:16px; padding:20px 24px; align-items:flex-start; }
#board { display:flex; gap:16px; flex:1; overflow-x:auto; align-items:flex-start; }
.column { width:248px; flex:0 0 auto; background:#eef0f3; border-radius:12px; padding:10px; transition:background .15s ease, outline .15s ease; }
.column.dragover { background:var(--accent-soft); outline:2px dashed var(--accent); outline-offset:-2px; }
.column h2 { font-size:13px; margin:0 0 10px; color:var(--meta); display:flex; align-items:center; gap:6px; }
.column h2 .count { font-size:11px; font-weight:600; background:var(--card); border:1px solid var(--border); color:var(--meta); border-radius:10px; padding:0 7px; }
.cards { min-height:40px; }
.cards.empty { border:1px dashed var(--border); border-radius:10px; color:var(--meta); font-size:12px; padding:18px 8px; text-align:center; min-height:64px; }
.card { background:var(--card); border:1px solid var(--border); border-radius:10px; padding:10px 12px; margin-bottom:8px; cursor:grab; box-shadow:0 1px 2px rgba(16,24,40,.05); transition:box-shadow .15s ease, transform .15s ease, border-color .15s ease; }
.card:hover { border-color:var(--accent); transform:translateY(-1px); box-shadow:0 4px 12px rgba(16,24,40,.08); }
.card.dragging { opacity:.5; }
.card .id { font-size:11px; color:var(--accent); font-weight:600; letter-spacing:.02em; }
.card .title { margin:2px 0; cursor:pointer; }
.card .desc { color:var(--meta); font-size:12px; margin:2px 0; }
.card .meta { font-size:11px; color:var(--meta); }
.card .badge { display:inline-block; border-radius:10px; padding:0 6px; margin-right:4px; font-size:11px; }
.badge.claim { background:var(--accent-soft); color:var(--accent); }
.badge.label { background:var(--amber); color:var(--amber-text); }
.card .comments { margin-top:6px; border-top:1px solid var(--border); padding-top:4px; }
.card .comment { font-size:12px; color:var(--meta); margin:2px 0; }
.card .comment .who { color:var(--text); font-weight:600; }
.card .comment .when { font-size:11px; }
.card .comment-form { display:flex; gap:4px; margin-top:6px; }
.card .comment-form input { flex:1; border:1px solid var(--border); border-radius:8px; padding:4px 8px; font:inherit; }
.card .comment-form button { border:1px solid var(--border); background:var(--card); border-radius:8px; padding:4px 10px; cursor:pointer; }
.card .comment-form button:hover { border-color:var(--accent); color:var(--accent); }
#panel { width:320px; flex:0 0 auto; background:var(--card); border:1px solid var(--border); border-radius:14px; padding:14px; position:sticky; top:20px; box-shadow:0 1px 2px rgba(16,24,40,.05); }
#panel h2 { font-size:14px; margin:0 0 8px; }
#changes { list-style:none; margin:0 0 8px; padding:0; }
#changes li { font-size:12px; padding:4px 0; border-bottom:1px solid var(--border); }
#changes li .msg { color:var(--meta); }
#diff { font:11px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace; background:var(--bg); border:1px solid var(--border); border-radius:8px; padding:8px; max-height:200px; overflow:auto; white-space:pre; }
#message { width:100%; margin:8px 0; border:1px solid var(--border); border-radius:8px; padding:6px; font:inherit; }
#commit { width:100%; background:var(--accent); color:#fff; border:0; border-radius:8px; padding:8px; font:inherit; cursor:pointer; transition:background .15s ease; }
#commit:hover { background:#4c58c4; }
#commit:disabled { background:#b9bfea; cursor:default; }
#status { font-size:12px; color:var(--meta); margin-top:8px; min-height:16px; }
#modal { display:none; position:fixed; inset:0; background:rgba(16,24,40,.45); z-index:50; padding:40px 16px; overflow-y:auto; }
#modal.open { display:block; }
.modal-box { background:var(--card); border-radius:14px; max-width:680px; margin:0 auto; box-shadow:0 24px 64px rgba(16,24,40,.25); }
.modal-head { display:flex; align-items:flex-start; gap:12px; padding:18px 22px; border-bottom:1px solid var(--border); }
.modal-head .ids { font-size:12px; color:var(--accent); font-weight:600; letter-spacing:.02em; }
.modal-head h2 { margin:2px 0 0; font-size:18px; letter-spacing:-.01em; }
.modal-close { margin-left:auto; border:0; background:var(--bg); border-radius:8px; width:28px; height:28px; cursor:pointer; font-size:15px; color:var(--meta); }
.modal-close:hover { background:var(--border); }
.modal-body { padding:18px 22px; }
.modal-body h3 { font-size:12px; text-transform:uppercase; letter-spacing:.06em; color:var(--meta); margin:18px 0 6px; }
.modal-body .desc { color:var(--text); margin:0 0 6px; }
.modal .links { display:flex; flex-wrap:wrap; gap:6px; }
.modal .link { font-size:12px; background:var(--bg); border:1px solid var(--border); border-radius:8px; padding:2px 8px; color:var(--meta); }
.modal .link b { color:var(--text); font-weight:600; }
.modal .comment { font-size:13px; margin:0 0 8px; }
.modal .comment .who { font-weight:600; }
.modal .comment .when { font-size:11px; color:var(--meta); margin-left:6px; }
.modal .comment-form { display:flex; gap:6px; margin-top:10px; }
.modal .comment-form input { flex:1; border:1px solid var(--border); border-radius:8px; padding:6px 10px; font:inherit; }
.modal .comment-form button { border:1px solid var(--accent); background:var(--accent); color:#fff; border-radius:8px; padding:6px 14px; cursor:pointer; }
.modal .comment-form button:hover { background:#4c58c4; }
.modal .history { font-size:12px; color:var(--meta); }
.modal .history .ev { padding:4px 0; border-bottom:1px solid var(--bg); }
.modal .history .ev .when { font-size:11px; margin-right:6px; }
.modal .history .ev b { color:var(--text); font-weight:600; }
</style>
</head>
<body>
<header><h1 id="name">hexdeck</h1><span class="updated" id="updated"></span></header>
<main>
<div id="board"></div>
<aside id="panel">
<h2>Changes</h2>
<ul id="changes"></ul>
<pre id="diff"></pre>
<input id="message" placeholder="commit message">
<button id="commit" disabled>Commit</button>
<div id="status"></div>
</aside>
</main>
<div id="modal"></div>
<script>
const state = { board:null, changes:[], message:"" };
const $ = id => document.getElementById(id);
async function api(path, body) {
  const res = await fetch(path, body === undefined ? {} : {
    method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify(body)
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.status);
  return data;
}
function esc(s) { return (s||"").replace(/[&<>"]/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c])); }
function cardHTML(t) {
  const badges = [];
  if (t.claimedBy) badges.push('<span class="badge claim">' + esc(t.claimedBy) + (t.claimStale ? " (stale)" : "") + '</span>');
  (t.labels || []).forEach(l => badges.push('<span class="badge label">' + esc(l) + '</span>'));
  return '<div class="card" draggable="true" data-id="' + esc(t.id) + '">' +
    '<div class="id">' + esc(t.id) + '</div>' +
    '<div class="title">' + esc(t.title) + '</div>' +
    (badges.length ? '<div class="meta">' + badges.join("") + '</div>' : "") +
    '</div>';
}
function modalHTML(t) {
  const badges = [];
  if (t.claimedBy) badges.push('<span class="badge claim">' + esc(t.claimedBy) + (t.claimStale ? " (stale)" : "") + '</span>');
  (t.labels || []).forEach(l => badges.push('<span class="badge label">' + esc(l) + '</span>'));
  const links = [];
  if (t.blocks && t.blocks.length) links.push('<span class="link"><b>blocks</b> ' + t.blocks.map(esc).join(", ") + '</span>');
  if (t.blockedBy && t.blockedBy.length) links.push('<span class="link"><b>blocked by</b> ' + t.blockedBy.map(esc).join(", ") + '</span>');
  if (t.related && t.related.length) links.push('<span class="link"><b>related</b> ' + t.related.map(esc).join(", ") + '</span>');
  const comments = (t.comments || []).map(c =>
    '<div class="comment"><span class="who">' + esc(c.actor) + '</span><span class="when">' + esc(c.ts) + '</span><br>' + esc(c.text) + '</div>'
  ).join("");
  return '<div class="modal-head"><div><div class="ids">' + esc(t.id) + '</div>' +
    '<h2>' + esc(t.title) + '</h2>' + (badges.length ? '<div class="meta">' + badges.join("") + '</div>' : "") + '</div>' +
    '<button class="modal-close" aria-label="Close">&times;</button></div>' +
    '<div class="modal-body">' +
    (t.description ? '<h3>Description</h3><p class="desc">' + esc(t.description) + '</p>' : "") +
    (links.length ? '<h3>Links</h3><div class="links">' + links.join("") + '</div>' : "") +
    (comments ? '<h3>Comments</h3><div class="comments">' + comments + '</div>' : "") +
    '<h3>History</h3><div class="history" id="history">…</div>' +
    '<form class="comment-form" data-id="' + esc(t.id) + '">' +
    '<input placeholder="Add a comment…"><button>Add</button></form>' +
    '</div>';
}
document.addEventListener("click", e => {
  const title = e.target.closest(".card .title");
  if (!title) return;
  const id = title.parentElement.dataset.id;
  const t = state.board.tickets[id];
  if (!t) return;
  $("modal").innerHTML = modalHTML(t);
  $("modal").classList.add("open");
  api("/api/history?ticket=" + encodeURIComponent(id)).then(h => {
    $("history").innerHTML = h.ops.map(op =>
      '<div class="ev"><span class="when">' + esc(op.ts) + '</span><b>' + esc(op.actor) + '</b> ' + esc(op.type) + '</div>'
    ).join("");
  }).catch(() => { $("history").textContent = "history unavailable"; });
});
document.addEventListener("click", e => {
  const close = e.target.closest(".modal-close");
  if (close) $("modal").classList.remove("open");
});
document.addEventListener("keydown", e => {
  if (e.key === "Escape" && $("modal").classList.contains("open")) $("modal").classList.remove("open");
});
function render() {
  const b = state.board;
  if (!b) return;
  $("name").textContent = b.name || "hexdeck";
  $("updated").textContent = "Updated: " + (b.updated || "");
  $("board").innerHTML = b.columns.map(col => {
    const tickets = Object.values(b.tickets || {}).filter(t => !t.archived && t.status === col)
      .sort((a, b2) => a.id.localeCompare(b2.id, undefined, {numeric:true}));
    const cards = tickets.map(cardHTML).join("");
    const body = cards || '<div class="cards empty">No tickets</div>';
    return '<div class="column" data-column="' + esc(col) + '"><h2>' + esc(col) +
      '<span class="count">' + tickets.length + '</span></h2>' +
      '<div class="cards' + (cards ? "" : " empty") + '">' + cards + '</div></div>';
  }).join("");
  $("changes").innerHTML = (state.changes || []).map(c =>
    '<li>' + esc(c.ticket) + ' ' + esc(c.type) + '<div class="msg">' + esc(c.message) + '</div></li>'
  ).join("");
  $("diff").textContent = state.diff || "";
  $("message").value = state.message || "";
  $("commit").disabled = state.changes.length === 0;
}
async function refresh() {
  state.board = await api("/api/state");
  const panel = await api("/api/changes");
  state.changes = panel.changes; state.diff = panel.diff; state.message = panel.message;
  render();
}
async function act(path, body) {
  try {
    const data = await api(path, body);
    state.board = data.state;
    const panel = await api("/api/changes");
    state.changes = panel.changes; state.diff = panel.diff; state.message = panel.message;
    render();
    $("status").textContent = "staged — " + data.message;
  } catch (e) { $("status").textContent = e.message; }
}
document.addEventListener("dragstart", e => {
  const card = e.target.closest(".card");
  if (!card) return;
  e.dataTransfer.setData("text/plain", card.dataset.id);
  card.classList.add("dragging");
});
document.addEventListener("dragend", e => {
  const card = e.target.closest(".card");
  if (card) card.classList.remove("dragging");
});
document.addEventListener("dragover", e => {
  const col = e.target.closest(".column");
  if (!col) return;
  e.preventDefault();
  col.classList.add("dragover");
});
document.addEventListener("dragleave", e => {
  const col = e.target.closest(".column");
  if (col && !col.contains(e.relatedTarget)) col.classList.remove("dragover");
});
document.addEventListener("drop", e => {
  const col = e.target.closest(".column");
  if (!col) return;
  e.preventDefault();
  col.classList.remove("dragover");
  const id = e.dataTransfer.getData("text/plain");
  if (id) act("/api/move", {ticket:id, to:col.dataset.column});
});
document.addEventListener("submit", e => {
  const form = e.target.closest(".comment-form");
  if (!form) return;
  e.preventDefault();
  const input = form.querySelector("input");
  const text = input.value.trim();
  if (!text) return;
  input.value = "";
  act("/api/comment", {ticket:form.dataset.id, text});
});
$("commit").addEventListener("click", async () => {
  try {
    const data = await api("/api/commit", {message:$("message").value.trim()});
    state.changes = []; state.diff = ""; state.message = "";
    render();
    $("status").textContent = "committed — " + data.message;
  } catch (e) { $("status").textContent = e.message; }
});
refresh().catch(e => { $("status").textContent = e.message; });
</script>
</body>
</html>
`)

// webChange is one change the web server made. The changes panel shows
// these until the user commits.
type webChange struct {
	Type    string `json:"type"`
	Ticket  string `json:"ticket"`
	Message string `json:"message"`
}

// webServer serves the web view: the page, the board state, and the
// write endpoints. Every write goes through the same path as the CLI —
// AppendOp, RenderAll, git staging — so the web view and the CLI can
// never disagree about the board.
type webServer struct {
	boardDir string
	repoDir  string
	actor    string
	noPull   bool
	// mu guards changes: HTTP handlers run in separate goroutines, and
	// a drag followed by a commit can land concurrently. The board
	// writes themselves stay serialized by git's lock; this mutex
	// keeps the panel bookkeeping race-free.
	mu      sync.Mutex
	changes []webChange
}

// newWebServer builds a web server over a board dir. The repo dir is
// the board's parent. The actor is the writer name for every op.
func newWebServer(boardDir, actor string, noPull bool) *webServer {
	return &webServer{
		boardDir: boardDir,
		repoDir:  findRepoRoot(boardDir),
		actor:    actor,
		noPull:   noPull,
	}
}

// mux builds the routes.
func (s *webServer) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePage)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/move", s.handleMove)
	mux.HandleFunc("/api/comment", s.handleComment)
	mux.HandleFunc("/api/changes", s.handleChanges)
	mux.HandleFunc("/api/commit", s.handleCommit)
	return mux
}

// handlePage serves the web view.
func (s *webServer) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(webPage)
}

// handleState serves the projection.
func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, state)
}

// handleHistory serves the op history for one ticket, newest first —
// the modal's history feed. The ticket must exist.
func (s *webServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("ticket is required"))
		return
	}
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, ok := state.Tickets[ticket]; !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("ticket %s does not exist", ticket))
		return
	}
	ops, warnings, err := hexdeck.ReadOpsDir(filepath.Join(s.boardDir, "ops"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var feed []hexdeck.Op
	for _, op := range ops {
		if op.Ticket == ticket {
			feed = append(feed, op)
		}
	}
	// Newest first — the modal shows the latest activity on top.
	for i, j := 0, len(feed)-1; i < j; i, j = i+1, j-1 {
		feed[i], feed[j] = feed[j], feed[i]
	}
	writeJSON(w, struct {
		Ops      []hexdeck.Op `json:"ops"`
		Warnings []string     `json:"warnings"`
	}{Ops: feed, Warnings: warnings})
}

// handleMove moves a ticket. The body is {"ticket": "T-1", "to":
// "todo"}.
func (s *webServer) handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("move takes POST"))
		return
	}
	var body struct {
		Ticket string `json:"ticket"`
		To     string `json:"to"`
	}
	if err := readBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ticket, ok := state.Tickets[body.Ticket]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("ticket %s does not exist", body.Ticket))
		return
	}
	if !contains(state.Columns, body.To) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("column %q does not exist — columns: %s", body.To, strings.Join(state.Columns, ", ")))
		return
	}
	payload, err := json.Marshal(hexdeck.TicketMovedPayload{
		From: ticket.Status,
		To:   body.To,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := fmt.Sprintf("board: move %s → %s", body.Ticket, body.To)
	if err := s.append(hexdeck.Op{
		Type:    hexdeck.OpTicketMoved,
		Ticket:  body.Ticket,
		Actor:   s.actor,
		Payload: payload,
	}, message); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeResult(w, message)
}

// handleComment adds a comment. The body is {"ticket": "T-1", "text":
// "on it"}.
func (s *webServer) handleComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("comment takes POST"))
		return
	}
	var body struct {
		Ticket string `json:"ticket"`
		Text   string `json:"text"`
	}
	if err := readBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Text == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("text is required"))
		return
	}
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, ok := state.Tickets[body.Ticket]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("ticket %s does not exist", body.Ticket))
		return
	}
	payload, err := json.Marshal(hexdeck.CommentAddedPayload{Text: body.Text})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := fmt.Sprintf("board: comment on %s", body.Ticket)
	if err := s.append(hexdeck.Op{
		Type:    hexdeck.OpCommentAdded,
		Ticket:  body.Ticket,
		Actor:   s.actor,
		Payload: payload,
	}, message); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeResult(w, message)
}

// handleChanges serves the changes panel: the list of changes, the
// staged diff, and the suggested commit message (the last change's).
func (s *webServer) handleChanges(w http.ResponseWriter, r *http.Request) {
	diff, err := s.stagedDiff()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.mu.Lock()
	message := ""
	if len(s.changes) > 0 {
		message = s.changes[len(s.changes)-1].Message
	}
	changes := s.changes
	if changes == nil {
		changes = []webChange{}
	}
	s.mu.Unlock()
	writeJSON(w, struct {
		Changes []webChange `json:"changes"`
		Diff    string      `json:"diff"`
		Message string      `json:"message"`
	}{changes, diff, message})
}

// handleCommit commits the staged changes. The optional body is
// {"message": "..."} — the message the user edited in the panel. The
// default is the suggested message.
func (s *webServer) handleCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("commit takes POST"))
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	// The body is optional — an empty body means the suggested message.
	// The decoder runs unconditionally (a chunked body has no
	// ContentLength), but an empty body is a clean EOF, not an error.
	readErr := readBody(r, &body)
	if readErr != nil && !strings.Contains(readErr.Error(), "EOF") {
		writeError(w, http.StatusBadRequest, readErr)
		return
	}
	s.mu.Lock()
	if len(s.changes) == 0 {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, fmt.Errorf("nothing to commit"))
		return
	}
	message := body.Message
	if message == "" {
		message = s.changes[len(s.changes)-1].Message
	}
	s.mu.Unlock()
	rel, err := filepath.Rel(s.repoDir, s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	paths, err := stagedBoardPaths(s.repoDir, rel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(paths) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("nothing staged to commit"))
		return
	}
	// Commit only the board's staged files — never whatever else the
	// user has staged in the repo.
	cmd := exec.Command("git", append([]string{"commit", "-m", message, "--"}, paths...)...)
	cmd.Dir = s.repoDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("git commit failed: %w", err))
		return
	}
	s.mu.Lock()
	s.changes = nil
	s.mu.Unlock()
	writeJSON(w, struct {
		Committed bool   `json:"committed"`
		Message   string `json:"message"`
	}{true, message})
}

// append writes the op, re-renders the board, stages the change, and
// records it in the changes panel. The write itself goes through the
// shared writeOp path — the same one the CLI uses. The whole sequence
// runs under the mutex: HTTP handlers run in separate goroutines, and
// serializing the write prevents two quick drags from interleaving
// their pull/append/render stages or racing the seq computation.
func (s *webServer) append(op hexdeck.Op, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := writeOp(s.boardDir, s.repoDir, s.noPull, op); err != nil {
		return err
	}
	s.changes = append(s.changes, webChange{Type: string(op.Type), Ticket: op.Ticket, Message: message})
	return nil
}

// writeResult responds to a write: the fresh state and the suggested
// commit message.
func (s *webServer) writeResult(w http.ResponseWriter, message string) {
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, struct {
		State   hexdeck.BoardState `json:"state"`
		Message string             `json:"message"`
	}{state, message})
}

// stagedDiff returns the staged diff of the board dir, or an empty
// string when the board is not in a git repo.
func (s *webServer) stagedDiff() (string, error) {
	if _, err := os.Stat(filepath.Join(s.repoDir, ".git")); err != nil {
		return "", nil
	}
	rel, err := filepath.Rel(s.repoDir, s.boardDir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "diff", "--cached", "--", rel)
	cmd.Dir = s.repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(out), nil
}

// readBody decodes a JSON request body, capped at 1 MiB so an
// oversized body cannot be an easy DoS even on localhost.
func readBody(r *http.Request, v any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// writeJSON writes v as an indented JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(append(data, '\n'))
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{err.Error()})
	w.Write(append(data, '\n'))
}

// runWeb serves the local web view. It prints the URL and blocks until
// the server stops. The port defaults to 8080.
func runWeb(args []string) error {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	port := fs.Int("port", 8080, "port to listen on")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("web takes no positional arguments")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf, filepath.Dir(boardDir))
	if err != nil {
		return err
	}
	server := newWebServer(boardDir, actor, cf.noPull)
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Printf("hexdeck web: http://%s\n", addr)
	fmt.Printf("board: %s\n", boardDir)
	return http.ListenAndServe(addr, server.mux())
}
