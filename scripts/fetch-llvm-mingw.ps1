<#
.SYNOPSIS
  Put a native aarch64 Windows C toolchain on this machine.

.DESCRIPTION
  Go's cgo needs a C compiler that targets the architecture it is building for.
  The windows-11-arm runner image ships an x86_64 mingw gcc, so an arm64 build
  hands runtime/cgo's gcc_arm64.S to an x86 assembler and dies on the first
  instruction:

      gcc_arm64.S:30: Error: no such instruction: `stp x29,x30,[sp,'

  llvm-mingw publishes a toolchain that both RUNS on an ARM64 Windows host and
  TARGETS aarch64-w64-mingw32. It is pinned and hash-checked the way ANGLE and
  the AppImage tools are, because a toolchain is as much a part of what ships as
  a library is.

    pwsh scripts/fetch-llvm-mingw.ps1 -Dest C:\toolchains

  Writes the bin directory to stdout; the caller puts it on PATH and points CC
  at aarch64-w64-mingw32-clang.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Dest
)

$ErrorActionPreference = 'Stop'

$tag = '20260908'
$name = "llvm-mingw-$tag-ucrt-aarch64"
$url = "https://github.com/mstorsjo/llvm-mingw/releases/download/$tag/$name.zip"
$sha = '7fe35f60407473c420ac72b25137f54385b0affd2c42efd507c0062fc626ab56'

$root = Join-Path $Dest $name
if (Test-Path (Join-Path $root 'bin\aarch64-w64-mingw32-clang.exe')) {
  Write-Host "llvm-mingw $tag already present"
  Write-Output (Join-Path $root 'bin')
  exit 0
}

New-Item -ItemType Directory -Force $Dest | Out-Null
$zip = Join-Path ([System.IO.Path]::GetTempPath()) "$name.zip"
Write-Host "downloading llvm-mingw $tag ..."
Invoke-WebRequest $url -OutFile $zip
$got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
if ($got -ne $sha) { throw "llvm-mingw $tag hash $got, expected $sha" }
Expand-Archive $zip -DestinationPath $Dest -Force
Remove-Item $zip -Force

$cc = Join-Path $root 'bin\aarch64-w64-mingw32-clang.exe'
if (-not (Test-Path $cc)) { throw "the pinned toolchain has no aarch64-w64-mingw32-clang" }
Write-Host "llvm-mingw $tag in $root"
Write-Output (Join-Path $root 'bin')
