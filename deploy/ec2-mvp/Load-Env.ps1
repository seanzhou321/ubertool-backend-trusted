# Load-Env.ps1
# Dot-source this into other scripts to load .env files into script scope.
# Usage:
#   . "$PSScriptRoot\Load-Env.ps1"
#   Import-Env "$PSScriptRoot\config.env"

function Import-Env {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        Write-Error "Env file not found: $Path"
        exit 1
    }

    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        if ($line -match '^([^=]+)=(.*)$') {
            $name  = $matches[1].Trim()
            $value = $matches[2].Trim()
            Set-Variable -Name $name -Value $value -Scope Script
        }
    }
}
