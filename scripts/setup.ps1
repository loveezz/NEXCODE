param([string]$AdminLogin = 'admin')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $projectRoot '.env'
function New-NexcodeSecret {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
  return ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
}
function Get-NexcodePasswordHash([string]$ClearText) {
  $previousEncoding = $OutputEncoding
  $OutputEncoding = New-Object System.Text.UTF8Encoding($false)
  try {
    if (Get-Command go -ErrorAction SilentlyContinue) {
      Push-Location (Join-Path $projectRoot 'backend')
      try { $hashResult = $ClearText | go run ./cmd/hash-password; if ($LASTEXITCODE -ne 0) { throw 'Password hashing failed.' } } finally { Pop-Location }
    } else {
      if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Go or Docker is required to generate a bcrypt hash.' }
      docker build --target api -t nexcode-api:local (Join-Path $projectRoot 'backend') | Out-Host
      if ($LASTEXITCODE -ne 0) { throw 'Could not build the password hashing helper.' }
      $hashResult = $ClearText | docker run --rm -i --entrypoint hash-password nexcode-api:local
      if ($LASTEXITCODE -ne 0) { throw 'Password hashing failed.' }
    }
    $hashValue = [string]($hashResult | Select-Object -Last 1)
    if ($hashValue -notmatch '^\$2[aby]\$\d\d\$') { throw 'Invalid bcrypt output.' }
    return $hashValue.Trim()
  } finally { $OutputEncoding = $previousEncoding }
}
if (Test-Path -LiteralPath $envPath) {
  $existing = [System.IO.File]::ReadAllText($envPath)
  $legacy = [regex]::Match($existing, '(?m)^ADMIN_PASSWORD=([^\r\n]*)')
  if ($legacy.Success) {
    $hasHash = [regex]::Match($existing, '(?m)^ADMIN_PASSWORD_HASH=([^\r\n]+)')
    if ($hasHash.Success -and $hasHash.Groups[1].Value.Contains('$2')) {
      $existing = $existing.Replace($legacy.Value, '')
    } else {
      $adminClear = $legacy.Groups[1].Value.Trim()
      if (($adminClear.StartsWith("'") -and $adminClear.EndsWith("'")) -or ($adminClear.StartsWith('"') -and $adminClear.EndsWith('"'))) { $adminClear = $adminClear.Substring(1, $adminClear.Length - 2) }
      $newHash = Get-NexcodePasswordHash $adminClear
      $existing = [regex]::Replace($existing, '(?m)^ADMIN_PASSWORD_HASH=[^\r\n]*\r?\n?', '')
      $existing = $existing.Replace($legacy.Value, "ADMIN_PASSWORD_HASH='$newHash'")
      $adminClear = $null
    }
    [System.IO.File]::WriteAllText($envPath, $existing, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host 'Migrated administrator configuration to bcrypt. The existing password still works; plaintext was removed.'
  } else { Write-Host '.env already exists; preserved.' }
  return
}
$initialPassword = New-NexcodeSecret
$passwordHash = Get-NexcodePasswordHash $initialPassword
$content = [System.IO.File]::ReadAllText((Join-Path $projectRoot '.env.example'))
$content = $content.Replace('replace-with-another-random-64-hex-secret', (New-NexcodeSecret)).Replace('replace-with-bcrypt-hash', $passwordHash).Replace('ADMIN_LOGIN=admin', "ADMIN_LOGIN=$AdminLogin")
[System.IO.File]::WriteAllText($envPath, $content, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "Created .env. Initial staff login: $AdminLogin"
Write-Host "Initial password (shown once; store it privately): $initialPassword"
Write-Host 'Only the bcrypt hash is stored in .env. Start: docker compose up --build -d'
