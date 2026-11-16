#Requires -RunAsAdministrator
# Installs the system prerequisites for the existing isolated Python executor.
# Run only after approving Windows component / Docker Desktop installation.
$ErrorActionPreference = 'Stop'

if (-not (Get-Command winget.exe -ErrorAction SilentlyContinue)) {
  throw 'Windows App Installer (winget) is required. Install Docker Desktop from https://docs.docker.com/desktop/setup/install/windows-install/'
}

Write-Host 'Installing WSL without an additional Linux distribution...'
wsl.exe --install --no-distribution
if ($LASTEXITCODE -notin @(0, 3010)) {
  throw "WSL setup returned exit code $LASTEXITCODE. Resolve the displayed Windows error before continuing."
}

Write-Host 'Opening the official Docker Desktop installer. Review its installation options and terms.'
winget.exe install --id Docker.DockerDesktop --exact --source winget --interactive
if ($LASTEXITCODE -notin @(0, 3010)) {
  throw "Docker Desktop setup returned exit code $LASTEXITCODE."
}

Write-Host 'Restart Windows if requested by the installers; this script does not restart it automatically.'
Write-Host 'Then open Docker Desktop, choose Linux containers with the WSL 2 backend and wait until the engine is running.'
Write-Host 'The local NEXCODE API and frontend can stay on their existing database; only the isolated executor needs to be started.'
