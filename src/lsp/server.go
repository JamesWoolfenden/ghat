// Package lsp implements a Language Server Protocol server for ghat.
// It speaks LSP over stdio and re-analyses each open document on every
// change, publishing diagnostics for unpinned action/image references,
// missing permissions blocks, dangerous trigger patterns, and missing job
// timeouts. A "Pin to SHA" code action rewrites the reference under the
// cursor by resolving the tag against the GitHub API.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/jameswoolfenden/ghat/src/core"
)

// ----------------------------------------------------------------------------
// Minimal LSP protocol types
// ----------------------------------------------------------------------------

type position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type diagnostic struct {
	Range    lspRange        `json:"range"`
	Severity int             `json:"severity"` // 1=Error 2=Warning 3=Info 4=Hint
	Code     string          `json:"code,omitempty"`
	Source   string          `json:"source,omitempty"`
	Message  string          `json:"message"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// pinData is round-tripped via diagnostic.Data so codeAction can build the
// "Pin to SHA" edit without re-analysing. RefStart/RefEnd are byte columns
// within the diagnostic line delimiting the text to replace.
type pinData struct {
	Action     string `json:"action,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Image      string `json:"image,omitempty"`
	Dockerfile bool   `json:"dockerfile,omitempty"`
	RefStart   uint32 `json:"refStart,omitempty"`
	RefEnd     uint32 `json:"refEnd,omitempty"`
	JobsLine   int    `json:"jobsLine,omitempty"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type codeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind"`
	Diagnostics []diagnostic   `json:"diagnostics,omitempty"`
	Edit        map[string]any `json:"edit"`
}

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// ----------------------------------------------------------------------------
// JSON-RPC 2.0 framing
// ----------------------------------------------------------------------------

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result"`
}

type rpcErrorResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *rpcError        `json:"error"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ----------------------------------------------------------------------------
// Server
// ----------------------------------------------------------------------------

// Server is a ghat LSP server. Create with New and call Run.
type Server struct {
	token string

	mu    sync.Mutex
	docs  map[string][]byte  // uri → current buffer
	diags map[string]diagSet // uri → last published diagnostics, keyed by line for hover/codeAction

	out     io.Writer
	writeMu sync.Mutex
}

type diagSet map[uint32][]diagnostic

// New creates a Server. token is used by the "Pin to SHA" code action to
// authenticate GitHub API calls; it may be empty.
func New(token string) *Server {
	return &Server{
		token: token,
		docs:  make(map[string][]byte),
		diags: make(map[string]diagSet),
		out:   os.Stdout,
	}
}

// Run reads LSP messages from stdin and writes responses to stdout until the
// client sends shutdown/exit or the connection is closed.
func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		req, err := readRequest(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		go s.dispatch(req)
	}
}

// ----------------------------------------------------------------------------
// Dispatch
// ----------------------------------------------------------------------------

func (s *Server) dispatch(req rpcRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// no-op acknowledgement
	case "shutdown":
		s.reply(req.ID, nil)
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didSave":
		// buffer is already current from didChange
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/codeAction":
		s.handleCodeAction(req)
	default:
		if req.ID != nil {
			s.replyError(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

func (s *Server) handleInitialize(req rpcRequest) {
	s.reply(req.ID, map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": 1, // Full
			"hoverProvider":    true,
			"codeActionProvider": map[string]any{
				"codeActionKinds": []string{"quickfix"},
			},
		},
		"serverInfo": map[string]any{"name": "ghat"},
	})
}

// ----------------------------------------------------------------------------
// textDocument lifecycle — store buffer, analyse, publish
// ----------------------------------------------------------------------------

func (s *Server) handleDidOpen(req rpcRequest) {
	var params struct {
		TextDocument textDocumentItem `json:"textDocument"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		return
	}
	s.update(params.TextDocument.URI, []byte(params.TextDocument.Text))
}

