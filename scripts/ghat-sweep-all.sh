#!/usr/bin/env bash
# ghat-sweep-all.sh — clone every non-fork repo, run ghat all, open a PR if anything changed.
set -uo pipefail

GHAT="${GHAT_BIN:-ghat}"
BRANCH="ghat/pin-dependencies"
PR_TITLE="chore: pin dependencies to immutable SHAs via ghat"
PR_BODY="Automated dependency pinning by [ghat](https://github.com/JamesWoolfenden/ghat).

Pins GitHub Actions, pre-commit hooks, Terraform modules/providers, Dockerfiles, and Kubernetes images to SHA digests so supply chain attacks cannot silently change behaviour."

WORK_DIR=$(mktemp -d)
LOG="$WORK_DIR/errors.log"
GAPS="$WORK_DIR/gaps.log"
trap 'rm -rf "$WORK_DIR"' EXIT

# scan_gaps — grep the cloned repo for version-pinning patterns ghat does not handle.
# Findings are written to GAPS as "repo | pattern | file:line".
scan_gaps() {
  local repo="$1" dir="$2"

  declare -A patterns=(
    ["go install @version"]='go install .+@v[0-9]'
    ["pip install pinned"]='pip install [^-].+==[0-9]'
    ["npm/yarn add pinned"]='(npm install|yarn add) .+@[0-9]'
    ["apk add pinned"]='apk add .+=[0-9]'
    ["apt-get install pinned"]='apt-get install .+=[0-9]'
    ["curl release download"]='curl .+releases/download'
    ["wget release download"]='wget .+releases/download'
    ["gem install versioned"]='gem install .+ -v [0-9]'
  )

  for label in "${!patterns[@]}"; do
    pat="${patterns[$label]}"
    hits=$(grep -rn --include="*.sh" --include="*.bash" \
                    --include="Makefile" --include="*.mk" \
                    --include="Dockerfile*" --include="*.dockerfile" \
                    -E "$pat" "$dir" 2>/dev/null \
          | grep -v "\.git/" | head -5)
    if [[ -n "$hits" ]]; then
      while IFS= read -r hit; do
        # strip the temp dir prefix for readability
        short="${hit#"$dir"/}"
        echo "$repo | $label | $short" >> "$GAPS"
      done <<< "$hits"
    fi
  done
}

pass=0
fail=0
skipped=0
prs=()

log_error() {
  local repo="$1" reason="$2"
  echo "  → ERROR: $reason"
  echo "$repo: $reason" >> "$LOG"
  fail=$((fail + 1))
}

# wait_for_rate_limit — pause if fewer than THRESHOLD requests remain.
# Sleeps until the GitHub rate limit window resets.
THRESHOLD=200
wait_for_rate_limit() {
  local remaining reset now wait_secs
  remaining=$(gh api rate_limit --jq '.resources.core.remaining' 2>/dev/null || echo 9999)
  if [[ "$remaining" -lt "$THRESHOLD" ]]; then
    reset=$(gh api rate_limit --jq '.resources.core.reset' 2>/dev/null || echo 0)
    now=$(date +%s)
    wait_secs=$(( reset - now + 5 ))
    if [[ "$wait_secs" -gt 0 ]]; then
      echo "  ⏳ rate limit low ($remaining remaining) — sleeping ${wait_secs}s until reset"
      sleep "$wait_secs"
    fi
  fi
}

repos=$(gh repo list --limit 1000 --json nameWithOwner,isFork \
  --jq '.[] | select(.isFork == false) | .nameWithOwner')

total=$(echo "$repos" | wc -l)
echo "Processing $total non-fork repos..."
echo

i=0
while IFS= read -r repo; do
  i=$((i + 1))
  name="${repo##*/}"
  dir="$WORK_DIR/$name"

  echo "[$i/$total] $repo"

  # skip if a ghat PR is already open
  existing=$(gh pr list --repo "$repo" --head "$BRANCH" --json number --jq 'length' 2>/dev/null || echo 0)
  if [[ "$existing" -gt 0 ]]; then
    echo "  → PR already open, skipping"
    skipped=$((skipped + 1))
    continue
  fi

  if ! gh repo clone "$repo" "$dir" -- --depth=1 --quiet 2>/dev/null; then
    log_error "$repo" "clone failed"
    continue
  fi

  ghat_out=$("$GHAT" all -d "$dir" --token "$GITHUB_TOKEN" --continue-on-error 2>&1) || true
  if echo "$ghat_out" | grep -q "FTL"; then
    ghat_err=$(echo "$ghat_out" | grep "FTL" | head -1)
    log_error "$repo" "ghat: $ghat_err"
    rm -rf "$dir"
    continue
  fi

  scan_gaps "$repo" "$dir"

  if git -C "$dir" diff --quiet && git -C "$dir" diff --cached --quiet; then
    echo "  → already pinned"
    pass=$((pass + 1))
    rm -rf "$dir"
    continue
  fi

  default_branch=$(git -C "$dir" symbolic-ref --short HEAD)

  if ! git -C "$dir" checkout -b "$BRANCH" 2>/dev/null; then
    log_error "$repo" "could not create branch $BRANCH"
    rm -rf "$dir"
    continue
  fi

  git -C "$dir" add -A
  git -C "$dir" commit -m "chore: pin dependencies to immutable SHAs via ghat"

  if ! git -C "$dir" push origin "$BRANCH" 2>/dev/null; then
    log_error "$repo" "push failed (branch protection or no write access)"
    rm -rf "$dir"
    continue
  fi

  pr_url=$(gh pr create \
    --repo "$repo" \
    --head "$BRANCH" \
    --base "$default_branch" \
    --title "$PR_TITLE" \
    --body "$PR_BODY" 2>/dev/null) || true

  if [[ -n "$pr_url" ]]; then
    echo "  → PR opened: $pr_url"
    prs+=("$pr_url")
  else
    log_error "$repo" "PR creation failed"
  fi
  pass=$((pass + 1))
  rm -rf "$dir"

  wait_for_rate_limit

done <<< "$repos"

echo
echo "Done. $pass processed, $fail failed, $skipped already had open PRs."

if [[ ${#prs[@]} -gt 0 ]]; then
  echo
  echo "PRs opened:"
  printf '  %s\n' "${prs[@]}"
fi

if [[ -s "$LOG" ]]; then
  echo
  echo "Errors:"
  cat "$LOG"
  cp "$LOG" ./ghat-sweep-errors.log
  echo "Saved to ./ghat-sweep-errors.log"
fi

if [[ -s "$GAPS" ]]; then
  echo
  echo "Patterns ghat does not handle (potential enhancements):"
  # summarise by pattern label, sorted by frequency
  awk -F' | ' '{print $2}' "$GAPS" | sort | uniq -c | sort -rn | \
    awk '{printf "  %4d  %s\n", $1, substr($0, length($1)+2)}'
  echo
  echo "Full details:"
  cat "$GAPS"
  cp "$GAPS" ./ghat-sweep-gaps.log
  echo
  echo "Saved to ./ghat-sweep-gaps.log"
fi
