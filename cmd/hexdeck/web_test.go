package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danmurf/hexdeck"
)

// update refreshes the golden files when set.
var update = flag.Bool("update", false, "update golden files")

// newWebTestServer builds a web server over a fresh temp repo with a
// board holding two tickets.
func newWebTestServer(t *testing.T) (*webServer, string) {
	t.Helper()
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "Two", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	return newWebServer(filepath.Join(dir, ".kanban"), "claude-a", true), dir
}

// doJSON sends a request to the web server and returns the recorder.
func doJSON(t *testing.T, s *webServer, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	return rec
}

// decodeJSON decodes the recorder body into v and fails the test on
// error.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
}

// freePort returns a free TCP port on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// TestWebPageGolden pins the embedded web page byte for byte. The page
// is a render — the web view of the board — so it gets the same golden
// treatment as board.md and board.svg.
func TestWebPageGolden(t *testing.T) {
	golden := filepath.Join("testdata", "golden", "web.html")
	if *update {
		if err := os.WriteFile(golden, webPage, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(webPage, want) {
		t.Errorf("web page drifted from the golden file — run with -update to refresh")
	}
}

// TestWebPageSaaSDesign locks the T-16 design language: the page is a
// modern SaaS board, Linear-flavoured — a light canvas, the brand
// indigo accent, cards with soft shadows and hover-lift transitions,
// column count pills, empty-column placeholders, and dragover
// feedback. The prototype palette and the underline-link title are
// gone.
func TestWebPageSaaSDesign(t *testing.T) {
	page := string(webPage)
	for _, want := range []string{
		"--bg:#f7f8f8",     // light canvas
		"--accent:#5e6ad2", // brand indigo (Linear)
		".card:hover",      // hover state
		"box-shadow",       // card shadows
		"transition:",      // smooth hover transitions
		`class="count"`,    // column count pill
		"cards.empty",      // empty-column placeholder
		"No tickets",       // the placeholder hint
		"dragover",         // drag feedback on columns
	} {
		if !strings.Contains(page, want) {
			t.Errorf("web page lost the SaaS design token %q — the polish did not land", want)
		}
	}
	for _, banned := range []string{
		"#24292f", // dark header bar
		"#eaeef2", // grey column
		"#0969da", // github blue accent
		"#ddf4ff", // claim tint
		"text-decoration:underline dotted",
	} {
		if strings.Contains(page, banned) {
			t.Errorf("web page still carries the prototype token %q — the design language did not change", banned)
		}
	}
}

// TestWebPageNoExternalAssets pins the no-network property: the page is
// one self-contained HTML file — no stylesheets, no font CDNs, no
// remote fetch. The polish must never cost the offline single-file
// property.
func TestWebPageNoExternalAssets(t *testing.T) {
	page := string(webPage)
	for _, banned := range []string{"http://", "https://", "@import", "<link", "url("} {
		if strings.Contains(page, banned) {
			t.Errorf("web page references an external asset (%q) — the page must stay self-contained", banned)
		}
	}
}

// TestWebPageNoCommentBadge checks the T-12 contract: the board pages
// carry no comment-count badge — comments live on the ticket view, and
// the page still renders them in the expanded card detail.
func TestWebPageNoCommentBadge(t *testing.T) {
	page := string(webPage)
	for _, banned := range []string{
		`badge comment`,
		`' comment'`,
		`comments.length + ' comment'`,
		`badge.comment`,
	} {
		if strings.Contains(page, banned) {
			t.Errorf("web page still has the comment badge (%q) — comments live on the ticket view", banned)
		}
	}
	if !strings.Contains(page, `class="comments"`) {
		t.Errorf("web page lost the expanded comment detail — comments belong to the ticket view")
	}
}

// TestWebCommentFormOnlyInModal checks the T-18 contract for the web
// page: the Add-comment form lives in the ticket modal, never on a
// board card. A collapsed card is id, title and badges only — the
// modal is the ticket view (description, links, comments, history).
func TestWebCommentFormOnlyInModal(t *testing.T) {
	page := string(webPage)
	modal := strings.Index(page, `id="modal"`)
	form := strings.Index(page, `class="comment-form"`)
	if modal == -1 {
		t.Fatalf("web page lost the ticket modal")
	}
	if form == -1 {
		t.Fatalf("web page lost the comment form")
	}
	if !(modal < form) {
		t.Errorf("comment form sits outside the modal — the board still offers comments on cards")
	}
	// The card template itself must not carry the form: between
	// cardHTML and the modal opener there is no comment-form.
	cardFn := strings.Index(page, "function cardHTML")
	modalFn := strings.Index(page, "function modalHTML")
	if cardFn == -1 || modalFn == -1 {
		t.Fatalf("web page lost cardHTML or modalHTML")
	}
	if strings.Contains(page[cardFn:modalFn], "comment-form") {
		t.Errorf("cardHTML still renders the comment form — collapsed cards must be id, title, badges only")
	}
}

// TestWebHistory checks GET /api/history?ticket=T-1 returns the ops
// for that ticket, newest first — the modal's history feed.
func TestWebHistory(t *testing.T) {
	s, _ := newWebTestServer(t)
	// Move T-1 so the ticket has more than one op.
	rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "todo"})
	if rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, s, "GET", "/api/history?ticket=T-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history: status %d\n%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Ops []hexdeck.Op `json:"ops"`
	}
	decodeJSON(t, rec, &out)
	if len(out.Ops) < 2 {
		t.Fatalf("history ops = %d, want at least 2 (created + move)", len(out.Ops))
	}
	if out.Ops[0].Seq < out.Ops[len(out.Ops)-1].Seq {
		t.Errorf("history not newest-first: seq %d before %d", out.Ops[0].Seq, out.Ops[len(out.Ops)-1].Seq)
	}
	for _, op := range out.Ops {
		if op.Ticket != "T-1" {
			t.Errorf("history includes op for %s, want only T-1", op.Ticket)
		}
	}
	// Unknown ticket is an error, not an empty 200.
	rec = doJSON(t, s, "GET", "/api/history?ticket=T-99", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("history for unknown ticket: status %d, want 404", rec.Code)
	}
}

