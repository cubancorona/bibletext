<#
.SYNOPSIS
  Launch a Windows build and say whether it actually DREW anything.

.DESCRIPTION
  A process that is still running proves less than it looks: a graphics
  failure can leave the process alive with no window, or with a window that
  never paints. This verifies that the app owns the visible foreground
  client area before and after capturing it, then judges its pixel content.
  A different window covering the app cannot count as app rendering.

    pwsh scripts/assert-windows-render.ps1 -Exe build\probe\BibleText.exe -Expect render
    pwsh scripts/assert-windows-render.ps1 -Exe build\probe\BibleText.exe -Expect blank -Shot control.png
    pwsh scripts/assert-windows-render.ps1 -ProcessName BibleText -Expect render

  -ProcessName or -TargetProcessId judges an app that is ALREADY running, how a
  packaged app has to be started: through the shell, so that it keeps its
  package identity. It leaves that process running.

  -Expect blank is the control: it FAILS if the app renders. A check that
  cannot fail proves nothing. A process that exits or creates no window can
  satisfy it; an existing window with unverifiable capture ownership cannot.
  -Shot contains only a verified app client capture. Failed ownership checks
  can save a separately named .diagnostic.png of the desktop for diagnosis.
#>
[CmdletBinding()]
param(
  [string]$Exe,
  [string]$ProcessName,
  [int]$TargetProcessId,
  [ValidateSet('render', 'blank')][string]$Expect = 'render',
  [string]$Shot,
  [int]$WindowTimeout = 60,
  [int]$Settle = 40,
  [ValidateRange(0, 60)][int]$ForegroundTimeout = 10,
  [double]$FlatShare = 98.0
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

Add-Type -Namespace Probe -Name Native -MemberDefinition @"
[DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out RECT r);
[DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h, ref POINT p);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
[DllImport("user32.dll")] public static extern bool IsIconic(IntPtr h);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int command);
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern IntPtr GetAncestor(IntPtr h, uint flags);
[DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT point);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint process);
public struct RECT { public int Left, Top, Right, Bottom; }
public struct POINT { public int X, Y; }

public static uint ProcessForWindow(IntPtr h) {
  uint process;
  GetWindowThreadProcessId(h, out process);
  return process;
}

private static string PointFailure(IntPtr h, uint process, int x, int y) {
  IntPtr at = WindowFromPoint(new POINT { X = x, Y = y });
  IntPtr root = GetAncestor(at, 2); // GA_ROOT, never GA_ROOTOWNER.
  uint owner = ProcessForWindow(root);
  if (root != h || owner != process)
    return "client pixel (" + x + "," + y + ") is covered by root HWND " + root + " owned by PID " + owner;
  return null;
}

public static string ValidateCapture(IntPtr h, uint process, int left, int top, int right, int bottom, out RECT client, out RECT visible) {
  client = new RECT();
  visible = new RECT();
  if (GetAncestor(h, 2) != h || ProcessForWindow(h) != process)
    return "main HWND does not belong to the selected process";
  if (!IsWindowVisible(h) || IsIconic(h))
    return "main window is hidden or minimized";
  IntPtr foreground = GetAncestor(GetForegroundWindow(), 2);
  if (foreground != h || ProcessForWindow(foreground) != process)
    return "foreground root HWND " + foreground + " is not the selected main HWND " + h;
  RECT area;
  POINT origin = new POINT();
  if (!GetClientRect(h, out area) || !ClientToScreen(h, ref origin))
    return "client geometry could not be read";
  client = new RECT { Left = origin.X, Top = origin.Y, Right = origin.X + area.Right, Bottom = origin.Y + area.Bottom };
  visible = new RECT { Left = Math.Max(client.Left, left), Top = Math.Max(client.Top, top), Right = Math.Min(client.Right, right), Bottom = Math.Min(client.Bottom, bottom) };
  if (area.Right <= 0 || area.Bottom <= 0 || visible.Right <= visible.Left || visible.Bottom <= visible.Top)
    return "client area has no visible pixels inside the monitor work area";
  // Validate every pixel sampled by the flatness test, plus the far edges.
  for (int y = visible.Top; y < visible.Bottom; y += 8)
    for (int x = visible.Left; x < visible.Right; x += 8) {
      string failure = PointFailure(h, process, x, y);
      if (failure != null) return failure;
    }
  string edge = PointFailure(h, process, visible.Right - 1, visible.Top);
  if (edge != null) return edge;
  edge = PointFailure(h, process, visible.Left, visible.Bottom - 1);
  if (edge != null) return edge;
  return PointFailure(h, process, visible.Right - 1, visible.Bottom - 1);
}
"@

