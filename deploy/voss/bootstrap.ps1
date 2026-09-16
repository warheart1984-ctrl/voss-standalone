# Voss Standalone Bootstrap
# Checks Go, runs full governance test suite, builds router

$ErrorActionPreference = 'Stop'

function Test-Go {
    try {
        $null = Get-Command go -ErrorAction Stop
        Write-Host "Go found: $(go version)"
        return $true
    } catch {
        Write-Host "Go not found. Install from https://go.dev/dl/"
        return $false
    }
}

if (-not (Test-Go)) { exit 1 }

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$goDir = "$root\router\service\go"

Push-Location $goDir
try {
    go mod tidy
    go test ./...
    go vet ./...
    go build -o router.exe .
} finally {
    Pop-Location
}

Write-Host "Bootstrap complete. Run docker compose up from $root"