// TestWebState checks GET /api/state returns the projection.
func TestWebState(t *testing.T) {
	s, _ := newWebTestServer(t)
	rec := doJSON(t, s, "GET", "/api/state", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("state: status %d\n%s", rec.Code, rec.Body.String())
	}
	var state hexdeck.BoardState
	decodeJSON(t, rec, &state)
	if len(state.Tickets) != 2 {
		t.Errorf("tickets = %d, want 2", len(state.Tickets))
	}
	if state.Tickets["T-1"].Status != "backlog" {
		t.Errorf("T-1 status = %q, want backlog — new tickets start there", state.Tickets["T-1"].Status)
	}
}

// TestWebMove checks POST /api/move: the op lands, the board files
// re-render, the change is staged, and the response carries the fresh
// state and the suggested commit message.
func TestWebMove(t *testing.T) {
	s, dir := newWebTestServer(t)
	rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "todo"})
	if rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		State   hexdeck.BoardState `json:"state"`
		Message string             `json:"message"`
	}
	decodeJSON(t, rec, &resp)
	if resp.State.Tickets["T-1"].Status != "todo" {
		t.Errorf("T-1 status = %q, want todo", resp.State.Tickets["T-1"].Status)
	}
	if resp.Message != "board: move T-1 → todo" {
		t.Errorf("message = %q, want the suggested commit message", resp.Message)
	}
	// The op landed on disk and the projection agrees.
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Tickets["T-1"].Status != "todo" {
		t.Errorf("projected T-1 status = %q, want todo", state.Tickets["T-1"].Status)
	}
	// The op and the board files are staged — the same-commit rule.
	out := runGitOut(t, dir, "status", "--porcelain")
	for _, want := range []string{".kanban/ops/", ".kanban/board.md", ".kanban/board.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("staged files missing %s:\n%s", want, out)
		}
	}
}

