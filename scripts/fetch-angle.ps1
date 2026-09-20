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

    pwsh scripts/fetch-angle.ps1 -Dest cmd\bibletext [-Arch x64|arm64]

  ANGLE publishes no binaries of its own, so this is a reproducible
  third-party build, pinned and checked by hash the way the AppImage tools
  are. The licence travels with them (windows/ANGLE-LICENSE.txt).
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Dest,
  [ValidateSet('x64', 'arm64')][string]$Arch = 'x64'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

$tag = '2026-09-13'
# One pinned hash per architecture. Windows on ARM runs x64 binaries under
# emulation, so an x64 ANGLE beside an arm64 executable would LOAD and the app
# would appear to work while translating every draw call twice; the hash is
# what makes the mismatch impossible rather than merely unlikely.
$shas = @{
  'x64'   = '9855b571e0cd1e17631a8c96a016ae6d266b2c74fd6474d0c376aa28f2b9bb80'
  'arm64' = 'e47c11ef8a898afd4d9a104fcdb53de0579d259df1c829ab6ee7eb9f578fa586'
}
$url = "https://github.com/mmozeiko/build-angle/releases/download/$tag/angle-$Arch-$tag.zip"
$sha = $shas[$Arch]
# libGLESv1_CM.dll is the old fixed-function ES 1.x library; nothing here asks
# for it. d3dcompiler_47 is what ANGLE translates shaders with.
$want = @('libEGL.dll', 'libGLESv2.dll', 'd3dcompiler_47.dll')

$work = Join-Path ([System.IO.Path]::GetTempPath()) ("angle-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $work | Out-Null
try {
  $zip = Join-Path $work 'angle.zip'
  Invoke-WebRequest $url -OutFile $zip
  $got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
  if ($got -ne $sha) { throw "ANGLE $Arch $tag hash $got, expected $sha" }
  Expand-Archive $zip -DestinationPath (Join-Path $work 'x') -Force
  New-Item -ItemType Directory -Force $Dest | Out-Null
  foreach ($name in $want) {
    $f = Get-ChildItem -Recurse (Join-Path $work 'x') -Filter $name | Select-Object -First 1
    if (-not $f) { throw "the pinned ANGLE build has no $name" }
    Copy-Item $f.FullName (Join-Path $Dest $name) -Force
  }
  Copy-Item (Join-Path $root 'windows\ANGLE-LICENSE.txt') (Join-Path $Dest 'ANGLE-LICENSE.txt') -Force
  $bytes = (Get-ChildItem $Dest -Include $want -Recurse | Measure-Object -Property Length -Sum).Sum
  Write-Host "ANGLE $Arch $tag in ${Dest}: $([math]::Round($bytes / 1MB, 1)) MB"
} finally {
  Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
