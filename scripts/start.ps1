$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install Docker Desktop with Linux containers, then run this command again.' }
& (Join-Path $PSScriptRoot 'setup.ps1')
docker compose --project-directory $projectRoot up --build -d
if ($LASTEXITCODE -ne 0) { throw 'Docker Compose failed; inspect its output above.' }
Write-Host 'NEXCODE is available at the APP_URL configured in .env (default http://localhost:3000).'
