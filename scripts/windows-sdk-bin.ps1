# Prints the x64 tool directory of the newest Windows 10/11 SDK on this
# machine (makepri, makeappx, signtool). Throws when no SDK is installed so a
# packaging step fails on the cause, not on a missing executable later.
$ErrorActionPreference = 'Stop'
$root = "C:\Program Files (x86)\Windows Kits\10\bin"
if (-not (Test-Path $root)) { throw "no Windows SDK under $root" }
$versions = Get-ChildItem $root -Directory |
  Where-Object { $_.Name -match '^10\.\d+\.\d+\.\d+$' -and (Test-Path (Join-Path $_.FullName 'x64\makeappx.exe')) } |
  Sort-Object { [version]$_.Name } -Descending
if (-not $versions) { throw "no Windows SDK version under $root carries x64\makeappx.exe" }
Join-Path $versions[0].FullName 'x64'