// TestWebComment checks POST /api/comment: the comment lands on the
// ticket and the response carries the fresh state.
func TestWebComment(t *testing.T) {
	s, dir := newWebTestServer(t)
	rec := doJSON(t, s, "POST", "/api/comment", map[string]string{"ticket": "T-1", "text": "on it"})
	if rec.Code != http.StatusOK {
		t.Fatalf("comment: status %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		State   hexdeck.BoardState `json:"state"`
		Message string             `json:"message"`
	}
	decodeJSON(t, rec, &resp)
	comments := resp.State.Tickets["T-1"].Comments
	if len(comments) != 1 || comments[0].Text != "on it" {
		t.Errorf("T-1 comments = %v, want one comment", comments)
	}
	if comments[0].Actor != "claude-a" {
		t.Errorf("comment actor = %q, want claude-a", comments[0].Actor)
	}
	if resp.Message != "board: comment on T-1" {
		t.Errorf("message = %q, want the suggested commit message", resp.Message)
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(state.Tickets["T-1"].Comments) != 1 {
		t.Errorf("projected T-1 comments = %d, want 1", len(state.Tickets["T-1"].Comments))
	}
}

// TestWebChanges checks GET /api/changes: it lists every change the
// server made, the staged diff, and the suggested commit message (the
// last change's message).
func TestWebChanges(t *testing.T) {
	s, _ := newWebTestServer(t)
	if rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "todo"}); rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, s, "POST", "/api/comment", map[string]string{"ticket": "T-2", "text": "later"}); rec.Code != http.StatusOK {
		t.Fatalf("comment: status %d\n%s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, s, "GET", "/api/changes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("changes: status %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Changes []struct {
			Type    string `json:"type"`
			Ticket  string `json:"ticket"`
			Message string `json:"message"`
		} `json:"changes"`
		Diff    string `json:"diff"`
		Message string `json:"message"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Changes) != 2 {
		t.Fatalf("changes = %d, want 2: %v", len(resp.Changes), resp.Changes)
	}
	if resp.Changes[0].Type != "ticket.moved" || resp.Changes[0].Ticket != "T-1" {
		t.Errorf("first change = %+v, want the move", resp.Changes[0])
	}
	if resp.Changes[1].Type != "comment.added" || resp.Changes[1].Ticket != "T-2" {
		t.Errorf("second change = %+v, want the comment", resp.Changes[1])
	}
	if resp.Message != "board: comment on T-2" {
		t.Errorf("suggested message = %q, want the last change's message", resp.Message)
	}
	if !strings.Contains(resp.Diff, ".kanban/ops/") {
		t.Errorf("diff does not show the staged ops:\n%s", resp.Diff)
	}
}

// TestWebChangesEmpty checks GET /api/changes on a fresh server: the
// changes list is an empty array, not null. A null list crashes the
// page's render loop (state.changes.map) — the panel must stay usable
// before anything is staged.
func TestWebChangesEmpty(t *testing.T) {
	s, _ := newWebTestServer(t)
	rec := doJSON(t, s, "GET", "/api/changes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("changes: status %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Changes []webChange `json:"changes"`
	}
	decodeJSON(t, rec, &resp)
	if resp.Changes == nil {
		t.Fatal("changes is null, want an empty array — the page crashes on a null list")
	}
	if len(resp.Changes) != 0 {
		t.Fatalf("changes = %d, want 0 on a fresh server", len(resp.Changes))
	}
}

// TestWebCommit checks POST /api/commit: the staged changes land in a
// commit with the suggested message, the working tree is clean, and
// the changes list empties.
func TestWebCommit(t *testing.T) {
	s, dir := newWebTestServer(t)
	if rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "todo"}); rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, s, "POST", "/api/commit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: status %d\n%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Committed bool   `json:"committed"`
		Message   string `json:"message"`
	}
	decodeJSON(t, rec, &resp)
	if !resp.Committed {
		t.Error("committed = false, want true")
	}
	out := runGitOut(t, dir, "log", "--oneline", "-1")
	if !strings.Contains(out, "board: move T-1 → todo") {
		t.Errorf("last commit = %q, want the suggested message", out)
	}
	out = runGitOut(t, dir, "status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		t.Errorf("working tree not clean after commit:\n%s", out)
	}
	rec = doJSON(t, s, "GET", "/api/changes", nil)
	var changes struct {
		Changes []json.RawMessage `json:"changes"`
	}
	decodeJSON(t, rec, &changes)
	if len(changes.Changes) != 0 {
		t.Errorf("changes after commit = %d, want 0", len(changes.Changes))
	}
}

// TestWebCommitCustomMessage checks the commit uses the message the
// user edited in the panel when one is sent.
func TestWebCommitCustomMessage(t *testing.T) {
	s, dir := newWebTestServer(t)
	if rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "done"}); rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, s, "POST", "/api/commit", map[string]string{"message": "board: T-1 is done"})
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: status %d\n%s", rec.Code, rec.Body.String())
	}
	out := runGitOut(t, dir, "log", "--oneline", "-1")
	if !strings.Contains(out, "board: T-1 is done") {
		t.Errorf("last commit = %q, want the edited message", out)
	}
}

// TestWebErrors checks the error paths: unknown ticket, unknown column,
// empty comment, bad JSON, and commit with nothing staged.
func TestWebErrors(t *testing.T) {
	s, _ := newWebTestServer(t)
	cases := []struct {
		path string
		body any
		want string
	}{
		{"/api/move", map[string]string{"ticket": "T-99", "to": "done"}, "does not exist"},
		{"/api/move", map[string]string{"ticket": "T-1", "to": "nope"}, "column"},
		{"/api/comment", map[string]string{"ticket": "T-99", "text": "hi"}, "does not exist"},
		{"/api/comment", map[string]string{"ticket": "T-1", "text": ""}, "text"},
	}
	for _, c := range cases {
		rec := doJSON(t, s, "POST", c.path, c.body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %v: status %d, want 400\n%s", c.path, c.body, rec.Code, rec.Body.String())
			continue
		}
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s %v: error %q does not mention %q", c.path, c.body, rec.Body.String(), c.want)
		}
	}
	// Bad JSON is a 400, not a crash.
	req := httptest.NewRequest("POST", "/api/move", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: status %d, want 400", rec.Code)
	}
	// Commit with nothing staged is a 400.
	rec = doJSON(t, s, "POST", "/api/commit", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty commit: status %d, want 400\n%s", rec.Code, rec.Body.String())
	}
}

// TestWebMethodNotAllowed checks the write endpoints reject non-POST
// methods with a 405 instead of a confusing parse error.
func TestWebMethodNotAllowed(t *testing.T) {
	s, _ := newWebTestServer(t)
	for _, path := range []string{"/api/move", "/api/comment", "/api/commit"} {
		rec := doJSON(t, s, "GET", path, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status %d, want 405\n%s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestWebCommitLeavesForeignStaged checks that the web commit commits
// only the board's files — a file the user staged for something else
// must stay staged and uncommitted.
func TestWebCommitLeavesForeignStaged(t *testing.T) {
	s, dir := newWebTestServer(t)
	// Stage a foreign file: not part of the board.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("user work\n"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	runGitOut(t, dir, "add", "notes.txt")
	// Move through the web server so the changes panel is populated.
	if rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": "T-1", "to": "todo"}); rec.Code != http.StatusOK {
		t.Fatalf("move: status %d\n%s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, s, "POST", "/api/commit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: status %d\n%s", rec.Code, rec.Body.String())
	}
	status := runGitOut(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "notes.txt") {
		t.Errorf("foreign staged file was committed:\n%s", status)
	}
	log := runGitOut(t, dir, "log", "--oneline", "-1", "--name-only")
	if strings.Contains(log, "notes.txt") {
		t.Errorf("foreign file landed in the board commit:\n%s", log)
	}
}

// TestWebConcurrentWrites fires several write requests at once and
// checks the server serializes them: every change lands on disk, the
// changes panel shows exactly the written changes, and the board
// reflects all of them. Run under -race this exercises the mutex.
func TestWebConcurrentWrites(t *testing.T) {
	s, dir := newWebTestServer(t)
	const n = 8
	// Create the tickets the concurrent moves will touch.
	for i := 1; i <= n; i++ {
		if out, code := runHexdeck(t, dir, "create", fmt.Sprintf("Ticket %d", i), "--as", "claude-a"); code != 0 {
			t.Fatalf("create %d: exit %d\n%s", i, code, out)
		}
	}
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := doJSON(t, s, "POST", "/api/move", map[string]string{"ticket": fmt.Sprintf("T-%d", i+1), "to": "todo"})
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("concurrent move %d: status %d", i+1, code)
		}
	}
	// The changes panel must list exactly the many moves.
	rec := doJSON(t, s, "GET", "/api/changes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("changes: status %d\n%s", rec.Code, rec.Body.String())
	}
	var panel struct {
		Changes []webChange `json:"changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &panel); err != nil {
		t.Fatalf("decode changes: %v", err)
	}
	if len(panel.Changes) != n {
		t.Errorf("changes = %d, want %d — a concurrent write was lost or duplicated", len(panel.Changes), n)
	}
	// The board must reflect all the moves.
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	moved := 0
	for _, ticket := range state.Tickets {
		if ticket.Status == "todo" {
			moved++
		}
	}
	if moved != n {
		t.Errorf("todo tickets = %d, want %d", moved, n)
	}
}

