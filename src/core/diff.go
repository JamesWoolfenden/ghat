package core

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// printDiff prints a coloured diff only when before and after differ.
// When f.Silent is set (e.g. bulk org mode) diffs are suppressed and only
// the log line is emitted so output stays readable at scale.
func (f *Flags) printDiff(file, before, after string) {
	if before == after {
		if !f.Silent {
			log.Info().Str("file", file).Msg("already pinned")
		}
		return
	}
	if f.Silent {
		log.Warn().Str("file", file).Msg("updated")
		return
	}
	dmp := diffmatchpatch.New()
	fmt.Println(dmp.DiffPrettyText(dmp.DiffMain(before, after, false)))
}
