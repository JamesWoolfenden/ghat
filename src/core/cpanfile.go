package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	CpanfileName      = "cpanfile"
	metaCPANModuleURL = "https://fastapi.metacpan.org/v1/module/"
)

// cpanRequireRE matches one requires/recommends/suggests statement. cpanfile is
// technically Perl, but real-world manifests are line-per-dep so a regex keeps
// formatting and comments intact (same trade-off swot/sift make).
//
//	$1 indent+verb  $2 quote  $3 module  $4 args before `;`  $5 trailing comment
var cpanRequireRE = regexp.MustCompile(`^(\s*(?:requires|recommends|suggests)\s+)(['"])([\w:]+)['"](.*?);(.*)$`)

type cpanDep struct {
	raw    string
	verb   string
	quote  string
	module string
	rest   string
	tail   string
}

func parseCpanfile(data string) []cpanDep {
	var deps []cpanDep
	for _, line := range strings.Split(data, "\n") {
		m := cpanRequireRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		deps = append(deps, cpanDep{raw: line, verb: m[1], quote: m[2], module: m[3], rest: m[4], tail: m[5]})
	}
	return deps
}

// rewriteCpanfile replaces each loose dep with `, '== <ver>'`. Lines that
// already pin with `==`, carry named options (`=>`), or are suppressed are
// left alone.
func rewriteCpanfile(data string, pins map[string]string) string {
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		m := cpanRequireRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if ok, reason := parseSuppression(line); ok {
			log.Info().Str("module", m[3]).Str("reason", reason).Msg("skipping suppressed cpanfile entry")
			continue
		}
		if strings.Contains(m[4], "==") || strings.Contains(m[4], "=>") {
			continue
		}
		ver, ok := pins[m[3]]
		if !ok {
			continue
		}
		lines[i] = m[1] + m[2] + m[3] + m[2] + ", '== " + ver + "';" + m[5]
	}
	return strings.Join(lines, "\n")
}

func getMetaCPANVersion(module string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, metaCPANModuleURL+module, nil)
	if err != nil {
		return "", &requestFailedError{err: err}
	}
	// fastapi.metacpan.org rejects default Go-http-client UA with 402.
	req.Header.Set("User-Agent", "ghat (https://github.com/JamesWoolfenden/ghat)")
	resp, err := client.Do(req)
	if err != nil {
		return "", &httpGetError{err: err}
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("metacpan %s: status %d: %s", module, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Version json.Number `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", &unmarshalJSONError{err: err}
	}
	if payload.Version == "" {
		return "", fmt.Errorf("metacpan %s: no version in response", module)
	}
	return payload.Version.String(), nil
}

func (myFlags *Flags) UpdateCpanfile() error {
	dir, err := filepath.Abs(myFlags.Directory)
	if err != nil {
		return &absolutePathError{directory: myFlags.Directory, err: err}
	}

	config := filepath.Join(dir, CpanfileName)
	data, err := os.ReadFile(config) // #nosec G304 -- <directory>/cpanfile, directory is user-supplied like every other command
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msgf("no %s found in %s, skipping", CpanfileName, dir)
			return nil
		}
		return &readConfigError{config: &config, err: err}
	}

	pins := map[string]string{}
	for _, d := range parseCpanfile(string(data)) {
		if _, done := pins[d.module]; done {
			continue
		}
		if ok, _ := parseSuppression(d.raw); ok {
			continue
		}
		if strings.Contains(d.rest, "==") || strings.Contains(d.rest, "=>") {
			continue
		}
		ver, err := getMetaCPANVersion(d.module)
		if err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Msgf("failed to resolve %s", d.module)
				continue
			}
			log.Info().Err(err).Msgf("failed to resolve %s", d.module)
			continue
		}
		pins[d.module] = ver
	}

	replacement := rewriteCpanfile(string(data), pins)

	printDiff(config, string(data), replacement)

	if !myFlags.DryRun {
		if err := os.WriteFile(config, []byte(replacement), FilePermissions); err != nil {
			return fmt.Errorf("failed to write %s: %w", config, err)
		}
	}
	return nil
}