if (@(@($Exe, $ProcessName, $TargetProcessId) | Where-Object { $_ }).Count -ne 1) {
  throw "give exactly one of -Exe, -ProcessName or -TargetProcessId"
}
$diagnostic = $null
if ($Shot) {
  $diagnostic = Join-Path ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Shot))) `
    ([IO.Path]::GetFileNameWithoutExtension($Shot) + '.diagnostic.png')
  Remove-Item -LiteralPath $Shot, $diagnostic -Force -ErrorAction SilentlyContinue
}
function Save-DiagnosticDesktop {
  if (-not $diagnostic) { return }
  $bitmap = $null; $graphics = $null
  try {
    $bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bitmap = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
    $bitmap.Save($diagnostic, [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Host "diagnostic desktop saved to $diagnostic; this is not an app capture"
  } catch { Write-Warning "cannot save diagnostic desktop: $_" }
  finally {
    if ($graphics) { $graphics.Dispose() }
    if ($bitmap) { $bitmap.Dispose() }
  }
}
$started = $false
$app = $null
try {
if ($Exe) {
  $app = Start-Process -FilePath $Exe -PassThru
  $started = $true
} elseif ($TargetProcessId) {
  $app = Get-Process -Id $TargetProcessId -ErrorAction SilentlyContinue
  if (-not $app) { throw "selected process $TargetProcessId is not running" }
} else {
  for ($i = 0; $i -lt $WindowTimeout; $i++) {
    $candidates = @(Get-Process -Name $ProcessName -ErrorAction SilentlyContinue)
    if ($candidates.Count -gt 1) { throw "more than one process named $ProcessName is running; select -TargetProcessId" }
    if ($candidates.Count -eq 1) { $app = $candidates[0]; break }
    Start-Sleep 1
  }
  if (-not $app) { throw "no process named $ProcessName is running" }
}
$h = [IntPtr]::Zero
for ($i = 0; $i -lt $WindowTimeout; $i++) {
  $app.Refresh()
  if ($app.HasExited) { break }
  if ($app.MainWindowHandle -ne [IntPtr]::Zero) { $h = $app.MainWindowHandle; break }
  Start-Sleep 1
}
if ($h -ne [IntPtr]::Zero) { Start-Sleep $Settle; $app.Refresh() }

$alive = -not $app.HasExited
if (-not $alive -or $h -eq [IntPtr]::Zero) {
  Write-Host "alive: $alive; window: False; rendered: False (no app window)"
  if ($Expect -eq 'render') { throw "expected the app to render; no app window exists" }
  Write-Host "did not render, as expected; no app PNG was created"
  return
}
try {
  if ([Probe.Native]::ProcessForWindow($h) -ne $app.Id -or [Probe.Native]::GetAncestor($h, 2) -ne $h) {
    throw "capture provenance failed: main HWND does not belong to the selected process"
  }
  if ([Probe.Native]::IsIconic($h)) { [void][Probe.Native]::ShowWindow($h, 9) }
  # Crop the capture, never the window: taskbar/off-screen pixels are not app
  # pixels. The complete client geometry remains part of the stability check.
  $desktop = ([System.Windows.Forms.Screen]::FromHandle($h)).WorkingArea
  $before = New-Object Probe.Native+RECT
  $visibleBefore = New-Object Probe.Native+RECT
  $deadline = [DateTime]::UtcNow.AddSeconds($ForegroundTimeout)
  do {
    [void][Probe.Native]::SetForegroundWindow($h)
    Start-Sleep -Milliseconds 100
    $failure = [Probe.Native]::ValidateCapture($h, [uint32]$app.Id,
      $desktop.Left, $desktop.Top, $desktop.Right, $desktop.Bottom, [ref]$before, [ref]$visibleBefore)
    if (-not $failure) { break }
  } while ([DateTime]::UtcNow -lt $deadline)
  if ($failure) { throw "capture provenance failed: $failure" }

  $w = $visibleBefore.Right - $visibleBefore.Left
  $ht = $visibleBefore.Bottom - $visibleBefore.Top
  $bmp = New-Object System.Drawing.Bitmap $w, $ht
  $g = $null
  try {
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($visibleBefore.Left, $visibleBefore.Top, 0, 0, (New-Object System.Drawing.Size($w, $ht)))
    $after = New-Object Probe.Native+RECT
    $visibleAfter = New-Object Probe.Native+RECT
    $failure = [Probe.Native]::ValidateCapture($h, [uint32]$app.Id,
      $desktop.Left, $desktop.Top, $desktop.Right, $desktop.Bottom, [ref]$after, [ref]$visibleAfter)
    $app.Refresh()
    if ($failure -or $app.HasExited -or $app.MainWindowHandle -ne $h -or
        $before.Left -ne $after.Left -or $before.Top -ne $after.Top -or
        $before.Right -ne $after.Right -or $before.Bottom -ne $after.Bottom -or
        $visibleBefore.Left -ne $visibleAfter.Left -or $visibleBefore.Top -ne $visibleAfter.Top -or
        $visibleBefore.Right -ne $visibleAfter.Right -or $visibleBefore.Bottom -ne $visibleAfter.Bottom) {
      throw "capture provenance failed after copying pixels: $failure; the app or client geometry changed"
    }
    if ($Shot) {
      $bmp.Save($Shot, [System.Drawing.Imaging.ImageFormat]::Png)
      Write-Host "verified app client capture: PID $($app.Id), main HWND $h; full client [$($before.Left),$($before.Top)..$($before.Right),$($before.Bottom)], visible [$($visibleBefore.Left),$($visibleBefore.Top)..$($visibleBefore.Right),$($visibleBefore.Bottom)] ${w}x${ht}; saved to $Shot"
    }
    $counts = @{}
    for ($y = 0; $y -lt $ht; $y += 8) {
      for ($x = 0; $x -lt $w; $x += 8) {
        $c = $bmp.GetPixel($x, $y).ToArgb()
        $counts[$c] = 1 + $counts[$c]
      }
    }
    $total = ($counts.Values | Measure-Object -Sum).Sum
    $top = ($counts.Values | Measure-Object -Maximum).Maximum
    $share = [math]::Round(100.0 * $top / $total, 1)
    $rendered = $share -lt $FlatShare
  } finally {
    if ($g) { $g.Dispose() }
    $bmp.Dispose()
  }
} catch {
  Save-DiagnosticDesktop
  throw
}

Write-Host "alive: $alive; window: $($h -ne [IntPtr]::Zero); commonest colour in the client area: $share%; rendered: $rendered"
if ($Expect -eq 'render') {
  if (-not $rendered) { throw "expected the app to render; it did not (alive=$alive, window=$($h -ne [IntPtr]::Zero), flat=$share%)" }
  Write-Host "rendered, as expected"
} else {
  if ($rendered) { throw "expected the app NOT to render, but it did: whatever this run was meant to prove, it proves nothing" }
  Write-Host "did not render, as expected"
}
} finally {
  # A process this script did not start belongs to the caller.
  if ($started -and $app -and -not $app.HasExited) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
}
