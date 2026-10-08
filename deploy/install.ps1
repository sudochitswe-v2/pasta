# Pasta Windows installer: registers pasta.exe to run silently at logon via
# the HKCU Run key. Run in PowerShell:
#   powershell -ExecutionPolicy Bypass -File deploy\install.ps1
param(
  [string]$BinaryPath = "$PSScriptRoot\..\bin\pasta.exe",
  [string]$CliBinaryPath = "$PSScriptRoot\..\bin\pasta-cli.exe"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BinaryPath)) {
  Write-Error "Binary not found at $BinaryPath (run 'make build-windows' first)."
}

# Stop existing process to avoid file lock
Stop-Process -Name pasta -ErrorAction SilentlyContinue | Out-Null
Start-Sleep -Seconds 1

$destDir = Join-Path $env:LOCALAPPDATA "Pasta"
$dest = Join-Path $destDir "pasta.exe"
$cliDest = Join-Path $destDir "pasta-cli.exe"

New-Item -ItemType Directory -Force -Path $destDir | Out-Null
Copy-Item -Force $BinaryPath $dest
if (Test-Path $CliBinaryPath) {
  Copy-Item -Force $CliBinaryPath $cliDest
}
Write-Host "installed to $destDir"

# Add to User PATH if missing
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notmatch [regex]::Escape($destDir)) {
  $newPath = $userPath
  if (-not $newPath.EndsWith(";")) { $newPath += ";" }
  $newPath += $destDir
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  $env:Path = $newPath + ";" + $env:Path
  Write-Host "added $destDir to user PATH."
}

$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
New-Item -Path $runKey -Force | Out-Null
# Quoted path; --no-inject omitted so auto-paste is enabled.
New-ItemProperty -Path $runKey -Name "Pasta" -Value "`"$dest`"" -PropertyType String -Force | Out-Null
Write-Host "registered HKCU Run key 'Pasta' (starts silently at logon)."

Write-Host ""
Write-Host "NOTE (UIPI): auto-paste is blocked into elevated (Administrator)"
Write-Host "windows unless pasta also runs elevated. Clipboard sync still works;"
Write-Host "paste manually with Ctrl+V in that case."
Write-Host ""
Write-Host "NOTE: pasta.exe is built with -H=windowsgui so no console window appears."
Write-Host "      Use pasta-cli.exe in your terminal if you need console output."

Write-Host "Starting pasta background process..."
Start-Process $dest
Write-Host "Process started successfully."
