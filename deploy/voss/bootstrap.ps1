# Voss Standalone Bootstrap
# Checks Go, runs full governance test suite, builds router + support service binaries,
# validates compose config, and checks tenant JSON is valid.

$ErrorActionPreference = 'Stop'

$goExe = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path $goExe)) {
    Write-Host "Go not found at $goExe. Install from https://go.dev/dl/"
    exit 1
}
Write-Host "Go found: $(& $goExe version)"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$goDir = "$root\router\service\go"

Push-Location $goDir
try {
    & $goExe mod tidy
    & $goExe test ./...
    & $goExe vet ./...
    & $goExe build -o router.exe .
    Copy-Item router.exe router.usl-gate.exe -Force
    Copy-Item router.exe router.gre-1001.exe -Force
    Copy-Item router.exe router.ledger.exe -Force
    Copy-Item router.exe router.drift.exe -Force
} finally {
    Pop-Location
}

# Validate compose config (requires docker compose, may be unavailable)
try {
    docker compose -f "$root\docker-compose.yml" config --quiet
    Write-Host "docker-compose.yml is valid"
} catch {
    Write-Host "docker compose config check skipped (docker not available)"
}

# Validate tenant JSON parses
$json = Get-Content "$root\config\tenants.production.json" -Raw | ConvertFrom-Json
Write-Host "tenants.production.json parsed successfully: $($json.tenants.Count) tenant(s)"

Write-Host "`nBootstrap complete. Run 'docker compose up -d' from $root"
