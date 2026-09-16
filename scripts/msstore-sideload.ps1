#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Install the Store package on a Windows machine of your own, for testing
  before the first submission.

.DESCRIPTION
  The Store signs the real package; the workflow's artifact
  (BibleText-Windows-x64-msix from .github/workflows/msstore.yml) is
  unsigned and cannot be installed as it is. This does what the workflow's
  smoke step does, on your machine: signs a COPY with a throwaway
  self-signed certificate whose subject is the reserved publisher
  (msstore/identity.json), trusts its public half machine-wide, installs the
  copy with Add-AppxPackage, and discards the private key. The original
  artifact is untouched.

  Run from PowerShell 7 started with "Run as administrator" under YOUR OWN
  account (elevating with another account's credentials installs the app
  for that account), in the repository root, with the Windows SDK installed
  (makeappx, signtool). Developer Mode or sideloading must be enabled once
  under Settings > For developers.

    pwsh scripts/msstore-sideload.ps1 -Package .\BibleText-Windows-x64.msix
    pwsh scripts/msstore-sideload.ps1 -Package .\BibleText-Windows-x64.msix -Mesa .\mesa\x64
    pwsh scripts/msstore-sideload.ps1 -Uninstall

  -Mesa <dir> copies llvmpipe's DLLs beside the exe inside the copy, for a
  virtual machine with no OpenGL 2.0 driver (the workflow fetches them from
  pal1000/mesa-dist-win, release-msvc, folder x64). Never ship that copy.

  Uninstalling removes the package and the throwaway certificate from the
  stores; the app's data under
  %LocalAppData%\Packages\<package family name>\LocalCache\ goes with the
  package.

.NOTES
  A sideloaded package has its web-to-app links validated at install
  without fetching the site's consent file; see "Resuming with a Windows
  machine" in docs/WINDOWS_STORE_LISTING.md for what that means for each
  check, and for the checks a Windows 11 client can settle that the server
  runner could not.
#>
[CmdletBinding()]
param(
  [string]$Package,
  [string]$Mesa,
  [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$id = Get-Content (Join-Path $root 'msstore\identity.json') | ConvertFrom-Json
$friendly = 'BibleText sideload (throwaway)'

function Sdk-Bin {
  & (Join-Path $root 'scripts\windows-sdk-bin.ps1')
}

# The throwaway certificate is self-signed with the reserved publisher as
# both subject and issuer; a Store-signed package's certificate has the
# Store's CA as issuer and can never match.
function Remove-ThrowawayCertificates {
  foreach ($store in 'Cert:\CurrentUser\My', 'Cert:\LocalMachine\TrustedPeople') {
    Get-ChildItem $store | Where-Object { $_.FriendlyName -eq $friendly -or ($_.Subject -eq $id.identityPublisher -and $_.Issuer -eq $id.identityPublisher) } |
      ForEach-Object { Remove-Item $_.PSPath -DeleteKey; Write-Host "removed $($_.Thumbprint) from $store" }
  }
}

if ($Uninstall) {
  $pkg = Get-AppxPackage -Name $id.identityName
  if ($pkg) { Remove-AppxPackage -Package $pkg.PackageFullName; Write-Host "removed $($pkg.PackageFullName)" }
  else { Write-Host "no $($id.identityName) package is installed" }
  Remove-ThrowawayCertificates
  return
}

if (-not $Package) { throw "-Package <path to BibleText-Windows-x64.msix> is required (or -Uninstall)" }
$Package = (Resolve-Path $Package).Path
$bin = Sdk-Bin
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("bibletext-sideload-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $work | Out-Null
try {
  $layout = Join-Path $work 'layout'
  $signed = Join-Path $work 'BibleText-sideload.msix'

  # Unpack, optionally add Mesa, repack: the artifact itself stays as it was.
  # The tools' own output is left on the console; it is the diagnosis when
  # they fail.
  & "$bin\makeappx.exe" unpack /p $Package /d $layout /o
  if ($LASTEXITCODE -ne 0) { throw "makeappx unpack failed" }
  if ($Mesa) {
    Copy-Item (Join-Path $Mesa '*.dll') $layout
    Write-Host "added Mesa DLLs from $Mesa (software OpenGL; this copy is for testing only)"
  }
  & "$bin\makeappx.exe" pack /d $layout /p $signed /o
  if ($LASTEXITCODE -ne 0) { throw "makeappx pack failed" }

  # A throwaway certificate whose subject is the reserved publisher: AppX
  # install requires the subject to equal Identity/Publisher exactly. Only
  # its PUBLIC half is trusted, in the machine's Trusted People store; the
  # private key signs the copy and is then deleted.
  Remove-ThrowawayCertificates
  $cert = New-SelfSignedCertificate -Type Custom -Subject $id.identityPublisher `
    -KeyUsage DigitalSignature -FriendlyName $friendly `
    -CertStoreLocation Cert:\CurrentUser\My `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3", "2.5.29.19={text}")
  $cer = Join-Path $work 'sideload.cer'
  Export-Certificate -Cert $cert -FilePath $cer | Out-Null
  Import-Certificate -FilePath $cer -CertStoreLocation Cert:\LocalMachine\TrustedPeople | Out-Null
  & "$bin\signtool.exe" sign /fd SHA256 /a /sha1 $cert.Thumbprint $signed
  if ($LASTEXITCODE -ne 0) { throw "signtool failed" }
  Remove-Item $cert.PSPath -DeleteKey

  $old = Get-AppxPackage -Name $id.identityName
  if ($old) { Remove-AppxPackage -Package $old.PackageFullName }
  Add-AppxPackage -Path $signed
  $pkg = Get-AppxPackage -Name $id.identityName
  if (-not $pkg) { throw "the package did not install" }
  Write-Host "installed $($pkg.PackageFullName)"
  Write-Host "In this PowerShell window (start = Start-Process):"
  Write-Host "  launch:  start shell:AppsFolder\$($pkg.PackageFamilyName)!BibleText"
  Write-Host "  scheme:  start bibletext://bibletext.co.uk/web/john/3/#v16"
  Write-Host "  https:   start https://bibletext.co.uk/web/john/3/#v16   (web-to-app handler; Settings > Apps > Apps for websites)"
  Write-Host "Or paste the bare URL into Win+R. The single-instance record lives at:"
  Write-Host "  $env:LocalAppData\Packages\$($pkg.PackageFamilyName)\LocalCache\Local\bibletext\"
  Write-Host "Remove everything with: pwsh scripts/msstore-sideload.ps1 -Uninstall"
} finally {
  if (Test-Path $work) { Remove-Item -Recurse -Force $work }
}
