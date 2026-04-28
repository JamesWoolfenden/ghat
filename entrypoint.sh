#!/bin/bash
set -e

if [[ $GITHUB_ACTIONS != "true" ]]; then
  exec /usr/bin/ghat "$@"
fi

VERB="${INPUT_VERB:-sweep}"

args=("$VERB" -d "${INPUT_DIRECTORY:-.}")

if [[ "$VERB" != "audit" ]]; then
  if [[ -n "$INPUT_FILE" ]]; then
    args=("$VERB" -f "$INPUT_FILE")
  fi
  if [[ "$INPUT_DRYRUN" == "true" ]]; then
    args+=(--dryrun)
  fi
fi

echo "running: ghat ${args[*]}"
exec /usr/bin/ghat "${args[@]}"
