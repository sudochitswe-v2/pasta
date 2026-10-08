# Pasta Windows installer: registers pasta.exe to run silently at logon via
# the HKCU Run key. Run in PowerShell:
#   powershell -ExecutionPolicy Bypass -File deploy\install.ps1
param(
  [string]$BinaryPath = "$PSScriptRoot\..\bin\pasta.exe"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BinaryPath)) {
  Write-Error "Binary not found at $BinaryPath (run 'make build-windows' first)."
}

$dest = Join-Path $env:LOCALAPPDATA "Pasta\pasta.exe"
New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
Copy-Item -Force $BinaryPath $dest
Write-Host "installed $dest"

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
