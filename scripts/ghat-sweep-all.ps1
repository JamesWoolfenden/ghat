# ghat-sweep-all.ps1 — clone every non-fork repo, run ghat all, open a PR if anything changed.
param(
    [int]$Limit = 0,          # 0 = all repos; set e.g. -Limit 10 for a trial run
    [int]$Threshold = 200,    # pause when fewer than this many API calls remain
    [string]$GhatBin = "ghat",
    [string]$Branch = "ghat/pin-dependencies"
)

$ErrorActionPreference = "Continue"

$PrTitle = "chore: pin dependencies to immutable SHAs via ghat"
$PrBody  = @"
Automated dependency pinning by [ghat](https://github.com/JamesWoolfenden/ghat).

Pins GitHub Actions, pre-commit hooks, Terraform modules/providers, Dockerfiles, and Kubernetes images to SHA digests so supply chain attacks cannot silently change behaviour.
"@

$WorkDir  = Join-Path $env:TEMP "ghat-sweep-$(Get-Random)"
$ErrorLog = Join-Path $PSScriptRoot "ghat-sweep-errors.log"
$GapsLog  = Join-Path $PSScriptRoot "ghat-sweep-gaps.log"
New-Item -ItemType Directory -Path $WorkDir -Force | Out-Null
"" | Set-Content $ErrorLog
"" | Set-Content $GapsLog

$pass = 0; $fail = 0; $skipped = 0; $prs = @()

function Log-Error($repo, $reason) {
    Write-Host "  -> ERROR: $reason" -ForegroundColor Red
    "$repo`: $reason" | Add-Content $ErrorLog
    $script:fail++
}

function Wait-RateLimit {
    $remaining = gh api rate_limit --jq '.resources.core.remaining' 2>$null
    if ([int]$remaining -lt $Threshold) {
        $reset = gh api rate_limit --jq '.resources.core.reset' 2>$null
        $now   = [int][double]::Parse((Get-Date -UFormat %s))
        $wait  = [int]$reset - $now + 5
        if ($wait -gt 0) {
            Write-Host "  [rate limit: $remaining remaining — sleeping ${wait}s]" -ForegroundColor Yellow
            Start-Sleep -Seconds $wait
        }
    }
}

# Gap patterns ghat does not handle — scanned after ghat runs on each repo.
$GapPatterns = [ordered]@{
    "go install @version"    = 'go install .+@v[0-9]'
    "pip install pinned"     = 'pip install [^-].+==[0-9]'
    "npm/yarn add pinned"    = '(npm install|yarn add) .+@[0-9]'
    "apk add pinned"         = 'apk add .+=[0-9]'
    "apt-get install pinned" = 'apt-get install .+=[0-9]'
    "curl release download"  = 'curl .+releases/download'
    "wget release download"  = 'wget .+releases/download'
    "gem install versioned"  = 'gem install .+ -v [0-9]'
}

$GapExtensions = @("*.sh","*.bash","Makefile","*.mk","Dockerfile","Dockerfile.*","*.dockerfile")

function Scan-Gaps($repo, $dir) {
    foreach ($label in $GapPatterns.Keys) {
        $pat = $GapPatterns[$label]
        foreach ($ext in $GapExtensions) {
            $hits = Get-ChildItem -Recurse -Path $dir -Filter $ext -ErrorAction SilentlyContinue |
                    Where-Object { $_.FullName -notmatch '\\\.git\\' } |
                    Select-String -Pattern $pat -ErrorAction SilentlyContinue |
                    Select-Object -First 5
            foreach ($hit in $hits) {
                $short = $hit.Path.Replace($dir, "").TrimStart('\','/')
                "$repo | $label | ${short}:$($hit.LineNumber)" | Add-Content $GapsLog
            }
        }
    }
}

Write-Host "Fetching non-fork repo list..." -ForegroundColor Cyan
$repos = gh repo list --limit 1000 --json nameWithOwner,isFork `
         --jq '.[] | select(.isFork == false) | .nameWithOwner' |
         Where-Object { $_ -ne "" }

$total = $repos.Count
if ($Limit -gt 0) { Write-Host "Sample mode: processing first $Limit of $total repos" -ForegroundColor Cyan }
else              { Write-Host "Processing $total non-fork repos" -ForegroundColor Cyan }
Write-Host ""

$i = 0
foreach ($repo in $repos) {
    $i++
    if ($Limit -gt 0 -and $i -gt $Limit) {
        Write-Host "Reached sample limit of $Limit, stopping."
        break
    }

    $name = $repo.Split('/')[-1]
    $dir  = Join-Path $WorkDir $name
    Write-Host "[$i/$total] $repo"

    # skip if a ghat PR is already open
    $existing = gh pr list --repo $repo --head $Branch --json number --jq 'length' 2>$null
    if ([int]$existing -gt 0) {
        Write-Host "  -> PR already open, skipping" -ForegroundColor DarkGray
        $skipped++
        continue
    }

    gh repo clone $repo $dir -- --depth=1 --quiet 2>$null
    if (-not $?) {
        Log-Error $repo "clone failed"
        continue
    }

    $ghatOut = & $GhatBin all -d $dir --token $env:GITHUB_TOKEN --continue-on-error 2>&1
    if ($ghatOut -match "FTL") {
        $ghatErr = ($ghatOut | Where-Object { $_ -match "FTL" } | Select-Object -First 1)
        Log-Error $repo "ghat: $ghatErr"
        Remove-Item -Recurse -Force $dir
        continue
    }

    Scan-Gaps $repo $dir

    $changes = git -C $dir status --porcelain 2>$null
    if (-not $changes) {
        Write-Host "  -> already pinned" -ForegroundColor DarkGray
        $pass++
        Remove-Item -Recurse -Force $dir
        Wait-RateLimit
        continue
    }

    $defaultBranch = git -C $dir symbolic-ref --short HEAD 2>$null

    git -C $dir checkout -b $Branch 2>$null
    if (-not $?) {
        Log-Error $repo "could not create branch $Branch"
        Remove-Item -Recurse -Force $dir
        continue
    }

    git -C $dir add -A
    git -C $dir commit -m "chore: pin dependencies to immutable SHAs via ghat"

    git -C $dir push origin $Branch 2>$null
    if (-not $?) {
        Log-Error $repo "push failed (branch protection or no write access)"
        Remove-Item -Recurse -Force $dir
        continue
    }

    $prUrl = gh pr create --repo $repo --head $Branch --base $defaultBranch `
             --title $PrTitle --body $PrBody 2>$null
    if ($prUrl) {
        Write-Host "  -> PR opened: $prUrl" -ForegroundColor Green
        $prs += $prUrl
    } else {
        Log-Error $repo "PR creation failed"
    }
    $pass++
    Remove-Item -Recurse -Force $dir

    Wait-RateLimit
}

Remove-Item -Recurse -Force $WorkDir -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Done. $pass processed, $fail failed, $skipped already had open PRs." -ForegroundColor Cyan

if ($prs.Count -gt 0) {
    Write-Host "`nPRs opened:"
    $prs | ForEach-Object { Write-Host "  $_" }
}

if ((Get-Content $ErrorLog | Where-Object { $_ -ne "" }).Count -gt 0) {
    Write-Host "`nErrors (saved to $ErrorLog):" -ForegroundColor Red
    Get-Content $ErrorLog | Where-Object { $_ -ne "" } | ForEach-Object { Write-Host "  $_" }
}

if ((Get-Content $GapsLog | Where-Object { $_ -ne "" }).Count -gt 0) {
    Write-Host "`nPatterns ghat does not handle — frequency summary:" -ForegroundColor Yellow
    Get-Content $GapsLog | Where-Object { $_ -ne "" } |
        ForEach-Object { ($_ -split ' \| ')[1] } |
        Group-Object | Sort-Object Count -Descending |
        ForEach-Object { Write-Host ("  {0,4}  {1}" -f $_.Count, $_.Name) }
    Write-Host "`nFull details saved to $GapsLog"
}
