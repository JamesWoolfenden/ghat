package core

import (
	"encoding/json"
	"strings"

	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

// ManifestKind identifies the type of dependency manifest.
type ManifestKind int

const (
	ManifestGHA ManifestKind = iota
	ManifestGoMod
	ManifestNPM
	ManifestPyPI
	ManifestCargo
	ManifestGem
	ManifestPreCommit
	ManifestCpanfile
)

// DepRef is a single dependency reference extracted from a manifest file,
// with the 1-indexed line number of its declaration.
type DepRef struct {
	Ecosystem string // one of the Source* constants
	Name      string // e.g. "actions/checkout", "lodash", "requests"
	Version   string // e.g. "v4", "^1.0.0", "==2.31.0"
	Line      int    // 1-indexed line number
}

// ParseManifest parses a manifest file's raw bytes and returns the dependency
// references it contains. No network calls are made.
func ParseManifest(kind ManifestKind, content []byte) []DepRef {
	switch kind {
	case ManifestGHA:
		return parseGHAManifest(content)
	case ManifestGoMod:
		return parseGoModManifest(content)
	case ManifestNPM:
		return parseNPMManifest(content)
	case ManifestPyPI:
		return parsePyPIManifest(content)
	case ManifestCargo:
		return parseCargoManifest(content)
	case ManifestGem:
		return parseGemManifest(content)
	case ManifestPreCommit:
		return parsePreCommitManifest(content)
	case ManifestCpanfile:
		return parseCpanfileManifest(content)
	default:
		return nil
	}
}

func parseGHAManifest(content []byte) []DepRef {
	var refs []DepRef
	seen := map[string]bool{}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		m := usesRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if isSuppressed(line) {
			continue
		}
		ref := strings.TrimSpace(m[1])
		if strings.HasPrefix(ref, "./") ||
			strings.HasPrefix(ref, "docker://") ||
			strings.Contains(ref, "/.github/workflows/") {
			continue
		}
		path, ver, _ := strings.Cut(ref, "@")
		parts := strings.SplitN(path, "/", 3)
		if len(parts) < 2 {
			continue
		}
		name := parts[0] + "/" + parts[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		refs = append(refs, DepRef{Ecosystem: SourceGHA, Name: name, Version: ver, Line: i + 1})
	}
	return refs
}

func parseGoModManifest(content []byte) []DepRef {
	mf, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil
	}
	var refs []DepRef
	for _, r := range mf.Require {
		if r.Indirect {
			continue
		}
		line := 0
		if r.Syntax != nil {
			line = r.Syntax.Start.Line
		}
		refs = append(refs, DepRef{Ecosystem: SourceGo, Name: r.Mod.Path, Version: r.Mod.Version, Line: line})
	}
	return refs
}

func parseNPMManifest(content []byte) []DepRef {
	var p struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(content, &p) != nil {
		return nil
	}
	lineIdx := buildLineIndex(content)
	seen := map[string]bool{}
	var refs []DepRef
	for _, m := range []map[string]string{p.Dependencies, p.DevDependencies} {
		for name, ver := range m {
			if seen[name] {
				continue
			}
			seen[name] = true
			refs = append(refs, DepRef{
				Ecosystem: SourceNpm,
				Name:      name,
				Version:   ver,
				Line:      lineIdx(`"` + name + `"`),
			})
		}
	}
	return refs
}

func parsePyPIManifest(content []byte) []DepRef {
	var refs []DepRef
	seen := map[string]bool{}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		spec := strings.TrimSpace(line)
		if spec == "" || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "-") {
			continue
		}
		m := pypiNameRe.FindStringSubmatch(spec)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		ver := strings.TrimSpace(strings.TrimPrefix(spec, m[1]))
		refs = append(refs, DepRef{Ecosystem: SourcePypi, Name: m[1], Version: ver, Line: i + 1})
	}
	return refs
}

func parseCargoManifest(content []byte) []DepRef {
	var refs []DepRef
	in := false
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			in = t == "[dependencies]" || strings.HasSuffix(t, "dependencies]")
			continue
		}
		if !in {
			continue
		}
		m := cargoDepRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ver := ""
		if _, after, ok := strings.Cut(line, "="); ok {
			v := strings.TrimSpace(after)
			if strings.HasPrefix(v, `"`) {
				v = strings.Trim(v, `"`)
			} else if strings.HasPrefix(v, "{") {
				// table form: { version = "1.0", features = [...] }
				if qi := strings.Index(v, `"`); qi >= 0 {
					v = v[qi+1:]
					if end := strings.Index(v, `"`); end >= 0 {
						v = v[:end]
					}
				}
			}
			ver = v
		}
		refs = append(refs, DepRef{Ecosystem: SourceCargo, Name: m[1], Version: ver, Line: i + 1})
	}
	return refs
}

func parseGemManifest(content []byte) []DepRef {
	var refs []DepRef
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		m := gemRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ver := ""
		// gem 'name', '~> 1.0' — extract optional version constraint
		after := line[strings.Index(line, m[0])+len(m[0]):]
		after = strings.TrimSpace(after)
		after = strings.TrimPrefix(after, ",")
		after = strings.TrimSpace(after)
		if len(after) > 0 && (after[0] == '\'' || after[0] == '"') {
			after = after[1:]
			if end := strings.IndexByte(after, after[0]-1+1); end >= 0 {
				ver = after[:end]
			}
		}
		refs = append(refs, DepRef{Ecosystem: SourceGem, Name: m[1], Version: ver, Line: i + 1})
	}
	return refs
}

func parsePreCommitManifest(content []byte) []DepRef {
	lineIdx := buildLineIndex(content)
	var cfg ConfigFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil
	}
	var refs []DepRef
	for _, r := range cfg.Repos {
		if !strings.Contains(r.Repo, "://") {
			continue
		}
		refs = append(refs, DepRef{
			Ecosystem: SourcePreCommit,
			Name:      r.Repo,
			Version:   r.Rev,
			Line:      lineIdx(r.Repo),
		})
	}
	return refs
}

const SourceCpanfile = "cpanfile"

func parseCpanfileManifest(content []byte) []DepRef {
	var refs []DepRef
	for i, d := range parseCpanfile(string(content)) {
		_ = i
		refs = append(refs, DepRef{
			Ecosystem: SourceCpanfile,
			Name:      d.module,
			Version:   "",
			Line:      cpanfileLine(content, d.module),
		})
	}
	return refs
}

func cpanfileLine(content []byte, module string) int {
	lines := strings.Split(string(content), "\n")
	for i, l := range lines {
		if strings.Contains(l, module) {
			return i + 1
		}
	}
	return 0
}

// buildLineIndex returns a function that, given a substring, returns the
// 1-indexed line number of the first line that contains it.
func buildLineIndex(content []byte) func(sub string) int {
	lines := strings.Split(string(content), "\n")
	return func(sub string) int {
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i + 1
			}
		}
		return 0
	}
}
