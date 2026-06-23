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
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jameswoolfenden/ghat/src/core"
)

// moduleRefSHARe matches a git module source already pinned to an immutable
// commit SHA via ?ref=<40-hex>, the format ghat itself writes (with the tag
// recorded as a trailing "# vX.Y.Z" comment rather than a version attribute).
var moduleRefSHARe = regexp.MustCompile(`\?ref=[0-9a-f]{40}(&|$)`)

// Server is a stateful stdio LSP server.
type Server struct {
	token string
	cache *core.Cache

	mu         sync.Mutex
	docs       map[string][]byte        // uri → current content
	deps       map[string][]core.DepRef // uri → dep refs from ParseManifest
	auditDiags map[string][]diagnostic  // uri → diagnostics appended by executeCommand

	wmu sync.Mutex // serialises all writes to the stdout writer
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
			// I/O errors on the pipe are fatal; framing/parse errors are skipped.
			if isIOError(err) {
				return err
			}
			fmt.Fprintf(os.Stderr, "ghat-lsp: skipping malformed message: %v\n", err)
			continue
		}
		s.wmu.Lock()
		err = s.dispatch(wr, msg)
		_ = wr.Flush()
		s.wmu.Unlock()
		if err != nil {
			return err
		}
	}
}

// lockedWrite serialises a write + flush from any goroutine.
func (s *Server) lockedWrite(w *bufio.Writer, fn func(io.Writer) error) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	if err := fn(w); err != nil {
		return err
	}
	return w.Flush()
}

func isIOError(err error) bool {
	return err == io.ErrUnexpectedEOF || err == io.ErrClosedPipe
}

// ---- JSON-RPC plumbing -------------------------------------------------------

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
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
	// Responses to our workspace/applyEdit requests — discard.
	if msg.Method == "" && msg.ID != nil {
		return nil
	}
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
			"textDocumentSync": 1, // full-document sync
			"codeActionProvider": map[string]interface{}{
				"codeActionKinds": []string{"source", "source.ghat"},
				"resolveProvider": false,
			},
			"executeCommandProvider": map[string]interface{}{
				"commands": []string{"ghat.audit", "ghat.auditFile", "ghat.pin", "ghat.update", "ghat.suppress"},
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
		// Content-based detection for Kubernetes manifests: any YAML not
		// already matched by classifyURI is checked for apiVersion+kind.
		p := uriToPath(uri)
		if (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) &&
			core.HasKubeResource(content) {
			kind = core.ManifestKube
		} else {
			return s.publishDiags(w, uri, nil)
		}
	}

	refs := core.ParseManifest(kind, content)
	s.mu.Lock()
	s.deps[uri] = refs
	s.mu.Unlock()

	switch kind {
	case core.ManifestGHA:
		return s.publishDiags(w, uri, ghaStaticDiags(uriToPath(uri), content))
	case core.ManifestPreCommit:
		return s.publishDiags(w, uri, preCommitStaticDiags(refs))
	case core.ManifestDockerfile:
		return s.publishDiags(w, uri, dockerfileStaticDiags(refs))
	case core.ManifestGitLab:
		return s.publishDiags(w, uri, gitlabStaticDiags(refs))
	case core.ManifestKube, core.ManifestCompose:
		return s.publishDiags(w, uri, imageStaticDiags(refs))
	case core.ManifestTerraform:
		return s.publishDiags(w, uri, terraformStaticDiags(refs))
	case core.ManifestCpanfile:
		return s.publishDiags(w, uri, cpanfileStaticDiags(refs))
	default:
		return s.publishDiags(w, uri, nil)
	}
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

// dockerfileStaticDiags warns on FROM lines not pinned to a digest.
func dockerfileStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		if strings.HasPrefix(ref.Version, "sha256:") {
			continue
		}
		tag := ref.Version
		if tag == "" {
			tag = "latest"
		}
		diags = append(diags, lineDiag(ref.Line, 2,
			ref.Name+":"+tag+" is not pinned to an immutable digest (@sha256:...)"))
	}
	return diags
}

