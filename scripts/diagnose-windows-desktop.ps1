<#
.SYNOPSIS
  Record native desktop ownership while a synthetic fixture becomes visible.

.DESCRIPTION
  Desktop PNGs are diagnostics, never application render evidence. Each PNG
  is bracketed by timestamped native window and point-owner metadata. Window
  enumeration is supplemented by FindWindow and foreground probes because
  EnumWindows does not enumerate every modern application surface. Only the
  synthetic child started here is stopped; no shell input is sent.
#>
[CmdletBinding()]
param([Parameter(Mandatory)][string]$OutputDir)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
namespace DesktopDiagnostic {
  public static class Native {
    public sealed class ThreadDesktop {
      public uint ThreadId;
      public long Handle;
      public string Name;
      public bool NameRead;
      public int Error;
    }
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)]
    public struct POINT { public int X, Y; }
    private delegate bool EnumWindowProc(IntPtr window, IntPtr parameter);
    [DllImport("user32.dll")]
    private static extern bool EnumWindows(EnumWindowProc callback, IntPtr parameter);
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern IntPtr FindWindow(string className, string title);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetClassName(IntPtr window, StringBuilder text, int capacity);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetWindowText(IntPtr window, StringBuilder text, int capacity);
    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll")]
    public static extern bool IsIconic(IntPtr window);
    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr window, out RECT rectangle);
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll")]
    public static extern IntPtr GetAncestor(IntPtr window, uint flags);
    [DllImport("user32.dll")]
    public static extern IntPtr GetWindow(IntPtr window, uint command);
    [DllImport("user32.dll")]
    private static extern IntPtr WindowFromPoint(POINT point);
    [DllImport("dwmapi.dll")]
    public static extern int DwmGetWindowAttribute(IntPtr window, uint attribute, out uint value, int size);
    [DllImport("kernel32.dll")]
    public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll", SetLastError = true)]
    private static extern IntPtr GetThreadDesktop(uint threadId);
    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool GetUserObjectInformation(IntPtr handle, int index, StringBuilder text, uint bytes, out uint needed);
    public static ThreadDesktop DesktopOfThread(uint threadId) {
      var result = new ThreadDesktop { ThreadId = threadId };
      IntPtr desktop = GetThreadDesktop(threadId);
      result.Handle = desktop.ToInt64();
      if (desktop == IntPtr.Zero) { result.Error = Marshal.GetLastWin32Error(); return result; }
      var text = new StringBuilder(256);
      uint needed;
      result.NameRead = GetUserObjectInformation(desktop, 2, text, 512, out needed); // UOI_NAME.
      if (result.NameRead) result.Name = text.ToString();
      else result.Error = Marshal.GetLastWin32Error();
      return result; // GetThreadDesktop returns a borrowed handle.
    }
    public static IntPtr AtPoint(int x, int y) {
      return WindowFromPoint(new POINT { X = x, Y = y });
    }
    public static IntPtr[] TopLevelWindows(out bool complete) {
      var windows = new List<IntPtr>();
      EnumWindowProc callback = (window, parameter) => { windows.Add(window); return true; };
      complete = EnumWindows(callback, IntPtr.Zero);
      GC.KeepAlive(callback);
      return windows.ToArray();
    }
  }
}
'@

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$OutputDir = (Resolve-Path -LiteralPath $OutputDir).Path
$jobSession = (Get-Process -Id $PID).SessionId
$startPath = Join-Path $env:WINDIR 'SystemApps\Microsoft.Windows.StartMenuExperienceHost_cw5n1h2txyewy\StartMenuExperienceHost.exe'
$searchPath = Join-Path $env:WINDIR 'SystemApps\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\SearchHost.exe'
$fixtureScript = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'test-windows-render-capture.ps1')).Path
$ready = Join-Path $OutputDir ('synthetic-' + [Guid]::NewGuid() + '.ready')
$child = $null
$fixtureWindow = [IntPtr]::Zero
$transcript = Join-Path $OutputDir 'desktop-diagnostic.log'

