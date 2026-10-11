<#
.SYNOPSIS
  Exercise render-capture ownership with synthetic native Windows surfaces.

.DESCRIPTION
  A visible pattern must render, a visible flat client must count as blank,
  and a patterned foreign TopMost window must fail capture provenance for
  both expectations. Only fixture processes started here are stopped.
#>
[CmdletBinding()]
param(
  [ValidateSet('run', 'pattern', 'blank', 'cover', 'no-window')][string]$Mode = 'run',
  [string]$OutputDir,
  [string]$ReadyFile,
  [long]$CoveredWindow
)
$ErrorActionPreference = 'Stop'
Add-Type -Namespace Fixture -Name Native -MemberDefinition @"
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int command);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
[DllImport("user32.dll")] public static extern IntPtr GetWindow(IntPtr h, uint command);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint process);
"@

if ($Mode -ne 'run') {
  if ($Mode -eq 'no-window') {
    [IO.File]::WriteAllText($ReadyFile, 'ready')
    Start-Sleep 90
    return
  }
  Add-Type -AssemblyName System.Windows.Forms
  Add-Type -AssemblyName System.Drawing
  $form = New-Object System.Windows.Forms.Form
  $form.Text = 'Synthetic capture fixture'
  $form.StartPosition = 'Manual'
  $form.Location = New-Object System.Drawing.Point 120, 120
  $form.ClientSize = New-Object System.Drawing.Size 480, 320
  $form.FormBorderStyle = 'FixedSingle'
  $form.MaximizeBox = $false
  $form.BackColor = [System.Drawing.Color]::White
  if ($Mode -ne 'blank') {
    $form.Add_Paint({
      param($sender, $paint)
      for ($y = 0; $y -lt $sender.ClientSize.Height; $y += 32) {
        for ($x = 0; $x -lt $sender.ClientSize.Width; $x += 32) {
          $brush = if (([int]($x / 32) + [int]($y / 32)) % 2) {
            [System.Drawing.Brushes]::DarkSlateBlue
          } else { [System.Drawing.Brushes]::LightYellow }
          $paint.Graphics.FillRectangle($brush, $x, $y, 32, 32)
        }
      }
    })
  }
  $form.TopMost = $Mode -eq 'cover'
  $form.Add_Shown({
    [void][Fixture.Native]::ShowWindow($form.Handle, 5) # SW_SHOW, preserve fixture geometry.
    $form.Activate()
    $form.Refresh()
    # Keep the target foreground while the foreign TopMost surface covers it.
    # The negative case must exercise pixel ownership, not focus alone.
    if ($CoveredWindow) { [void][Fixture.Native]::SetForegroundWindow([IntPtr]$CoveredWindow) }
    [IO.File]::WriteAllText($ReadyFile, $form.Handle.ToInt64().ToString())
  })
  $timer = New-Object System.Windows.Forms.Timer
  $timer.Interval = 90000
  $timer.Add_Tick({ $form.Close() })
  $timer.Start()
  try { [System.Windows.Forms.Application]::Run($form) }
  finally { $timer.Dispose(); $form.Dispose() }
  return
}