// gitlabStaticDiags warns on GitLab CI refs that are not immutably pinned.
// Component/project include refs should be commit-SHA pinned; image refs should be digest-pinned.
func gitlabStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		switch ref.Ecosystem {
		case core.SourceGitLabComponent:
			if core.IsSHAPinnedRef(ref.Version) {
				continue
			}
			diags = append(diags, lineDiag(ref.Line, 2,
				ref.Name+"@"+ref.Version+" is not pinned to an immutable commit SHA"))
		case core.SourceGitLab:
			if strings.HasPrefix(ref.Version, "sha256:") {
				continue
			}
			tag := ref.Version
			if tag == "" {
				tag = "latest"
			}
			diags = append(diags, lineDiag(ref.Line, 2,
				ref.Name+":"+tag+" is not pinned to an immutable digest (@sha256:...)"))
		}
	}
	return diags
}

// preCommitStaticDiags warns on any repo whose rev is not a SHA pin.
func preCommitStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		if core.IsSHAPinnedRef(ref.Version) {
			continue
		}
		diags = append(diags, lineDiag(ref.Line, 2,
			ref.Name+" rev "+ref.Version+" is not pinned to an immutable SHA"))
	}
	return diags
}

// imageStaticDiags warns on container images not pinned to an immutable digest.
// Used for both Kubernetes manifests and Docker Compose files.
func imageStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		if strings.HasPrefix(ref.Version, "sha256:") {
			continue
		}
		tag := ref.Version
		if tag == "" {
			tag = "latest"
		}
		diags = append(diags, lineDiag(ref.Line, 2,
			ref.Name+":"+tag+" is not pinned to an immutable digest (@sha256:...)"))
	}
	return diags
}

// terraformStaticDiags warns on providers/modules using version constraints
// instead of exact pins.
func terraformStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		if ref.Version == "" {
			if moduleRefSHARe.MatchString(ref.Name) {
				// Already pinned to an immutable SHA (ghat writes the tag as a
				// trailing comment, not a version attribute) — nothing to flag.
				continue
			}
			diags = append(diags, lineDiag(ref.Line, 2,
				ref.Name+" has no version constraint — run ghat to pin"))
			continue
		}
		if hasVersionConstraintOperator(ref.Version) {
			diags = append(diags, lineDiag(ref.Line, 2,
				ref.Name+" uses version constraint "+ref.Version+" instead of an exact pin"))
		}
	}
	return diags
}

func hasVersionConstraintOperator(v string) bool {
	for _, op := range []string{"~>", ">=", "<=", ">", "<", "!="} {
		if strings.Contains(v, op) {
			return true
		}
	}
	return false
}

