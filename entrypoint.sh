#!/bin/bash

# Leverage the default env variables as described in:
# https://docs.github.com/en/actions/reference/environment-variables#default-environment-variables
if [[ $GITHUB_ACTIONS != "true" ]]
then
  /usr/bin/ghat "$@"
  exit $?
fi

flags=""

echo "running command:"
echo ghat swot -f "$INPUT_FILE" "$flags"

/usr/bin/ghat swot -f "$INPUT_FILE" "$flags"
export ghat_EXIT_CODE=$?