function Read-Window([IntPtr]$window) {
  $key = $window.ToInt64().ToString()
  if ($script:windowCache.ContainsKey($key)) { return $script:windowCache[$key] }
  [uint32]$windowId = 0
  $threadId = [DesktopDiagnostic.Native]::GetWindowThreadProcessId($window, [ref]$windowId)
  $processKey = $windowId.ToString()
  if (-not $script:processCache.ContainsKey($processKey)) {
    $process = if ($windowId) { Get-Process -Id $windowId -ErrorAction SilentlyContinue }
    $name = if ($process) { $process.ProcessName } else { $null }
    $path = $null
    $session = $null
    $processError = $null
    if ($process) {
      try { $path = $process.Path; $session = $process.SessionId }
      catch { $processError = $_.Exception.GetType().FullName }
    }
    $script:processCache[$processKey] = [ordered]@{
      id = $windowId; name = $name; path = $path; session = $session
      metadataErrorType = $processError
    }
  }
  $processRecord = $script:processCache[$processKey]
  $class = [System.Text.StringBuilder]::new(256)
  $title = [System.Text.StringBuilder]::new(512)
  [void][DesktopDiagnostic.Native]::GetClassName($window, $class, $class.Capacity)
  [void][DesktopDiagnostic.Native]::GetWindowText($window, $title, $title.Capacity)
  $rectangle = [DesktopDiagnostic.Native+RECT]::new()
  $hasRectangle = [DesktopDiagnostic.Native]::GetWindowRect($window, [ref]$rectangle)
  [uint32]$cloaked = 0
  $cloakResult = [DesktopDiagnostic.Native]::DwmGetWindowAttribute($window, 14, [ref]$cloaked, 4)
  $canonicalKind = if ($processRecord.session -eq $jobSession -and
    $processRecord.name -eq 'StartMenuExperienceHost' -and
    [string]::Equals($processRecord.path, $startPath, [StringComparison]::OrdinalIgnoreCase)) { 'Start' }
  elseif ($processRecord.session -eq $jobSession -and $processRecord.name -eq 'SearchHost' -and
    [string]::Equals($processRecord.path, $searchPath, [StringComparison]::OrdinalIgnoreCase)) { 'Search' }
  else { $null }
  $record = [ordered]@{
    observedUtc = [DateTime]::UtcNow.ToString('o'); hwnd = $window.ToInt64()
    rootHwnd = [DesktopDiagnostic.Native]::GetAncestor($window, 2).ToInt64()
    ownerHwnd = [DesktopDiagnostic.Native]::GetWindow($window, 4).ToInt64()
    process = $processRecord; canonicalSystemKind = $canonicalKind
    threadDesktop = [DesktopDiagnostic.Native]::DesktopOfThread($threadId)
    class = $class.ToString(); title = $title.ToString()
    visible = [DesktopDiagnostic.Native]::IsWindowVisible($window)
    iconic = [DesktopDiagnostic.Native]::IsIconic($window)
    hasRectangle = $hasRectangle
    rectangle = [ordered]@{ left = $rectangle.Left; top = $rectangle.Top; right = $rectangle.Right; bottom = $rectangle.Bottom }
    cloakResult = $cloakResult; cloaked = $cloaked
  }
  $script:windowCache[$key] = $record
  return $record
}

function Read-DesktopMetadata([System.Drawing.Rectangle]$bounds) {
  $started = [DateTime]::UtcNow.ToString('o')
  $script:windowCache = @{}
  $script:processCache = @{}
  $foreground = Read-Window ([DesktopDiagnostic.Native]::GetForegroundWindow())
  $findStart = Read-Window ([DesktopDiagnostic.Native]::FindWindow('Windows.UI.Core.CoreWindow', 'Start'))
  $findSearch = Read-Window ([DesktopDiagnostic.Native]::FindWindow('Windows.UI.Core.CoreWindow', 'Search'))
  [bool]$enumComplete = $false
  $topWindows = [DesktopDiagnostic.Native]::TopLevelWindows([ref]$enumComplete)
  $candidates = @(
    foreach ($window in $topWindows) {
      $record = Read-Window $window
      if ($record.process.name -in @('StartMenuExperienceHost', 'SearchHost') -or $record.canonicalSystemKind) { $record }
    }
  )
  $points = [System.Collections.Generic.List[object]]::new()
  # A full desktop grid also covers the fixed fixture and the runner's Start
  # menu placement; edge points include the taskbar without presuming its size.
  $xs = @($bounds.Left + 1; for ($x = $bounds.Left + 32; $x -lt $bounds.Right; $x += 64) { $x }; $bounds.Right - 1)
  $ys = @($bounds.Top + 1; for ($y = $bounds.Top + 32; $y -lt $bounds.Bottom; $y += 64) { $y }; $bounds.Bottom - 1)
  $ownerHandles = [System.Collections.Generic.HashSet[long]]::new()
  foreach ($y in $ys) {
    foreach ($x in $xs) {
      $pointWindow = [DesktopDiagnostic.Native]::AtPoint($x, $y)
      $root = [DesktopDiagnostic.Native]::GetAncestor($pointWindow, 2)
      $points.Add([ordered]@{
        x = $x; y = $y; observedUtc = [DateTime]::UtcNow.ToString('o')
        hwnd = $pointWindow.ToInt64(); rootHwnd = $root.ToInt64()
      })
      [void]$ownerHandles.Add($pointWindow.ToInt64())
      [void]$ownerHandles.Add($root.ToInt64())
    }
  }
  $pointOwners = @(foreach ($handle in $ownerHandles) { Read-Window ([IntPtr]$handle) })
  return [ordered]@{
    startedUtc = $started; endedUtc = [DateTime]::UtcNow.ToString('o'); jobSession = $jobSession
    scriptThreadDesktop = [DesktopDiagnostic.Native]::DesktopOfThread([DesktopDiagnostic.Native]::GetCurrentThreadId())
    foreground = $foreground; findWindowStart = $findStart; findWindowSearch = $findSearch
    fixture = Read-Window $fixtureWindow
    enumWindowsComplete = $enumComplete; enumWindowsCount = $topWindows.Count
    enumerationLimit = 'EnumWindows absence does not establish absence of modern application surfaces.'
    startSearchCandidates = $candidates; pointOwners = $pointOwners; points = $points.ToArray()
    foregroundAtEnd = [DesktopDiagnostic.Native]::GetForegroundWindow().ToInt64()
  }
}