func (s *Server) handleDidChange(req rpcRequest) {
	var params struct {
		TextDocument   textDocumentIdentifier `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if json.Unmarshal(req.Params, &params) != nil || len(params.ContentChanges) == 0 {
		return
	}
	s.update(params.TextDocument.URI, []byte(params.ContentChanges[len(params.ContentChanges)-1].Text))
}

func (s *Server) handleDidClose(req rpcRequest) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		return
	}
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	delete(s.diags, params.TextDocument.URI)
	s.mu.Unlock()
	s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri":         params.TextDocument.URI,
		"diagnostics": []diagnostic{},
	})
}

func (s *Server) update(uri string, content []byte) {
	diags := s.analyze(uri, content)

	byLine := make(diagSet, len(diags))
	for _, d := range diags {
		byLine[d.Range.Start.Line] = append(byLine[d.Range.Start.Line], d)
	}

	s.mu.Lock()
	s.docs[uri] = content
	s.diags[uri] = byLine
	s.mu.Unlock()

	s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri":         uri,
		"diagnostics": diags,
	})
}

// ----------------------------------------------------------------------------
// File-kind classification
// ----------------------------------------------------------------------------

type fileKind int

const (
	kindUnknown fileKind = iota
	kindGHA
	kindGitlab
	kindPreCommit
	kindDockerfile
)

var dockerfileRe = regexp.MustCompile(`(?i)^(dockerfile|containerfile)([._-].*)?$`)

func classify(path string) fileKind {
	base := filepath.Base(path)
	dir := filepath.ToSlash(filepath.Dir(path))
	ext := strings.ToLower(filepath.Ext(base))

	switch {
	case base == ".gitlab-ci.yml" || base == ".gitlab-ci.yaml":
		return kindGitlab
	case base == ".pre-commit-config.yaml" || base == ".pre-commit-config.yml":
		return kindPreCommit
	case ext == ".dockerfile" || dockerfileRe.MatchString(base):
		return kindDockerfile
	case (ext == ".yml" || ext == ".yaml") &&
		(strings.HasSuffix(dir, "/.github/workflows") ||
			base == "action.yml" || base == "action.yaml"):
		return kindGHA
	}
	return kindUnknown
}

// ----------------------------------------------------------------------------
// Analysis → diagnostics
// ----------------------------------------------------------------------------

func (s *Server) analyze(uri string, content []byte) []diagnostic {
	switch classify(uriToPath(uri)) {
	case kindGHA:
		return diagnoseGHA(content)
	case kindGitlab:
		return diagnoseGitlab(content)
	case kindPreCommit:
		return diagnosePreCommit(content)
	case kindDockerfile:
		return diagnoseDockerfile(content)
	}
	return nil
}

func diagnoseGHA(content []byte) []diagnostic {
	a := core.AnalyzeWorkflow("", content)
	var out []diagnostic

	if !a.HasPermissions {
		out = append(out, mk(1, "ghat-permissions", 2,
			"workflow has no top-level permissions: block — GITHUB_TOKEN defaults to broad write access",
			pinData{JobsLine: a.JobsLine}))
	}
	if a.IsWriteAll {
		out = append(out, mk(a.WriteAllLine, "ghat-write-all", 1,
			"permissions: write-all grants the GITHUB_TOKEN full repository write access", pinData{}))
	}
	if a.HasDangerousTrigger {
		out = append(out, mk(a.DangerousTriggerLine, "ghat-dangerous-trigger", 1,
			strings.TrimPrefix(a.DangerousTriggerDesc, ": "), pinData{}))
	}
	if !a.HasConcurrency {
		out = append(out, mk(1, "ghat-concurrency", 4,
			"no top-level concurrency: block — overlapping runs can corrupt deploy state", pinData{}))
	}

	for _, st := range a.Steps {
		if st.Suppressed {
			continue
		}
		if !st.IsSHAPinned {
			rs, re := refSpan(content, st.Line)
			out = append(out, mk(st.Line, "ghat-pin", 2,
				fmt.Sprintf("uses: %s@%s is not pinned to a commit SHA", st.Action, st.Tag),
				pinData{Action: st.Action, Tag: st.Tag, RefStart: rs, RefEnd: re}))
		}
		if st.ExposesSecretInEnv {
			out = append(out, mk(st.Line, "ghat-secret-env", 2,
				fmt.Sprintf("%s exposes ${{ secrets.* }} in env: — secret values leak to child processes and debug logs", st.Action),
				pinData{}))
		}
	}

	for _, j := range a.Jobs {
		if !j.HasTimeout && !j.IsReusable {
			out = append(out, mk(j.Line, "ghat-timeout", 3,
				fmt.Sprintf("job %q has no timeout-minutes: — defaults to 6 h", j.Name), pinData{}))
		}
	}
	return out
}

func diagnoseGitlab(content []byte) []diagnostic {
	a := core.AnalyzeGitlabCI(content)
	var out []diagnostic
	for _, j := range a.Jobs {
		if !j.HasTimeout {
			out = append(out, mk(j.Line, "ghat-timeout", 3,
				fmt.Sprintf("job %q has no timeout:", j.Name), pinData{}))
		}
		for _, img := range j.Images {
			if img.IsSuppressed || img.IsDigestPinned {
				continue
			}
			rs, re := imageSpan(content, img.Line, img.Name)
			out = append(out, mk(img.Line, "ghat-image-pin", 2,
				fmt.Sprintf("image %s is not pinned to a sha256 digest", img.Name),
				pinData{Image: img.Name, RefStart: rs, RefEnd: re}))
		}
	}
	return out
}

func diagnosePreCommit(content []byte) []diagnostic {
	a := core.AnalyzePreCommit(content)
	var out []diagnostic
	for _, r := range a.Repos {
		if r.Suppressed || r.IsSHAPinned {
			continue
		}
		rs, re := refSpan(content, r.Line)
		action := strings.TrimPrefix(strings.TrimPrefix(r.Repo, "https://github.com/"), "git@github.com:")
		out = append(out, mk(r.Line, "ghat-pin", 2,
			fmt.Sprintf("rev: %s for %s is not pinned to a commit SHA", r.Rev, r.Repo),
			pinData{Action: action, Tag: r.Rev, RefStart: rs, RefEnd: re}))
	}
	return out
}

func diagnoseDockerfile(content []byte) []diagnostic {
	a := core.AnalyzeDockerfile(content)
	var out []diagnostic
	for _, img := range a.Images {
		if img.Suppressed || img.IsDigestPinned {
			continue
		}
		rs, re := imageSpan(content, img.Line, img.Raw)
		out = append(out, mk(img.Line, "ghat-image-pin", 2,
			fmt.Sprintf("FROM %s is not pinned to a sha256 digest", img.Raw),
			pinData{Image: img.Resolved, Dockerfile: true, RefStart: rs, RefEnd: re}))
	}
	return out
}

// mk builds a diagnostic spanning a single 1-indexed source line.
func mk(line int, code string, sev int, msg string, data pinData) diagnostic {
	d := diagnostic{
		Range:    lineRange(line),
		Severity: sev,
		Code:     code,
		Source:   "ghat",
		Message:  msg,
	}
	if data != (pinData{}) {
		raw, _ := json.Marshal(data)
		d.Data = raw
	}
	return d
}

func lineRange(line int) lspRange {
	l := uint32(0)
	if line > 0 {
		l = uint32(line - 1)
	}
	return lspRange{Start: position{Line: l}, End: position{Line: l + 1}}
}

// refSpan returns the [start,end) byte columns of the ref to replace on the
// given 1-indexed line: everything after `@` on a `uses:` line, or everything
// after `: ` on a `rev:` line.
func refSpan(content []byte, line int) (uint32, uint32) {
	txt := lineText(content, line)
	colon := strings.Index(txt, ":")
	if colon < 0 {
		return 0, uint32(len(txt))
	}
	start := colon + 1
	for start < len(txt) && txt[start] == ' ' {
		start++
	}
	if at := strings.Index(txt[start:], "@"); at >= 0 {
		start += at + 1
	}
	return uint32(start), uint32(len(txt))
}

// imageSpan returns the [start,end) byte columns of literal `image` on the
// given line. Falls back to whole-line when the literal isn't found verbatim
// (e.g. ARG-expanded Dockerfile refs).
func imageSpan(content []byte, line int, image string) (uint32, uint32) {
	txt := lineText(content, line)
	if i := strings.Index(txt, image); i >= 0 {
		return uint32(i), uint32(i + len(image))
	}
	return 0, uint32(len(txt))
}

func lineText(content []byte, line int) string {
	if line < 1 {
		return ""
	}
	lines := strings.SplitAfterN(string(content), "\n", line+1)
	if line > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[line-1], "\n")
}

// ----------------------------------------------------------------------------
// textDocument/hover
// ----------------------------------------------------------------------------

func (s *Server) handleHover(req rpcRequest) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
		Position     position               `json:"position"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		s.reply(req.ID, nil)
		return
	}

	s.mu.Lock()
	ds := s.diags[params.TextDocument.URI][params.Position.Line]
	s.mu.Unlock()

	if len(ds) == 0 {
		s.reply(req.ID, nil)
		return
	}
	var b strings.Builder
	for i, d := range ds {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&b, "**%s** `%s`\n\n%s", d.Message, d.Code, hoverDetail(d.Code))
	}
	s.reply(req.ID, map[string]any{
		"contents": markupContent{Kind: "markdown", Value: b.String()},
		"range":    ds[0].Range,
	})
}

