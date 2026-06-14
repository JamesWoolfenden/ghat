package core

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
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
	ManifestDockerfile
	ManifestGitLab
	ManifestKube
	ManifestCompose
	ManifestTerraform
)

// DepRef is a single dependency reference extracted from a manifest file,
// with the 1-indexed line number of its declaration.
type DepRef struct {
	Ecosystem   string // one of the Source* constants
	Name        string // e.g. "actions/checkout", "lodash", "requests"
	Version     string // e.g. "v4", "^1.0.0", "==2.31.0"
	Line        int    // 1-indexed line of the name/source declaration
	VersionLine int    // 1-indexed line of the version attribute when separate (0 = same as Line)
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
	case ManifestDockerfile:
		return parseDockerfileManifest(content)
	case ManifestGitLab:
		return parseGitLabManifest(content)
	case ManifestKube:
		return parseKubeManifest(content)
	case ManifestCompose:
		return parseComposeManifest(content)
	case ManifestTerraform:
		return parseTerraformManifest(content)
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
	lineIdxNear := buildLineIndexNear(content)
	var cfg ConfigFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil
	}
	var refs []DepRef
	for _, r := range cfg.Repos {
		if !strings.Contains(r.Repo, "://") {
			continue
		}
		repoLine := lineIdx(r.Repo)
		refs = append(refs, DepRef{
			Ecosystem:   SourcePreCommit,
			Name:        r.Repo,
			Version:     r.Rev,
			Line:        repoLine,
			VersionLine: lineIdxNear(repoLine, r.Rev),
		})
	}
	return refs
}

const SourceCpanfile = "cpanfile"