// TestWebBodyCap checks the 1 MiB body cap on the write endpoints: an
// oversized body is a 400 and no op is written.
func TestWebBodyCap(t *testing.T) {
	s, dir := newWebTestServer(t)
	big := strings.Repeat("x", 2<<20)
	rec := doJSON(t, s, "POST", "/api/comment", map[string]string{"ticket": "T-1", "text": big})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: status %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(state.Tickets["T-1"].Comments) != 0 {
		t.Errorf("oversized comment landed on the board — the body cap did not stop the write")
	}
}

// TestE2EWeb runs the real binary: it serves the page, and a move over
// HTTP lands as an op on disk. This proves the command wiring — port
// binding, actor resolution, and the write path.
func TestE2EWeb(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	port := freePort(t)
	bin := buildHexdeck(t)
	cmd := exec.Command(bin, "web", "--port", strconv.Itoa(port), "--no-pull", "--as", "claude-a")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start web: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	// Poll until the server answers.
	var lastErr error
	for i := 0; i < 50; i++ {
		resp, err := http.Get(base + "/api/state")
		if err == nil {
			resp.Body.Close()
			lastErr = nil
			break
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("server did not come up: %v", lastErr)
	}
	// The page is served at /.
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	page, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(page), "hexdeck") {
		t.Errorf("GET /: status %d, page does not look like the web view", resp.StatusCode)
	}
	// A move over HTTP lands as an op.
	data, _ := json.Marshal(map[string]string{"ticket": "T-1", "to": "todo"})
	resp, err = http.Post(base+"/api/move", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST /api/move: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/move: status %d", resp.StatusCode)
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Tickets["T-1"].Status != "todo" {
		t.Errorf("T-1 status = %q, want todo", state.Tickets["T-1"].Status)
	}
}