func hoverDetail(code string) string {
	switch code {
	case "ghat-pin":
		return "Pin third-party actions and hooks to an immutable commit SHA so a moved tag cannot silently change the code you run. Use the *Pin to SHA* quick-fix or run `ghat swot` / `ghat sift`."
	case "ghat-image-pin":
		return "Pin container images to a sha256 digest (`image:tag@sha256:…`) so registry tag moves cannot change your build. Run `ghat dock` / `ghat stun`."
	case "ghat-permissions":
		return "Without an explicit `permissions:` block the workflow's GITHUB_TOKEN inherits the repository default, which is often write-all. Declare least-privilege scopes; use the *Insert permissions block* quick-fix."
	case "ghat-write-all":
		return "`permissions: write-all` grants every job full write access to the repository. Replace with the minimal scopes each job needs."
	case "ghat-dangerous-trigger":
		return "This pattern lets untrusted PR content run with elevated privileges or inject shell. See https://securitylab.github.com/research/github-actions-preventing-pwn-requests/."
	case "ghat-timeout":
		return "Jobs without `timeout-minutes:` can run for up to 6 hours, burning runner minutes if they hang."
	case "ghat-concurrency":
		return "A top-level `concurrency:` block cancels superseded runs and prevents two deployments racing on the same ref."
	case "ghat-secret-env":
		return "Pass secrets via `with:` inputs instead of `env:` so they are not exported to every child process and masked from debug logs."
	}
	return ""
}

