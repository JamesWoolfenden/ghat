#!/bin/bash
set -e

if [[ $GITHUB_ACTIONS != "true" ]]; then
  exec /usr/bin/ghat "$@"
fi

VERB="${INPUT_VERB:-sweep}"

args=("$VERB")

if [[ -n "$INPUT_FILE" ]]; then
  args+=(-f "$INPUT_FILE")
else
  args+=(-d "${INPUT_DIRECTORY:-.}")
fi

if [[ "$INPUT_DRYRUN" == "true" ]]; then
  args+=(--dryrun)
fi

echo "running: ghat ${args[*]}"
exec /usr/bin/ghat "${args[@]}"