if (-not $OutputDir) {
  $OutputDir = Join-Path ([IO.Path]::GetTempPath()) ('synthetic-render-capture-' + [Guid]::NewGuid())
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$OutputDir = (Resolve-Path $OutputDir).Path
$pwsh = (Get-Process -Id $PID).Path
$guard = Join-Path $PSScriptRoot 'assert-windows-render.ps1'
$children = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

function Start-Fixture([string]$kind, [long]$covered = 0) {
  $ready = Join-Path $OutputDir ($kind + '-' + [Guid]::NewGuid() + '.ready')
  $fixtureArgs = @('-NoProfile', '-STA', '-File', ('"' + $PSCommandPath + '"'),
    '-Mode', $kind, '-ReadyFile', ('"' + $ready + '"'))
  if ($covered) { $fixtureArgs += @('-CoveredWindow', $covered.ToString()) }
  # A hidden startup window can suppress the first native form as well.
  # Reuse the console so the visible, unowned form is the child main window.
  $process = Start-Process -FilePath $pwsh -ArgumentList $fixtureArgs -NoNewWindow -PassThru
  $children.Add($process)
  $readiness = 'ready file absent'
  for ($i = 0; $i -lt 100; $i++) {
    $process.Refresh()
    if ($process.HasExited) { throw "synthetic $kind fixture exited before readiness" }
    if (Test-Path -LiteralPath $ready) {
      $process.Refresh()
      if ($kind -eq 'no-window') { return $process }
      [long]$fixtureWindow = 0
      $readyText = Get-Content -LiteralPath $ready -Raw
      if ([long]::TryParse($readyText, [ref]$fixtureWindow) -and $fixtureWindow) {
        $window = [IntPtr]$fixtureWindow
        [uint32]$windowProcess = 0
        [void][Fixture.Native]::GetWindowThreadProcessId($window, [ref]$windowProcess)
        $visible = [Fixture.Native]::IsWindowVisible($window)
        $owner = [Fixture.Native]::GetWindow($window, 4) # GW_OWNER.
        $main = $process.MainWindowHandle
        $readiness = "PID=$($process.Id) ready HWND=$window visible=$visible window PID=$windowProcess owner HWND=$owner main HWND=$main"
        if ($visible -and $windowProcess -eq $process.Id -and $owner -eq [IntPtr]::Zero -and $main -eq $window) {
          Write-Host "synthetic $kind ready: $readiness"
          return $process
        }
      } else { $readiness = 'ready file has no complete HWND' }
    }
    Start-Sleep -Milliseconds 100
  }
  throw "synthetic $kind fixture did not become ready: $readiness"
}

function Probe-Fixture([System.Diagnostics.Process]$process, [string]$expect, [string]$name) {
  $shot = Join-Path $OutputDir ($name + '.png')
  $previousEap = $ErrorActionPreference
  $previousNative = $PSNativeCommandUseErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $PSNativeCommandUseErrorActionPreference = $false
    $output = (& $pwsh -NoProfile -File $guard -TargetProcessId $process.Id `
      -Expect $expect -WindowTimeout 1 -Settle 0 -ForegroundTimeout 2 -Shot $shot 2>&1 | Out-String)
    $code = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $previousEap
    $PSNativeCommandUseErrorActionPreference = $previousNative
  }
  Write-Host $output.Trim()
  return @{ code = $code; text = $output; shot = $shot }
}

try {
  $pattern = Start-Fixture 'pattern'
  $result = Probe-Fixture $pattern 'render' 'visible-pattern'
  if ($result.code -ne 0 -or -not (Test-Path -LiteralPath $result.shot)) {
    throw 'visible synthetic pattern did not produce a verified render capture'
  }
  $pattern.Refresh()
  $cover = Start-Fixture 'cover' ($pattern.MainWindowHandle.ToInt64())
  foreach ($expect in @('render', 'blank')) {
    $name = 'covered-' + $expect
    $stale = Join-Path $OutputDir ($name + '.png')
    [IO.File]::WriteAllText($stale, 'stale synthetic capture')
    $result = Probe-Fixture $pattern $expect $name
    if ($result.code -eq 0 -or $result.text -notmatch 'capture provenance failed' -or
        $result.text -notmatch 'client pixel' -or
        (Test-Path -LiteralPath $result.shot)) {
      throw "foreign patterned TopMost cover was not rejected for -Expect $expect"
    }
    Write-Host "foreign patterned cover rejected for -Expect $expect; no app PNG survived"
  }
  Stop-Process -Id $cover.Id, $pattern.Id -Force

  $blank = Start-Fixture 'blank'
  $result = Probe-Fixture $blank 'blank' 'visible-blank'
  if ($result.code -ne 0 -or -not (Test-Path -LiteralPath $result.shot)) {
    throw 'visible synthetic flat client did not produce a verified blank capture'
  }
  Stop-Process -Id $blank.Id -Force

  $noWindow = Start-Fixture 'no-window'
  $result = Probe-Fixture $noWindow 'blank' 'no-window'
  if ($result.code -ne 0 -or (Test-Path -LiteralPath $result.shot)) {
    throw 'synthetic no-window control changed the explicit blank behavior'
  }
  Write-Host 'render capture controls passed: visible pattern, flat client, both occluded expectations, and no window'
} finally {
  foreach ($process in $children) {
    $process.Refresh()
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
  }
  Get-ChildItem -Path $OutputDir -Filter '*.ready' | Remove-Item -Force
}
