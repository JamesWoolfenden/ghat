package core

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// printDiff prints a coloured diff only when before and after differ;
// otherwise logs that the file is already up to date so sweep output
// is not flooded with unchanged file bodies.
func printDiff(file, before, after string) {
	if before == after {
		log.Info().Str("file", file).Msg("already pinned")
		return
	}
	dmp := diffmatchpatch.New()
	fmt.Println(dmp.DiffPrettyText(dmp.DiffMain(before, after, false)))
}