function Save-DesktopDiagnostic([string]$label) {
  $bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
  $before = Read-DesktopMetadata $bounds
  $stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ')
  $base = 'desktop-diagnostic-' + $label + '-' + $stamp
  $shot = Join-Path $OutputDir ($base + '.png')
  $bitmap = [System.Drawing.Bitmap]::new($bounds.Width, $bounds.Height)
  $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
  try {
    $captureStarted = [DateTime]::UtcNow.ToString('o')
    $graphics.CopyFromScreen($bounds.Left, $bounds.Top, 0, 0, $bounds.Size, [System.Drawing.CopyPixelOperation]::SourceCopy)
    $captureEnded = [DateTime]::UtcNow.ToString('o')
    $after = Read-DesktopMetadata $bounds
    $bitmap.Save($shot, [System.Drawing.Imaging.ImageFormat]::Png)
  } finally { $graphics.Dispose(); $bitmap.Dispose() }
  $record = [ordered]@{
    kind = 'desktop diagnostic; not application render evidence'; label = $label; png = [IO.Path]::GetFileName($shot)
    captureStartedUtc = $captureStarted; captureEndedUtc = $captureEnded
    bounds = [ordered]@{ left = $bounds.Left; top = $bounds.Top; width = $bounds.Width; height = $bounds.Height }
    before = $before; after = $after
  }
  $record | ConvertTo-Json -Depth 14 | Set-Content -LiteralPath (Join-Path $OutputDir ($base + '.json')) -Encoding utf8
  Write-Host "desktop diagnostic $label at $captureStarted -> $captureEnded; foreground before=$($before.foreground.hwnd) after=$($after.foreground.hwnd); Start before=$($before.findWindowStart.hwnd)/cloak=$($before.findWindowStart.cloaked) after=$($after.findWindowStart.hwnd)/cloak=$($after.findWindowStart.cloaked); PNG=$base.png"
}

Start-Transcript -LiteralPath $transcript | Out-Null
try {
  Save-DesktopDiagnostic 'before-fixture'
  $pwsh = (Get-Process -Id $PID).Path
  $fixtureArgs = @('-NoProfile', '-STA', '-File', ('"' + $fixtureScript + '"'),
    '-Mode', 'pattern', '-ReadyFile', ('"' + $ready + '"'))
  $child = Start-Process -FilePath $pwsh -ArgumentList $fixtureArgs -NoNewWindow -PassThru
  $readiness = 'ready file absent'
  $isReady = $false
  for ($attempt = 0; $attempt -lt 100; $attempt++) {
    $child.Refresh()
    if ($child.HasExited) { throw 'synthetic diagnostic fixture exited before readiness' }
    if (Test-Path -LiteralPath $ready) {
      [long]$savedHandle = 0
      if ([long]::TryParse((Get-Content -LiteralPath $ready -Raw), [ref]$savedHandle) -and $savedHandle) {
        $fixtureWindow = [IntPtr]$savedHandle
        [uint32]$windowId = 0
        [void][DesktopDiagnostic.Native]::GetWindowThreadProcessId($fixtureWindow, [ref]$windowId)
        $visible = [DesktopDiagnostic.Native]::IsWindowVisible($fixtureWindow)
        $owner = [DesktopDiagnostic.Native]::GetWindow($fixtureWindow, 4)
        $child.Refresh()
        $main = $child.MainWindowHandle
        $readiness = "PID=$($child.Id) HWND=$fixtureWindow window PID=$windowId visible=$visible owner=$owner main=$main"
        if ($windowId -eq $child.Id -and $visible -and $owner -eq [IntPtr]::Zero -and $main -eq $fixtureWindow) {
          $isReady = $true
          break
        }
      }
    }
    Start-Sleep -Milliseconds 100
  }
  Write-Host "synthetic diagnostic fixture readiness: $readiness"
  Save-DesktopDiagnostic 'at-readiness'
  if (-not $isReady) { throw "synthetic diagnostic fixture did not become ready: $readiness" }
  $readyAt = [DateTime]::UtcNow
  foreach ($seconds in @(5, 10, 15)) {
    $remaining = ($readyAt.AddSeconds($seconds) - [DateTime]::UtcNow).TotalMilliseconds
    if ($remaining -gt 0) { Start-Sleep -Milliseconds ([int][Math]::Ceiling($remaining)) }
    Save-DesktopDiagnostic ('after-' + $seconds + 's')
  }
} finally {
  if ($child) {
    $child.Refresh()
    if (-not $child.HasExited) { Stop-Process -InputObject $child -Force -ErrorAction SilentlyContinue }
  }
  if (Test-Path -LiteralPath $ready) { Remove-Item -LiteralPath $ready -Force }
  Stop-Transcript | Out-Null
}