// ----------------------------------------------------------------------------
// textDocument/codeAction
// ----------------------------------------------------------------------------

func (s *Server) handleCodeAction(req rpcRequest) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
		Range        lspRange               `json:"range"`
		Context      struct {
			Diagnostics []diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		s.reply(req.ID, []any{})
		return
	}
	uri := params.TextDocument.URI

	s.mu.Lock()
	content := s.docs[uri]
	cached := s.diags[uri]
	s.mu.Unlock()

	// Prefer client-supplied diagnostics; fall back to cached when absent.
	diags := params.Context.Diagnostics
	if len(diags) == 0 {
		for l := params.Range.Start.Line; l <= params.Range.End.Line; l++ {
			diags = append(diags, cached[l]...)
		}
	}

	var actions []codeAction
	for _, d := range diags {
		actions = append(actions, s.actionsFor(uri, content, d)...)
	}
	s.reply(req.ID, actions)
}

func (s *Server) actionsFor(uri string, content []byte, d diagnostic) []codeAction {
	var out []codeAction
	line := int(d.Range.Start.Line) + 1

	var data pinData
	_ = json.Unmarshal(d.Data, &data)

	switch d.Code {
	case "ghat-pin":
		if data.Action != "" && data.Tag != "" {
			sha, err := core.ResolveActionSHA(data.Action, data.Tag, s.token)
			if err == nil {
				out = append(out, edit(uri,
					fmt.Sprintf("Pin %s@%s to %.7s", data.Action, data.Tag, sha),
					d, textEdit{
						Range: lspRange{
							Start: position{Line: d.Range.Start.Line, Character: data.RefStart},
							End:   position{Line: d.Range.Start.Line, Character: data.RefEnd},
						},
						NewText: sha + " # " + data.Tag,
					}))
			} else {
				s.logError("ghat: resolve %s@%s: %v", data.Action, data.Tag, err)
			}
		}
	case "ghat-image-pin":
		if data.Image != "" && data.RefEnd > data.RefStart {
			pinned, err := core.ResolveImageDigest(data.Image, data.Dockerfile, s.token)
			if err == nil {
				out = append(out, edit(uri,
					fmt.Sprintf("Pin %s to digest", data.Image),
					d, textEdit{
						Range: lspRange{
							Start: position{Line: d.Range.Start.Line, Character: data.RefStart},
							End:   position{Line: d.Range.Start.Line, Character: data.RefEnd},
						},
						NewText: pinned,
					}))
			} else {
				s.logError("ghat: resolve digest for %s: %v", data.Image, err)
			}
		}
	case "ghat-permissions":
		if data.JobsLine > 0 {
			at := uint32(data.JobsLine - 1)
			out = append(out, edit(uri, "Insert least-privilege permissions block", d, textEdit{
				Range:   lspRange{Start: position{Line: at}, End: position{Line: at}},
				NewText: "permissions:\n  contents: read\n\n",
			}))
		}
	}

	if d.Code != "ghat-permissions" && d.Code != "ghat-concurrency" {
		txt := lineText(content, line)
		out = append(out, edit(uri, "Suppress (ghat:suppress)", d, textEdit{
			Range: lspRange{
				Start: position{Line: d.Range.Start.Line, Character: uint32(len(txt))},
				End:   position{Line: d.Range.Start.Line, Character: uint32(len(txt))},
			},
			NewText: "  # ghat:suppress",
		}))
	}
	return out
}

