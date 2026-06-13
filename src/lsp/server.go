// Package lsp implements a minimal stdio LSP server for ghat.
// It publishes diagnostics for GitHub Actions workflow files (static, instant)
// and provides on-demand "Audit dependency" code actions for all supported
// manifest kinds (go.mod, package.json, requirements.txt, Cargo.toml, Gemfile,
// cpanfile, .pre-commit-config.yaml, GHA workflows).
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jameswoolfenden/ghat/src/core"
)

// Server is a stateful stdio LSP server.
type Server struct {
	token string
	cache *core.Cache

	mu         sync.Mutex
	docs       map[string][]byte        // uri → current content
	deps       map[string][]core.DepRef // uri → dep refs from ParseManifest
	auditDiags map[string][]diagnostic  // uri → diagnostics appended by executeCommand
}

// New creates a Server that uses the given GitHub token (may be empty) and
// optional disk cache for API responses.
func New(token string, cache *core.Cache) *Server {
	return &Server{
		token:      token,
		cache:      cache,
		docs:       make(map[string][]byte),
		deps:       make(map[string][]core.DepRef),
		auditDiags: make(map[string][]diagnostic),
	}
}

// Run reads JSON-RPC messages from stdin and writes responses to stdout until
// stdin is closed or an unrecoverable I/O error occurs.
func (s *Server) Run() error {
	rd := bufio.NewReader(os.Stdin)
	wr := bufio.NewWriter(os.Stdout)
	for {
		msg, err := readMessage(rd)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := s.dispatch(wr, msg); err != nil {
			return err
		}
		_ = wr.Flush()
	}
}

// ---- JSON-RPC plumbing -------------------------------------------------------

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func readMessage(r *bufio.Reader) (rpcMsg, error) {
	contentLen := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return rpcMsg{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLen, _ = strconv.Atoi(v)
		}
	}
	if contentLen == 0 {
		return rpcMsg{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return rpcMsg{}, err
	}
	var msg rpcMsg
	return msg, json.Unmarshal(body, &msg)
}

func writeMessage(w io.Writer, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return err
}

func writeResult(w io.Writer, id json.RawMessage, result interface{}) error {
	if result == nil {
		result = json.RawMessage("null")
	}
	return writeMessage(w, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeNotification(w io.Writer, method string, params interface{}) error {
	return writeMessage(w, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

// ---- dispatch ---------------------------------------------------------------

func (s *Server) dispatch(w *bufio.Writer, msg rpcMsg) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(w, msg)
	case "initialized":
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(w, msg)
	case "textDocument/didChange":
		return s.handleDidChange(w, msg)
	case "textDocument/didClose":
		return s.handleDidClose(msg)
	case "textDocument/codeAction":
		return s.handleCodeAction(w, msg)
	case "workspace/executeCommand":
		return s.handleExecuteCommand(w, msg)
	case "shutdown":
		return writeResult(w, msg.ID, nil)
	case "exit":
		os.Exit(0)
	}
	return nil // unknown notifications are silently ignored
}

// ---- initialize -------------------------------------------------------------

func (s *Server) handleInitialize(w io.Writer, msg rpcMsg) error {
	return writeResult(w, msg.ID, map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync":   1, // full-document sync
			"codeActionProvider": true,
			"executeCommandProvider": map[string]interface{}{
				"commands": []string{"ghat.audit", "ghat.auditFile"},
			},
		},
		"serverInfo": map[string]string{"name": "ghat-lsp", "version": "0.1"},
	})
}

// ---- didOpen ----------------------------------------------------------------

type didOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

func (s *Server) handleDidOpen(w io.Writer, msg rpcMsg) error {
	var p didOpenParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	content := []byte(p.TextDocument.Text)
	s.mu.Lock()
	s.docs[p.TextDocument.URI] = content
	s.mu.Unlock()
	return s.analyze(w, p.TextDocument.URI, content)
}

// ---- didChange --------------------------------------------------------------

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

func (s *Server) handleDidChange(w io.Writer, msg rpcMsg) error {
	var p didChangeParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	if len(p.ContentChanges) == 0 {
		return nil
	}
	content := []byte(p.ContentChanges[len(p.ContentChanges)-1].Text)
	s.mu.Lock()
	s.docs[p.TextDocument.URI] = content
	delete(s.auditDiags, p.TextDocument.URI) // clear stale audit results on edit
	s.mu.Unlock()
	return s.analyze(w, p.TextDocument.URI, content)
}

// ---- didClose ---------------------------------------------------------------

func (s *Server) handleDidClose(msg rpcMsg) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.docs, p.TextDocument.URI)
	delete(s.deps, p.TextDocument.URI)
	delete(s.auditDiags, p.TextDocument.URI)
	s.mu.Unlock()
	return nil
}

// ---- static analysis & dep caching -----------------------------------------

type diagnostic struct {
	Range    diagRange `json:"range"`
	Severity int       `json:"severity"` // 1=Error 2=Warning 3=Info 4=Hint
	Message  string    `json:"message"`
	Source   string    `json:"source"`
}

