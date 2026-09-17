<#
.SYNOPSIS
  Launch a Windows build and say whether it actually DREW anything.

.DESCRIPTION
  A process that is still running proves less than it looks: a graphics
  failure can leave the process alive with no window, or with a window that
  never paints. This waits for the app's main window, samples the pixels
  inside its client area, and calls it rendered only when a window exists
  and its commonest colour does not fill almost all of it.

    pwsh scripts/assert-windows-render.ps1 -Exe build\probe\BibleText.exe -Expect render
    pwsh scripts/assert-windows-render.ps1 -Exe build\probe\BibleText.exe -Expect blank -Shot control.png

  -Expect blank is the control: it FAILS if the app renders. A check that
  cannot fail proves nothing, so every use of this in a gate should have a
  counterpart that expects the opposite.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Exe,
  [ValidateSet('render', 'blank')][string]$Expect = 'render',
  [string]$Shot,
  [int]$WindowTimeout = 60,
  [int]$Settle = 40,
  [double]$FlatShare = 98.0
)

$ErrorActionPreference = 'Continue'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

Add-Type -Namespace Probe -Name Native -MemberDefinition @"
[DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out RECT r);
[DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h, ref POINT p);
public struct RECT { public int Left, Top, Right, Bottom; }
public struct POINT { public int X, Y; }
"@

$app = Start-Process -FilePath $Exe -PassThru
$h = [IntPtr]::Zero
for ($i = 0; $i -lt $WindowTimeout; $i++) {
  $app.Refresh()
  if ($app.HasExited) { break }
  if ($app.MainWindowHandle -ne [IntPtr]::Zero) { $h = $app.MainWindowHandle; break }
  Start-Sleep 1
}
if ($h -ne [IntPtr]::Zero) { Start-Sleep $Settle; $app.Refresh() }

$alive = -not $app.HasExited
$share = 100.0
$rendered = $false
if ($alive -and $h -ne [IntPtr]::Zero) {
  $r = New-Object Probe.Native+RECT
  [void][Probe.Native]::GetClientRect($h, [ref]$r)
  $p = New-Object Probe.Native+POINT
  [void][Probe.Native]::ClientToScreen($h, [ref]$p)
  $w = $r.Right - $r.Left
  $ht = $r.Bottom - $r.Top
  if ($w -gt 0 -and $ht -gt 0) {
    $bmp = New-Object System.Drawing.Bitmap $w, $ht
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($p.X, $p.Y, 0, 0, (New-Object System.Drawing.Size($w, $ht)))
    if ($Shot) { $bmp.Save($Shot, [System.Drawing.Imaging.ImageFormat]::Png) }
    $counts = @{}
    for ($y = 0; $y -lt $ht; $y += 8) {
      for ($x = 0; $x -lt $w; $x += 8) {
        $c = $bmp.GetPixel($x, $y).ToArgb()
        $counts[$c] = 1 + $counts[$c]
      }
    }
    $g.Dispose(); $bmp.Dispose()
    $total = ($counts.Values | Measure-Object -Sum).Sum
    $top = ($counts.Values | Measure-Object -Maximum).Maximum
    $share = [math]::Round(100.0 * $top / $total, 1)
    $rendered = $share -lt $FlatShare
  }
} elseif ($Shot) {
  # No window: keep the desktop as evidence of what happened instead.
  $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
  $bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
  $bmp.Save($Shot, [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
}

Write-Host "alive: $alive; window: $($h -ne [IntPtr]::Zero); commonest colour in the client area: $share%; rendered: $rendered"
if (-not $app.HasExited) { Stop-Process -Id $app.Id -Force }

if ($Expect -eq 'render') {
  if (-not $rendered) { throw "expected the app to render; it did not (alive=$alive, window=$($h -ne [IntPtr]::Zero), flat=$share%)" }
  Write-Host "rendered, as expected"
} else {
  if ($rendered) { throw "expected the app NOT to render, but it did: whatever this run was meant to prove, it proves nothing" }
  Write-Host "did not render, as expected"
}