func parseCpanfileManifest(content []byte) []DepRef {
	var refs []DepRef
	for _, d := range parseCpanfile(string(content)) {
		refs = append(refs, DepRef{
			Ecosystem: SourceCpanfile,
			Name:      d.module,
			Version:   ExtractCpanVersion(d.rest),
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

const SourceDockerfile = "dockerfile"
const SourceGitLab = "gitlab"
const SourceGitLabComponent = "gitlab-component"
const SourceKube = "kube"
const SourceCompose = "compose"

func parseDockerfileManifest(content []byte) []DepRef {
	var refs []DepRef
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(t), "FROM ") {
			continue
		}
		fields := strings.Fields(t)
		imgIdx := 1
		if len(fields) > 1 && strings.HasPrefix(fields[1], "--") {
			imgIdx = 2
		}
		if imgIdx >= len(fields) {
			continue
		}
		img := fields[imgIdx]
		if img == "scratch" {
			continue
		}
		// strip AS alias
		name, version := img, ""
		if idx := strings.Index(img, "@"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		} else if idx := strings.Index(img, ":"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		}
		refs = append(refs, DepRef{Ecosystem: SourceDockerfile, Name: name, Version: version, Line: i + 1})
	}
	return refs
}

func parseGitLabManifest(content []byte) []DepRef {
	lineIdx := buildLineIndex(content)
	var out struct {
		Include []struct {
			Component string `yaml:"component"`
			Project   string `yaml:"project"`
			Ref       string `yaml:"ref"`
		} `yaml:"include"`
	}
	var refs []DepRef
	if err := yaml.Unmarshal(content, &out); err == nil {
		for _, inc := range out.Include {
			switch {
			case inc.Component != "":
				// component: host/group/name@version — GitLab CI catalog component, not a container image.
				name, ver := inc.Component, ""
				if idx := strings.LastIndex(inc.Component, "@"); idx >= 0 {
					name, ver = inc.Component[:idx], inc.Component[idx+1:]
				}
				refs = append(refs, DepRef{Ecosystem: SourceGitLabComponent, Name: name, Version: ver, Line: lineIdx(inc.Component)})
			case inc.Project != "" && inc.Ref != "":
				refs = append(refs, DepRef{Ecosystem: SourceGitLabComponent, Name: inc.Project, Version: inc.Ref, Line: lineIdx(inc.Ref)})
			}
		}
	}
	// Container images referenced via image: (top-level or per-job).
	images, _ := extractImages(string(content))
	seen := map[string]bool{}
	for _, img := range images {
		if seen[img] {
			continue
		}
		seen[img] = true
		name, version := img, ""
		if idx := strings.Index(img, "@"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		} else if idx := strings.LastIndex(img, ":"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		}
		refs = append(refs, DepRef{Ecosystem: SourceGitLab, Name: name, Version: version, Line: lineIdx(img)})
	}
	return refs
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

// buildLineIndexNear returns a function that searches for a substring starting
// at a given 1-indexed line, scanning up to 10 lines forward before falling
// back to a full-file search. Used to locate version attributes that follow
// their source/repo declaration.
func buildLineIndexNear(content []byte) func(startLine int, sub string) int {
	lines := strings.Split(string(content), "\n")
	return func(startLine int, sub string) int {
		if sub == "" {
			return 0
		}
		start := startLine - 1
		if start < 0 {
			start = 0
		}
		end := start + 10
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i < end; i++ {
			if strings.Contains(lines[i], sub) {
				return i + 1
			}
		}
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i + 1
			}
		}
		return 0
	}
}

func parseKubeManifest(content []byte) []DepRef {
	if !hasKubeResource(string(content)) {
		return nil
	}
	images, err := extractKubeImages(string(content))
	if err != nil {
		return nil
	}
	lineIdx := buildLineIndex(content)
	seen := map[string]bool{}
	var refs []DepRef
	for _, img := range images {
		if seen[img] {
			continue
		}
		seen[img] = true
		name, version := img, ""
		if idx := strings.Index(img, "@"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		} else if idx := strings.LastIndex(img, ":"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		}
		refs = append(refs, DepRef{Ecosystem: SourceKube, Name: name, Version: version, Line: lineIdx(img)})
	}
	return refs
}

func parseComposeManifest(content []byte) []DepRef {
	images, err := extractImages(string(content))
	if err != nil {
		return nil
	}
	lineIdx := buildLineIndex(content)
	seen := map[string]bool{}
	var refs []DepRef
	for _, img := range images {
		if seen[img] {
			continue
		}
		seen[img] = true
		name, version := img, ""
		if idx := strings.Index(img, "@"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		} else if idx := strings.LastIndex(img, ":"); idx >= 0 {
			name, version = img[:idx], img[idx+1:]
		}
		refs = append(refs, DepRef{Ecosystem: SourceCompose, Name: name, Version: version, Line: lineIdx(img)})
	}
	return refs
}

func parseTerraformManifest(content []byte) []DepRef {
	lineIdx := buildLineIndex(content)
	lineIdxNear := buildLineIndexNear(content)
	inFile, diags := hclwrite.ParseConfig(content, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil
	}
	var refs []DepRef
	root := inFile.Body()
	for _, block := range root.Blocks() {
		switch block.Type() {
		case "terraform":
			for _, inner := range block.Body().Blocks() {
				if inner.Type() != "required_providers" {
					continue
				}
				for name, attr := range inner.Body().Attributes() {
					provider, err := parseProviderBlock(name, attr)
					if err != nil || provider.Source == "" {
						continue
					}
					sourceLine := lineIdx(`"` + provider.Source + `"`)
					refs = append(refs, DepRef{
						Ecosystem:   SourceTerraform,
						Name:        provider.Source,
						Version:     provider.CurrentVersion,
						Line:        sourceLine,
						VersionLine: lineIdxNear(sourceLine, `"`+provider.CurrentVersion+`"`),
					})
				}
			}
		case "module":
			source := GetStringValue(block, "source")
			if source == "" {
				continue
			}
			version := GetStringValue(block, "version")
			sourceLine := lineIdx(`"` + source + `"`)
			refs = append(refs, DepRef{
				Ecosystem:   SourceTerraform,
				Name:        source,
				Version:     version,
				Line:        sourceLine,
				VersionLine: lineIdxNear(sourceLine, `"`+version+`"`),
			})
		}
	}
	return refs
}