type diagRange struct {
	Start diagPos `json:"start"`
	End   diagPos `json:"end"`
}

type diagPos struct {
	Line      int `json:"line"`      // 0-indexed
	Character int `json:"character"` // 0-indexed
}

func (s *Server) analyze(w io.Writer, uri string, content []byte) error {
	kind, ok := classifyURI(uri)
	if !ok {
		return s.publishDiags(w, uri, nil)
	}

	refs := core.ParseManifest(kind, content)
	s.mu.Lock()
	s.deps[uri] = refs
	s.mu.Unlock()

	// Static diagnostics only for GHA workflow files.
	if kind != core.ManifestGHA {
		return s.publishDiags(w, uri, nil)
	}
	return s.publishDiags(w, uri, ghaStaticDiags(uriToPath(uri), content))
}

// ghaStaticDiags runs AnalyzeWorkflow and converts findings to LSP diagnostics.
func ghaStaticDiags(filename string, content []byte) []diagnostic {
	wa := core.AnalyzeWorkflow(filename, content)
	var diags []diagnostic

	if !wa.HasPermissions {
		diags = append(diags, fileDiag(2, "workflow missing top-level permissions: block (default GITHUB_TOKEN is write-all)"))
	} else if wa.IsWriteAll {
		diags = append(diags, fileDiag(1, "permissions: write-all grants the GITHUB_TOKEN full repository write access"))
	}
	if wa.HasDangerousTrigger {
		diags = append(diags, fileDiag(1, wa.DangerousTriggerDesc))
	}
	for _, step := range wa.Steps {
		if step.Suppressed || step.IsSHAPinned {
			continue
		}
		ref := step.Action
		if step.Ref != "" {
			ref = step.Action + "@" + step.Ref
		}
		line := findUsesLine(content, step.Action)
		diags = append(diags, lineDiag(line, 2, ref+" is not pinned to an immutable SHA"))
	}
	return diags
}

func (s *Server) publishDiags(w io.Writer, uri string, staticDiags []diagnostic) error {
	s.mu.Lock()
	audit := s.auditDiags[uri]
	s.mu.Unlock()

	all := append(staticDiags, audit...) //nolint:gocritic
	if all == nil {
		all = []diagnostic{}
	}
	return writeNotification(w, "textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": all,
	})
}

// ---- codeAction -------------------------------------------------------------

type codeActionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Range diagRange `json:"range"`
}

type codeAction struct {
	Title   string      `json:"title"`
	Kind    string      `json:"kind"`
	Command *lspCommand `json:"command,omitempty"`
}

type lspCommand struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments"`
}

func (s *Server) handleCodeAction(w io.Writer, msg rpcMsg) error {
	var p codeActionParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return writeResult(w, msg.ID, []codeAction{})
	}

	s.mu.Lock()
	refs := s.deps[p.TextDocument.URI]
	s.mu.Unlock()

	cursorLine := p.Range.Start.Line + 1 // 0-indexed → 1-indexed

	var actions []codeAction
	for _, ref := range refs {
		if ref.Line == cursorLine {
			title := "Audit " + ref.Name
			actions = append(actions, codeAction{
				Title: title,
				Kind:  "source",
				Command: &lspCommand{
					Title:     title,
					Command:   "ghat.audit",
					Arguments: []interface{}{p.TextDocument.URI, ref.Line},
				},
			})
		}
	}
	if len(refs) > 0 {
		actions = append(actions, codeAction{
			Title: "Audit all dependencies in this file",
			Kind:  "source",
			Command: &lspCommand{
				Title:     "Audit all dependencies in this file",
				Command:   "ghat.auditFile",
				Arguments: []interface{}{p.TextDocument.URI},
			},
		})
	}
	return writeResult(w, msg.ID, actions)
}

// ---- executeCommand ---------------------------------------------------------