func edit(uri, title string, d diagnostic, te textEdit) codeAction {
	return codeAction{
		Title:       title,
		Kind:        "quickfix",
		Diagnostics: []diagnostic{d},
		Edit: map[string]any{
			"changes": map[string][]textEdit{uri: {te}},
		},
	}
}

// ----------------------------------------------------------------------------
// Transport helpers
// ----------------------------------------------------------------------------

func (s *Server) reply(id *json.RawMessage, result any) {
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) replyError(id *json.RawMessage, code int, msg string) {
	s.write(rpcErrorResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) notify(method string, params any) {
	s.write(rpcNotification{JSONRPC: "2.0", Method: method, Params: params})
}

func (s *Server) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(data))
	_, _ = s.out.Write(data)
}

func readRequest(r *bufio.Reader) (rpcRequest, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return rpcRequest{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			_, _ = fmt.Sscanf(line[16:], "%d", &contentLength)
		}
	}
	if contentLength == 0 {
		return rpcRequest{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return rpcRequest{}, err
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return rpcRequest{}, err
	}
	return req, nil
}

func (s *Server) logError(format string, args ...any) {
	s.notify("window/logMessage", map[string]any{"type": 1, "message": fmt.Sprintf(format, args...)})
}

// ----------------------------------------------------------------------------
// URI helpers
// ----------------------------------------------------------------------------

func uriToPath(uri string) string {
	path := strings.TrimPrefix(uri, "file://")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	path = filepath.FromSlash(path)
	if runtime.GOOS == "windows" && len(path) > 2 && (path[0] == '/' || path[0] == '\\') && path[2] == ':' {
		path = path[1:]
	}
	return path
}
