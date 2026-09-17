<#
.SYNOPSIS
  Put ANGLE's libraries beside a Windows build.

.DESCRIPTION
  The Windows build renders through Direct3D rather than OpenGL: it is
  compiled with the toolkit's OpenGL ES path (-tags gles) and asks for its
  graphics context through EGL, which ANGLE answers by translating to
  Direct3D. Windows guarantees Direct3D even with no graphics driver, through
  its own software rasteriser, so the app runs on a virtual machine and uses
  the real card on a real one. Without these libraries the window is never
  created at all, so they ship in the Store package AND in the download zip.

    pwsh scripts/fetch-angle.ps1 -Dest cmd\desktop

  ANGLE publishes no binaries of its own, so this is a reproducible
  third-party build, pinned and checked by hash the way the AppImage tools
  are. The licence travels with them (windows/ANGLE-LICENSE.txt).
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Dest
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

$tag = '2026-09-13'
$url = "https://github.com/mmozeiko/build-angle/releases/download/$tag/angle-x64-$tag.zip"
$sha = '9855b571e0cd1e17631a8c96a016ae6d266b2c74fd6474d0c376aa28f2b9bb80'
# libGLESv1_CM.dll is the old fixed-function ES 1.x library; nothing here asks
# for it. d3dcompiler_47 is what ANGLE translates shaders with.
$want = @('libEGL.dll', 'libGLESv2.dll', 'd3dcompiler_47.dll')

$work = Join-Path ([System.IO.Path]::GetTempPath()) ("angle-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $work | Out-Null
try {
  $zip = Join-Path $work 'angle.zip'
  Invoke-WebRequest $url -OutFile $zip
  $got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
  if ($got -ne $sha) { throw "ANGLE $tag hash $got, expected $sha" }
  Expand-Archive $zip -DestinationPath (Join-Path $work 'x') -Force
  New-Item -ItemType Directory -Force $Dest | Out-Null
  foreach ($name in $want) {
    $f = Get-ChildItem -Recurse (Join-Path $work 'x') -Filter $name | Select-Object -First 1
    if (-not $f) { throw "the pinned ANGLE build has no $name" }
    Copy-Item $f.FullName (Join-Path $Dest $name) -Force
  }
  Copy-Item (Join-Path $root 'windows\ANGLE-LICENSE.txt') (Join-Path $Dest 'ANGLE-LICENSE.txt') -Force
  $bytes = (Get-ChildItem $Dest -Include $want -Recurse | Measure-Object -Property Length -Sum).Sum
  Write-Host "ANGLE $tag in ${Dest}: $([math]::Round($bytes / 1MB, 1)) MB"
} finally {
  Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
