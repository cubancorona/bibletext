<#
.SYNOPSIS
  Capture the Windows listing screenshots on a runner or a desktop.

.DESCRIPTION
  Launches the built BibleText.exe with a share link, sizes its window to a
  Store-sized client area, drives it through four scenes and saves one PNG of
  the WINDOW's client area per scene (no desktop, no frame, no evaluation
  watermark). Called by .github/workflows/windows-screenshots.yml; the
  PowerShell lives here rather than in the workflow because a here-string's
  terminator must start at column 0, which would end a YAML block.

    pwsh scripts/capture-windows-screenshots.ps1 -Exe build\shots\BibleText.exe `
      -Links build\shots\links.txt -Out build\shots\out

  -Links is the file the workflow writes with the app's own link minter: the
  first line a link carrying a note, the second a plain passage.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Exe,
  [Parameter(Mandatory = $true)][string]$Links,
  [string]$Out = 'build\shots\out',
  [int]$ClientWidth = 1600,
  [int]$ClientHeight = 960
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

Add-Type -Namespace Win32 -Name Native -MemberDefinition @"
[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int hh, bool repaint);
[DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out RECT r);
[DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h, ref POINT p);
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
[DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
[DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
public struct RECT { public int Left, Top, Right, Bottom; }
public struct POINT { public int X, Y; }
"@

function Get-ClientRect([IntPtr]$h) {
  $r = New-Object Win32.Native+RECT
  [void][Win32.Native]::GetClientRect($h, [ref]$r)
  $p = New-Object Win32.Native+POINT
  [void][Win32.Native]::ClientToScreen($h, [ref]$p)
  return @{ X = $p.X; Y = $p.Y; W = $r.Right - $r.Left; H = $r.Bottom - $r.Top }
}

function Save-Shot([string]$name, [IntPtr]$h) {
  $c = Get-ClientRect $h
  $bmp = New-Object System.Drawing.Bitmap $c.W, $c.H
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($c.X, $c.Y, 0, 0, (New-Object System.Drawing.Size($c.W, $c.H)))
  New-Item -ItemType Directory -Force $Out | Out-Null
  $bmp.Save((Join-Path (Resolve-Path $Out) "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
  Write-Host "captured $name ($($c.W)x$($c.H))"
}

# A click in CLIENT coordinates, so the scene scripts read like the screenshots.
function Invoke-Click([IntPtr]$h, [int]$cx, [int]$cy) {
  $c = Get-ClientRect $h
  [void][Win32.Native]::SetCursorPos(($c.X + $cx), ($c.Y + $cy))
  Start-Sleep -Milliseconds 250
  [Win32.Native]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)  # left down
  [Win32.Native]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)  # left up
  Start-Sleep -Milliseconds 250
}

function Wait-MainWindow($proc) {
  for ($i = 0; $i -lt 90; $i++) {
    $proc.Refresh()
    if ($proc.MainWindowHandle -ne [IntPtr]::Zero) { return $proc.MainWindowHandle }
    Start-Sleep 1
  }
  throw "BibleText showed no window within 90 s"
}

$link = Get-Content $Links
$noteLink = $link[0]
$plainLink = $link[1]
New-Item -ItemType Directory -Force $Out | Out-Null

# Scene 1: a note received inside a shared link, under its section heading.
$app = Start-Process -FilePath $Exe -ArgumentList "`"$noteLink`"" -PassThru
$h = Wait-MainWindow $app
# The client area is what the Store sees; the frame adds a little either way.
[void][Win32.Native]::MoveWindow($h, 0, 0, ($ClientWidth + 16), ($ClientHeight + 39), $true)
[void][Win32.Native]::SetForegroundWindow($h)
Start-Sleep 45
Save-Shot 'note' $h

# Scene 2: a plain passage, handed to the running instance by a second launch.
Start-Process -FilePath $Exe -ArgumentList "`"$plainLink`"" -Wait
Start-Sleep 6
[void][Win32.Native]::SetForegroundWindow($h)
Save-Shot 'reading' $h

# Scene 3: search results. The rail is at the left edge, Search its third
# entry; the field sits under the Search/Find toggle.
Invoke-Click $h 30 575
Start-Sleep 2
Invoke-Click $h 800 152
[System.Windows.Forms.SendKeys]::SendWait('light of the world{ENTER}')
Start-Sleep 10
Save-Shot 'search' $h

# Scene 4: settings, from the gear at the top right of the header.
Invoke-Click $h 30 360
Start-Sleep 2
$c = Get-ClientRect $h
Invoke-Click $h ($c.W - 33) 40
Start-Sleep 4
Save-Shot 'settings' $h

if (-not $app.HasExited) { Stop-Process -Id $app.Id -Force }