func (s *Server) handleExecuteCommand(w io.Writer, msg rpcMsg) error {
	var p struct {
		Command   string          `json:"command"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return writeResult(w, msg.ID, nil)
	}
	switch p.Command {
	case "ghat.audit":
		return s.execAuditOne(w, msg.ID, p.Arguments)
	case "ghat.auditFile":
		return s.execAuditFile(w, msg.ID, p.Arguments)
	}
	return writeResult(w, msg.ID, nil)
}

func (s *Server) execAuditOne(w io.Writer, id json.RawMessage, args json.RawMessage) error {
	var argv []interface{}
	if err := json.Unmarshal(args, &argv); err != nil || len(argv) < 2 {
		return writeResult(w, id, nil)
	}
	uri, _ := argv[0].(string)
	lineFloat, _ := argv[1].(float64)
	targetLine := int(lineFloat)

	s.mu.Lock()
	refs := s.deps[uri]
	s.mu.Unlock()

	var ref *core.DepRef
	for i := range refs {
		if refs[i].Line == targetLine {
			ref = &refs[i]
			break
		}
	}
	if ref == nil {
		return writeResult(w, id, nil)
	}

	// Acknowledge the command immediately; diagnostics follow asynchronously.
	_ = writeResult(w, id, nil)

	score, err := core.AuditOne(ref.Ecosystem, ref.Name, ref.Version, s.token, s.cache)
	var diag diagnostic
	if err != nil {
		diag = lineDiag(ref.Line, 2, fmt.Sprintf("audit %s: %v", ref.Name, err))
	} else {
		diag = scoreToDiag(score, ref.Name, ref.Line)
	}

	s.mu.Lock()
	s.auditDiags[uri] = append(s.auditDiags[uri], diag)
	s.mu.Unlock()
	return s.refreshDiags(w, uri)
}

func (s *Server) execAuditFile(w io.Writer, id json.RawMessage, args json.RawMessage) error {
	var argv []interface{}
	if err := json.Unmarshal(args, &argv); err != nil || len(argv) < 1 {
		return writeResult(w, id, nil)
	}
	uri, _ := argv[0].(string)

	s.mu.Lock()
	refs := s.deps[uri]
	s.mu.Unlock()

	_ = writeResult(w, id, nil)

	var newDiags []diagnostic
	for _, ref := range refs {
		score, err := core.AuditOne(ref.Ecosystem, ref.Name, ref.Version, s.token, s.cache)
		if err != nil {
			newDiags = append(newDiags, lineDiag(ref.Line, 2, fmt.Sprintf("audit %s: %v", ref.Name, err)))
			continue
		}
		newDiags = append(newDiags, scoreToDiag(score, ref.Name, ref.Line))
	}

	s.mu.Lock()
	s.auditDiags[uri] = newDiags
	s.mu.Unlock()
	return s.refreshDiags(w, uri)
}

func (s *Server) refreshDiags(w io.Writer, uri string) error {
	s.mu.Lock()
	content := s.docs[uri]
	s.mu.Unlock()

	var staticDiags []diagnostic
	if kind, ok := classifyURI(uri); ok && kind == core.ManifestGHA && content != nil {
		staticDiags = ghaStaticDiags(uriToPath(uri), content)
	}
	return s.publishDiags(w, uri, staticDiags)
}

// ---- helpers ----------------------------------------------------------------

func scoreToDiag(score core.AuditScore, name string, line int) diagnostic {
	sev := 3 // Info — bucket "ok"
	switch score.Bucket {
	case "RISK":
		sev = 1 // Error
	case "STALE":
		sev = 2 // Warning
	}
	return lineDiag(line, sev, fmt.Sprintf("audit %s [%s]%s", name, score.Bucket, formatChecks(score.Checks)))
}

func formatChecks(checks []core.Check) string {
	var b strings.Builder
	for _, c := range checks {
		switch c.Outcome {
		case core.CheckPass:
			fmt.Fprintf(&b, "  ✓ %s", c.Name)
		case core.CheckFail:
			if c.Detail != "" {
				fmt.Fprintf(&b, "  ✗ %s(%s)", c.Name, c.Detail)
			} else {
				fmt.Fprintf(&b, "  ✗ %s", c.Name)
			}
		}
	}
	return b.String()
}

func fileDiag(sev int, msg string) diagnostic {
	return lineDiag(1, sev, msg)
}

func lineDiag(line, sev int, msg string) diagnostic {
	if line < 1 {
		line = 1
	}
	l := line - 1 // convert to 0-indexed
	return diagnostic{
		Range:    diagRange{Start: diagPos{Line: l}, End: diagPos{Line: l, Character: 999}},
		Severity: sev,
		Message:  msg,
		Source:   "ghat",
	}
}

// findUsesLine returns the 1-indexed line number of the first uses: line that
// contains the given action name.
func findUsesLine(content []byte, action string) int {
	for i, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "uses:") && strings.Contains(line, action) {
			return i + 1
		}
	}
	return 1
}

// classifyURI maps a document URI to a ManifestKind, returning false for
// files ghat does not handle.
func classifyURI(uri string) (core.ManifestKind, bool) {
	path := uriToPath(uri)
	base := filepath.Base(path)
	slashed := filepath.ToSlash(path)

	switch {
	case (strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) &&
		strings.Contains(slashed, ".github/workflows/"):
		return core.ManifestGHA, true
	case base == "go.mod":
		return core.ManifestGoMod, true
	case base == "package.json":
		return core.ManifestNPM, true
	case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
		return core.ManifestPyPI, true
	case base == "Cargo.toml":
		return core.ManifestCargo, true
	case base == "Gemfile":
		return core.ManifestGem, true
	case base == ".pre-commit-config.yaml" || base == ".pre-commit-config.yml":
		return core.ManifestPreCommit, true
	case base == "cpanfile":
		return core.ManifestCpanfile, true
	}
	return 0, false
}

// uriToPath converts a file:// URI to a local filesystem path.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "file://")
	}
	p := u.Path
	// On Windows: /C:/foo → C:\foo
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