// cpanfileStaticDiags warns on CPAN modules not pinned to an exact version with ==.
func cpanfileStaticDiags(refs []core.DepRef) []diagnostic {
	var diags []diagnostic
	for _, ref := range refs {
		if strings.Contains(ref.Version, "==") {
			continue
		}
		desc := ref.Name
		if ref.Version != "" {
			desc += " (" + ref.Version + ")"
		}
		diags = append(diags, lineDiag(ref.Line, 2,
			desc+" is not pinned to an exact version (use == <ver>)"))
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

	kind, isKnown := classifyURI(p.TextDocument.URI)

	var actions []codeAction
	for _, ref := range refs {
		versionLine := ref.VersionLine
		if versionLine == 0 {
			versionLine = ref.Line
		}
		if ref.Line == cursorLine || versionLine == cursorLine {
			// Audit this dep — only for ecosystems AuditOne supports.
			if canAudit(ref.Ecosystem) {
				auditTitle := "Audit " + ref.Name
				actions = append(actions, codeAction{
					Title: auditTitle,
					Kind:  "source.ghat",
					Command: &lspCommand{
						Title:     auditTitle,
						Command:   "ghat.audit",
						Arguments: []interface{}{p.TextDocument.URI, ref.Line},
					},
				})
			}

			// Pin to SHA — only for GHA and pre-commit where SHA pinning applies.
			if isKnown && (kind == core.ManifestGHA || kind == core.ManifestPreCommit) &&
				ref.Version != "" && !core.IsSHAPinnedRef(ref.Version) {
				pinTitle := "Pin " + ref.Name + "@" + ref.Version + " to SHA"
				actions = append(actions, codeAction{
					Title: pinTitle,
					Kind:  "source.ghat",
					Command: &lspCommand{
						Title:     pinTitle,
						Command:   "ghat.pin",
						Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Name, ref.Version},
					},
				})
			}

			// Update to latest / Pin to digest — for ecosystems we can resolve server-side.
			if canUpdate(ref.Ecosystem) && ref.Version != "" {
				switch ref.Ecosystem {
				case core.SourceGitLab, core.SourceKube, core.SourceCompose, core.SourceDockerfile:
					// Two actions for image refs: pin current tag, or fetch latest + pin.
					pinTitle := "Pin " + ref.Name + ":" + ref.Version + " to digest"
					actions = append(actions, codeAction{
						Title: pinTitle,
						Kind:  "source.ghat",
						Command: &lspCommand{
							Title:   pinTitle,
							Command: "ghat.update",
							// 6th arg: fetchTag = currentVersion (pin as-is)
							Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Ecosystem, ref.Name, ref.Version, ref.Version},
						},
					})
					latestTitle := "Update " + ref.Name + " to latest (pinned)"
					actions = append(actions, codeAction{
						Title: latestTitle,
						Kind:  "source.ghat",
						Command: &lspCommand{
							Title:   latestTitle,
							Command: "ghat.update",
							// 6th arg: fetchTag = "latest"
							Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Ecosystem, ref.Name, ref.Version, "latest"},
						},
					})
				case core.SourceGitLabComponent:
					compName := ref.Name
					if idx := strings.LastIndex(ref.Name, "/"); idx >= 0 {
						compName = ref.Name[idx+1:]
					}
					pinTitle := "Pin " + compName + "@" + ref.Version + " to commit SHA"
					actions = append(actions, codeAction{
						Title: pinTitle,
						Kind:  "source.ghat",
						Command: &lspCommand{
							Title:     pinTitle,
							Command:   "ghat.update",
							Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Ecosystem, ref.Name, ref.Version},
						},
					})
					latestTitle := "Update " + compName + " to latest (pinned)"
					actions = append(actions, codeAction{
						Title: latestTitle,
						Kind:  "source.ghat",
						Command: &lspCommand{
							Title:   latestTitle,
							Command: "ghat.update",
							// 6th arg "latest" → resolve newest tag then pin to its SHA
							Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Ecosystem, ref.Name, ref.Version, "latest"},
						},
					})
				default:
					updateTitle := "Update " + ref.Name + " to latest"
					actions = append(actions, codeAction{
						Title: updateTitle,
						Kind:  "source.ghat",
						Command: &lspCommand{
							Title:     updateTitle,
							Command:   "ghat.update",
							Arguments: []interface{}{p.TextDocument.URI, ref.Line, ref.Ecosystem, ref.Name, ref.Version},
						},
					})
				}
			}

			// View on registry.
			if url := registryURL(ref.Ecosystem, ref.Name); url != "" {
				viewTitle := "View " + ref.Name + " on " + ecosystemLabel(ref.Ecosystem)
				actions = append(actions, codeAction{
					Title: viewTitle,
					Kind:  "source.ghat",
					Command: &lspCommand{
						Title:     viewTitle,
						Command:   "vscode.open",
						Arguments: []interface{}{url},
					},
				})
			}

			// Suppress.
			suppressTitle := "Suppress ghat warning for " + ref.Name
			actions = append(actions, codeAction{
				Title: suppressTitle,
				Kind:  "source.ghat",
				Command: &lspCommand{
					Title:     suppressTitle,
					Command:   "ghat.suppress",
					Arguments: []interface{}{p.TextDocument.URI, ref.Line},
				},
			})
		}
	}
	if len(refs) > 0 {
		actions = append(actions, codeAction{
			Title: "Audit all dependencies in this file",
			Kind:  "source.ghat",
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
	case "ghat.pin":
		return s.execPin(w, msg.ID, p.Arguments)
	case "ghat.update":
		return s.execUpdate(w, msg.ID, p.Arguments)
	case "ghat.suppress":
		return s.execSuppress(w, msg.ID, p.Arguments)
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

	_ = writeResult(w, id, nil)
	bw := w.(*bufio.Writer)
	snapshot := *ref
	go func() {
		score, err := core.AuditOne(snapshot.Ecosystem, snapshot.Name, snapshot.Version, s.token, s.cache)
		var diag diagnostic
		if err != nil {
			diag = lineDiag(snapshot.Line, 2, fmt.Sprintf("audit %s: %v", snapshot.Name, err))
		} else {
			diag = scoreToDiag(score, snapshot.Name, snapshot.Line)
		}
		s.mu.Lock()
		s.auditDiags[uri] = append(s.auditDiags[uri], diag)
		s.mu.Unlock()
		_ = s.lockedWrite(bw, func(w io.Writer) error { return s.refreshDiags(w, uri) })
	}()
	return nil
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
	bw := w.(*bufio.Writer)
	go func() {
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
		_ = s.lockedWrite(bw, func(w io.Writer) error { return s.refreshDiags(w, uri) })
	}()
	return nil
}

// execPin resolves a GHA action tag to its commit SHA and applies the edit.
func (s *Server) execPin(w io.Writer, id json.RawMessage, args json.RawMessage) error {
	var argv []interface{}
	if err := json.Unmarshal(args, &argv); err != nil || len(argv) < 4 {
		return writeResult(w, id, nil)
	}
	uri, _ := argv[0].(string)
	lineFloat, _ := argv[1].(float64)
	action, _ := argv[2].(string)
	tag, _ := argv[3].(string)
	line := int(lineFloat)
	_ = writeResult(w, id, nil)

	// Pre-commit repos arrive as full GitHub URLs; strip to "owner/repo".
	action = strings.TrimPrefix(action, "https://github.com/")

	bw := w.(*bufio.Writer)
	go func() {
		sha, err := core.ResolveTagSHA(action, tag, s.token)
		if err != nil {
			return
		}
		s.mu.Lock()
		content := s.docs[uri]
		s.mu.Unlock()
		if content == nil || line < 1 {
			return
		}
		lines := strings.Split(string(content), "\n")
		if line-1 >= len(lines) {
			return
		}
		old := lines[line-1]
		newLine := strings.Replace(old, "@"+tag, "@"+sha+" # "+tag, 1)
		if newLine == old {
			return
		}
		_ = s.lockedWrite(bw, func(w io.Writer) error { return applyEdit(w, uri, line-1, newLine) })
	}()
	return nil
}

// execSuppress adds a # ghat:suppress comment to the flagged uses: line.
func (s *Server) execSuppress(w io.Writer, id json.RawMessage, args json.RawMessage) error {
	var argv []interface{}
	if err := json.Unmarshal(args, &argv); err != nil || len(argv) < 2 {
		return writeResult(w, id, nil)
	}
	uri, _ := argv[0].(string)
	lineFloat, _ := argv[1].(float64)
	line := int(lineFloat)
	_ = writeResult(w, id, nil)

	s.mu.Lock()
	content := s.docs[uri]
	s.mu.Unlock()
	if content == nil || line < 1 {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	if line-1 >= len(lines) {
		return nil
	}
	new := strings.TrimRight(lines[line-1], " \t") + "  # ghat:suppress"
	return applyEdit(w, uri, line-1, new)
}

// canUpdate reports whether the LSP can resolve the latest version server-side.
func canUpdate(eco string) bool {
	switch eco {
	case core.SourceGHA, core.SourcePreCommit, core.SourceTerraform,
		core.SourceGitLab, core.SourceKube, core.SourceCompose,
		core.SourceDockerfile, core.SourceGitLabComponent,
		core.SourceNpm, core.SourcePypi, core.SourceCargo, core.SourceGem, core.SourceGo,
		core.SourceCpanfile:
		return true
	}
	return false
}

// execUpdate resolves the latest version for a dependency and applies a
// workspace edit to update it in the document.
//
// argv: [uri, line, ecosystem, name, currentVersion]
func (s *Server) execUpdate(w io.Writer, id json.RawMessage, args json.RawMessage) error {
	var argv []interface{}
	if err := json.Unmarshal(args, &argv); err != nil || len(argv) < 5 {
		return writeResult(w, id, nil)
	}
	uri, _ := argv[0].(string)
	lineFloat, _ := argv[1].(float64)
	eco, _ := argv[2].(string)
	name, _ := argv[3].(string)
	currentVersion, _ := argv[4].(string)
	refLine := int(lineFloat)

	_ = writeResult(w, id, nil) // ack immediately

	bw := w.(*bufio.Writer)
	go func() {
		var oldText, newText string
		switch eco {
		case core.SourceGHA:
			sha, tag, err := core.ResolveLatestSHA(name, s.token)
			if err != nil {
				return
			}
			oldText = currentVersion
			newText = sha + " # " + tag

		case core.SourcePreCommit:
			ownerRepo := strings.TrimPrefix(name, "https://github.com/")
			sha, tag, err := core.ResolveLatestSHA(ownerRepo, s.token)
			if err != nil {
				return
			}
			oldText = currentVersion
			newText = sha + "  # " + tag

		case core.SourceTerraform:
			parts := strings.SplitN(name, "/", 3)
			if len(parts) != 2 {
				return
			}
			latest, err := core.GetLatestProviderVersion(parts[0], parts[1])
			if err != nil {
				return
			}
			oldText = currentVersion
			newText = latest

		case core.SourceGitLab, core.SourceKube, core.SourceCompose, core.SourceDockerfile:
			// 6th arg (optional): the tag to fetch. Defaults to currentVersion (pin as-is).
			fetchTag := currentVersion
			if len(argv) >= 6 {
				if t, ok := argv[5].(string); ok && t != "" {
					fetchTag = t
				}
			}
			imageRef := name
			if fetchTag != "" {
				imageRef = name + ":" + fetchTag
			}
			dockerfileStyle := eco == core.SourceDockerfile
			pinned, err := core.ResolveImageDigest(imageRef, dockerfileStyle, s.token)
			if err != nil {
				return
			}
			// Always replace the full "name:currentVersion" token in the file.
			oldText = name
			if currentVersion != "" {
				oldText = name + ":" + currentVersion
			}
			newText = pinned

		case core.SourceGitLabComponent:
			var sha, resolvedTag string
			if len(argv) >= 6 {
				if t, ok := argv[5].(string); ok && t == "latest" {
					var e error
					sha, resolvedTag, e = core.ResolveGitLabComponentLatest(name, "")
					if e != nil {
						return
					}
				}
			}
			if sha == "" {
				var e error
				sha, e = core.ResolveGitLabComponentSHA(name, currentVersion, "")
				if e != nil {
					return
				}
				resolvedTag = currentVersion
			}
			oldText = name + "@" + currentVersion
			newText = name + "@" + sha + " # " + resolvedTag

		case core.SourceNpm, core.SourceCargo, core.SourceGem, core.SourceGo:
			latest, err := core.GetLatestPackageVersion(eco, name)
			if err != nil {
				return
			}
			oldText = currentVersion
			newText = latest

		case core.SourceCpanfile:
			latest, err := core.GetLatestPackageVersion(eco, name)
			if err != nil {
				return
			}
			oldText = currentVersion
			// Always normalise to exact pin; replace any existing constraint in-place.
			newText = "== " + latest

		case core.SourcePypi:
			latest, err := core.GetLatestPackageVersion(eco, name)
			if err != nil {
				return
			}
			oldText = currentVersion
			// Normalise to exact pin regardless of the current constraint operator.
			newText = "==" + latest

		default:
			return
		}

		s.mu.Lock()
		content := s.docs[uri]
		s.mu.Unlock()
		if content == nil {
			return
		}
		lines := strings.Split(string(content), "\n")
		zeroLine := findVersionLine(lines, refLine-1, oldText)
		if zeroLine < 0 {
			return
		}
		newLine := strings.Replace(lines[zeroLine], oldText, newText, 1)
		if newLine == lines[zeroLine] {
			return
		}
		_ = s.lockedWrite(bw, func(w io.Writer) error { return applyEdit(w, uri, zeroLine, newLine) })
	}()
	return nil
}

// findVersionLine searches lines starting at startIdx for the first line
// containing the version string. Returns the 0-indexed line number or -1.
func findVersionLine(lines []string, startIdx int, version string) int {
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(lines) && i < startIdx+10; i++ {
		if strings.Contains(lines[i], version) {
			return i
		}
	}
	// Fallback: search the whole file
	for i, l := range lines {
		if strings.Contains(l, version) {
			return i
		}
	}
	return -1
}

// applyEdit sends a workspace/applyEdit request to replace a single line.
func applyEdit(w io.Writer, uri string, zeroLine int, newText string) error {
	return writeMessage(w, map[string]any{
		"jsonrpc": "2.0",
		"id":      99,
		"method":  "workspace/applyEdit",
		"params": map[string]any{
			"edit": map[string]any{
				"changes": map[string]any{
					uri: []map[string]any{
						{
							"range": diagRange{
								Start: diagPos{Line: zeroLine, Character: 0},
								End:   diagPos{Line: zeroLine, Character: 9999},
							},
							"newText": newText,
						},
					},
				},
			},
		},
	})
}

// canAudit reports whether AuditOne supports the given ecosystem.
func canAudit(eco string) bool {
	switch eco {
	case core.SourceGHA, core.SourceGo, core.SourceNpm, core.SourcePypi,
		core.SourceCargo, core.SourceGem, core.SourcePreCommit:
		return true
	}
	return false
}

// registryURL returns a browser URL for viewing a dependency.
func registryURL(eco, name string) string {
	switch eco {
	case core.SourceGHA:
		return "https://github.com/" + name
	case core.SourceGo:
		return "https://pkg.go.dev/" + name
	case core.SourceNpm:
		return "https://www.npmjs.com/package/" + name
	case core.SourcePypi:
		return "https://pypi.org/project/" + name
	case core.SourceCargo:
		return "https://crates.io/crates/" + name
	case core.SourceGem:
		return "https://rubygems.org/gems/" + name
	case core.SourcePreCommit:
		return name // already a full URL
	case core.SourceKube, core.SourceCompose:
		if !strings.Contains(name, ".") {
			parts := strings.Split(name, "/")
			if len(parts) == 1 {
				return "https://hub.docker.com/_/" + name
			}
			return "https://hub.docker.com/r/" + name
		}
		return ""
	case core.SourceTerraform:
		parts := strings.Split(name, "/")
		if len(parts) == 2 {
			return "https://registry.terraform.io/providers/" + name
		}
		if len(parts) >= 3 {
			return "https://registry.terraform.io/modules/" + name
		}
		return ""
	default:
		return ""
	}
}

func ecosystemLabel(eco string) string {
	switch eco {
	case core.SourceGHA:
		return "GitHub"
	case core.SourceGo:
		return "pkg.go.dev"
	case core.SourceNpm:
		return "npm"
	case core.SourcePypi:
		return "PyPI"
	case core.SourceCargo:
		return "crates.io"
	case core.SourceGem:
		return "RubyGems"
	case core.SourcePreCommit:
		return "GitHub"
	case core.SourceKube, core.SourceCompose:
		return "Docker Hub"
	case core.SourceTerraform:
		return "Terraform Registry"
	default:
		return "registry"
	}
}

func (s *Server) refreshDiags(w io.Writer, uri string) error {
	s.mu.Lock()
	content := s.docs[uri]
	s.mu.Unlock()

	var staticDiags []diagnostic
	if kind, ok := classifyURI(uri); ok && content != nil {
		s.mu.Lock()
		refs := s.deps[uri]
		s.mu.Unlock()
		switch kind {
		case core.ManifestGHA:
			staticDiags = ghaStaticDiags(uriToPath(uri), content)
		case core.ManifestPreCommit:
			staticDiags = preCommitStaticDiags(refs)
		case core.ManifestDockerfile:
			staticDiags = dockerfileStaticDiags(refs)
		case core.ManifestGitLab:
			staticDiags = gitlabStaticDiags(refs)
		case core.ManifestKube, core.ManifestCompose:
			staticDiags = imageStaticDiags(refs)
		case core.ManifestTerraform:
			staticDiags = terraformStaticDiags(refs)
		}
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
	return lineDiag(line, sev, formatScore(name, score))
}

func formatScore(name string, score core.AuditScore) string {
	var b strings.Builder
	fmt.Fprintf(&b, "audit %s [%s]", name, score.Bucket)

	// Failed checks first, each on its own line with detail.
	for _, c := range score.Checks {
		if c.Outcome == core.CheckFail {
			if c.Detail != "" {
				fmt.Fprintf(&b, "\n  ✗ %s: %s", c.Name, c.Detail)
			} else {
				fmt.Fprintf(&b, "\n  ✗ %s", c.Name)
			}
		}
	}

	// Passing checks summarised on one line.
	var passing []string
	for _, c := range score.Checks {
		if c.Outcome == core.CheckPass {
			passing = append(passing, c.Name)
		}
	}
	if len(passing) > 0 {
		fmt.Fprintf(&b, "\n  ✓ %s", strings.Join(passing, "  ✓ "))
	}

	// Unpinned refs found inside the dependency's own workflows.
	const maxUnpinned = 5
	if len(score.Unpinned) > 0 {
		fmt.Fprintf(&b, "\n  unpinned in upstream workflows:")
		for i, u := range score.Unpinned {
			if i >= maxUnpinned {
				fmt.Fprintf(&b, "\n    … and %d more", len(score.Unpinned)-maxUnpinned)
				break
			}
			fmt.Fprintf(&b, "\n    %s", u)
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
	case base == "Dockerfile" || strings.HasPrefix(base, "Dockerfile."):
		return core.ManifestDockerfile, true
	case base == ".gitlab-ci.yml" || strings.HasSuffix(base, ".gitlab-ci.yml"):
		return core.ManifestGitLab, true
	case base == "docker-compose.yml" || base == "docker-compose.yaml" ||
		base == "compose.yml" || base == "compose.yaml":
		return core.ManifestCompose, true
	case strings.HasSuffix(base, ".tf"):
		return core.ManifestTerraform, true
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
