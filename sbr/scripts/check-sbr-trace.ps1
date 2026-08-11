<#
.SYNOPSIS
  Advisory check: reports which Go test functions lack an SBR-Trace annotation.

.DESCRIPTION
  Mechanical presence check only — never a judgment of whether an annotation's claim is
  accurate. That judgment (does the comment actually describe what the test asserts?) is
  speckit-sbr-audit's job, not this script's: it requires reading spec.md and the test body,
  which only the audit skill's reasoning can do. This script exists to make the *presence*
  question instant instead of something the audit skill has to notice incidentally per FR.

  Convention checked (see sbr/README.md -> "SBR-Trace test annotations"):
    // SBR-Trace: <FR-XXX|AV-ID|BUGFIX-NNN> — <one-line behavior claim>
    func TestSomething(t *testing.T) {

  The comment must sit directly above the func line (standard Go doc-comment attachment; a
  blank line in between means it isn't attached and is correctly reported as missing).

  Known limitation: this only checks top-level `func TestXxx(t *testing.T)` declarations. Most
  e2e coverage in this repo lives inside `t.Run("...")` subtests under one broad
  `Test<Service>_E2E` function per domain (see sbr/README.md's "e2e tests are a trap" note) —
  this script cannot see subtest-level annotation coverage, only whether the parent function
  has one. Treat e2e results here as a weaker signal than unit/integration.

  Advisory only: always exits 0. This never fails a build or gates a commit — it surfaces a
  gap for incremental pickup, matching the constitution's "does not reach backward" stance on
  the pre-existing suite (Principle IV).

.PARAMETER Tiers
  Directories to scan, relative to repo root. Defaults to this project's four adapter tiers.

.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File sbr/scripts/check-sbr-trace.ps1
#>

param(
    [string[]]$Tiers = @('tests/unit', 'tests/integration', 'tests/e2e', 'tests/smoke')
)

$ErrorActionPreference = 'Stop'

# Force UTF-8 console output so em-dashes in the report render correctly instead of as
# mojibake under PowerShell 5.1's default console codepage.
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
} catch {
    # Non-interactive/redirected output streams may not support this — harmless to skip.
}

$repoRoot = (Get-Item (Join-Path $PSScriptRoot '../..')).FullName
$funcPattern = '^\s*func\s+(Test[A-Za-z0-9_]*)\s*\(\s*\w+\s*\*testing\.T\s*\)'
$tracePattern = '^\s*//\s*SBR-Trace:\s*(\S+)\s*[—-]'

$results = @{}
$missing = New-Object System.Collections.Generic.List[string]
$totalFound = 0
$totalAnnotated = 0

foreach ($tier in $Tiers) {
    $tierPath = Join-Path $repoRoot $tier
    $tierFound = 0
    $tierAnnotated = 0

    if (-not (Test-Path $tierPath)) {
        $results[$tier] = [PSCustomObject]@{ Found = 0; Annotated = 0; Missing = 0; SkippedReason = 'directory not found' }
        continue
    }

    $goFiles = Get-ChildItem -Path $tierPath -Recurse -Filter '*_test.go' -File -ErrorAction SilentlyContinue
    foreach ($file in $goFiles) {
        $lines = [System.IO.File]::ReadAllLines($file.FullName)
        for ($i = 0; $i -lt $lines.Length; $i++) {
            if ($lines[$i] -match $funcPattern) {
                $funcName = $Matches[1]
                $tierFound++
                $totalFound++

                # Walk upward through the contiguous `//` comment block directly above, if any.
                $j = $i - 1
                $commentBlock = New-Object System.Collections.Generic.List[string]
                while ($j -ge 0 -and $lines[$j].TrimStart().StartsWith('//')) {
                    $commentBlock.Insert(0, $lines[$j])
                    $j--
                }

                $annotated = $false
                foreach ($commentLine in $commentBlock) {
                    if ($commentLine -match $tracePattern) {
                        $annotated = $true
                        break
                    }
                }

                if ($annotated) {
                    $tierAnnotated++
                    $totalAnnotated++
                } else {
                    $relPath = $file.FullName.Substring($repoRoot.Length + 1) -replace '\\', '/'
                    $missing.Add("$relPath`:$($i + 1) $funcName")
                }
            }
        }
    }

    $results[$tier] = [PSCustomObject]@{
        Found      = $tierFound
        Annotated  = $tierAnnotated
        Missing    = $tierFound - $tierAnnotated
        SkippedReason = $null
    }
}

Write-Host ''
Write-Host 'SBR-Trace annotation coverage (advisory - presence only, not accuracy)' -ForegroundColor Cyan
Write-Host '========================================================================'
Write-Host ''
Write-Host ("{0,-20} {1,8} {2,10} {3,8}" -f 'Tier', 'Found', 'Annotated', 'Missing')
foreach ($tier in $Tiers) {
    $r = $results[$tier]
    if ($r.SkippedReason) {
        Write-Host ("{0,-20} {1}" -f $tier, "(skipped: $($r.SkippedReason))")
    } else {
        Write-Host ("{0,-20} {1,8} {2,10} {3,8}" -f $tier, $r.Found, $r.Annotated, $r.Missing)
    }
}
Write-Host ''
if ($totalFound -eq 0) {
    Write-Host 'No test functions found under the configured tiers.'
} else {
    $pct = [math]::Round(($totalAnnotated / $totalFound) * 100, 1)
    Write-Host "Total: $totalAnnotated / $totalFound test functions annotated ($pct%)"
}

if ($missing.Count -gt 0) {
    Write-Host ''
    Write-Host "Missing SBR-Trace annotation ($($missing.Count)):" -ForegroundColor Yellow
    foreach ($m in $missing) {
        Write-Host "  $m"
    }
}

Write-Host ''
Write-Host 'Advisory only — this does not fail the build. See sbr/README.md -> "SBR-Trace test' -ForegroundColor DarkGray
Write-Host 'annotations" for the convention, and Constitution Principle IV for scope (forward-only,' -ForegroundColor DarkGray
Write-Host 'not retroactive against the pre-existing suite).' -ForegroundColor DarkGray
Write-Host ''

exit 0
