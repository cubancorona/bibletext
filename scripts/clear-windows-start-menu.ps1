<#
.SYNOPSIS
  Dismiss only the known same-session Windows Start startup surface.
.DESCRIPTION
  Unknown identities and cloaked surfaces receive no input. A capture caller
  can share its existing deadline; this routine never extends that deadline.
#>
function Initialize-WindowsStartMenuSupport {
  if ('CIStartup.Native' -as [type]) { return }
  Add-Type -Namespace CIStartup -Name Native -MemberDefinition @'
  [System.Runtime.InteropServices.DllImport("user32.dll")]
  public static extern System.IntPtr GetForegroundWindow();
  [System.Runtime.InteropServices.DllImport("user32.dll")]
  public static extern uint GetWindowThreadProcessId(System.IntPtr window, out uint processId);
  [System.Runtime.InteropServices.DllImport("user32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
  public static extern System.IntPtr FindWindow(string className, string title);
  [System.Runtime.InteropServices.DllImport("user32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
  public static extern int GetClassName(System.IntPtr window, System.Text.StringBuilder text, int capacity);
  [System.Runtime.InteropServices.DllImport("user32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
  public static extern int GetWindowText(System.IntPtr window, System.Text.StringBuilder text, int capacity);
  [System.Runtime.InteropServices.DllImport("user32.dll")]
  public static extern bool IsWindowVisible(System.IntPtr window);
  [System.Runtime.InteropServices.DllImport("dwmapi.dll")]
  public static extern int DwmGetWindowAttribute(System.IntPtr window, uint attribute, out uint value, int size);
  [System.Runtime.InteropServices.DllImport("user32.dll", SetLastError = true)]
  private static extern uint SendInput(uint count, INPUT[] inputs, int size);
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)]
  private struct INPUT { public uint type; public INPUTUNION data; }
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Explicit)]
  private struct INPUTUNION {
    [System.Runtime.InteropServices.FieldOffset(0)] public MOUSEINPUT mouse;
    [System.Runtime.InteropServices.FieldOffset(0)] public KEYBDINPUT key;
  }
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)]
  private struct MOUSEINPUT { public int x, y; public uint data, flags, time; public System.UIntPtr extra; }
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)]
  private struct KEYBDINPUT { public ushort key, scan; public uint flags, time; public System.UIntPtr extra; }
  public static uint EscapeIfForeground(System.IntPtr expected, System.IntPtr start) {
    INPUT[] inputs = new INPUT[2];
    inputs[0].type = inputs[1].type = 1; // INPUT_KEYBOARD.
    inputs[0].data.key.key = inputs[1].data.key.key = 0x1B; // VK_ESCAPE.
    inputs[1].data.key.flags = 2; // KEYEVENTF_KEYUP.
    uint cloaked;
    if (!IsWindowVisible(start) || DwmGetWindowAttribute(start, 14, out cloaked, 4) != 0 || cloaked != 0)
      throw new System.InvalidOperationException("Start visibility changed before Escape; no input sent");
    if (GetForegroundWindow() != expected)
      throw new System.InvalidOperationException("Startup foreground changed before Escape; no input sent");
    return SendInput(2, inputs, System.Runtime.InteropServices.Marshal.SizeOf(typeof(INPUT)));
  }
'@
}

function Read-StartupWindow([string]$label, [IntPtr]$window) {
  [uint32]$windowId = 0
  [void][CIStartup.Native]::GetWindowThreadProcessId($window, [ref]$windowId)
  $process = if ($windowId) { Get-Process -Id $windowId -ErrorAction SilentlyContinue }
  $name = if ($process) { $process.ProcessName } else { 'none' }
  $path = if ($process) { $process.Path } else { 'none' }
  $session = if ($process) { $process.SessionId } else { -1 }
  $class = [System.Text.StringBuilder]::new(256)
  $title = [System.Text.StringBuilder]::new(256)
  [void][CIStartup.Native]::GetClassName($window, $class, $class.Capacity)
  [void][CIStartup.Native]::GetWindowText($window, $title, $title.Capacity)
  [uint32]$cloaked = 0
  $cloakResult = [CIStartup.Native]::DwmGetWindowAttribute($window, 14, [ref]$cloaked, 4) # DWMWA_CLOAKED.
  $visible = [CIStartup.Native]::IsWindowVisible($window)
  Write-Host "ARM startup $label HWND=$window PID=$windowId process=$name path=$path session=$session class=$class title=$title visible=$visible cloakResult=$cloakResult cloaked=$cloaked"
  return @{ Window=$window; Id=$windowId; Name=$name; Path=$path; Session=$session;
    Class=$class.ToString(); Title=$title.ToString(); Visible=$visible; CloakResult=$cloakResult; Cloaked=$cloaked }
}

function Clear-WindowsStartMenu {
  [CmdletBinding()]
  param([DateTime]$Deadline = [DateTime]::MaxValue)
  $jobSession = (Get-Process -Id $PID).SessionId
  $startPath = Join-Path $env:WINDIR 'SystemApps\Microsoft.Windows.StartMenuExperienceHost_cw5n1h2txyewy\StartMenuExperienceHost.exe'
  # Microsoft documents this exact Windows 11 SearchHost package path:
  # https://learn.microsoft.com/windows/privacy/manage-connections-from-windows-operating-system-components-to-microsoft-services#21-cortana-and-search-group-policies
  $searchPath = Join-Path $env:WINDIR 'SystemApps\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\SearchHost.exe'
  # Start can remain shell-cloaked during runner startup, then become
  # visible. Observe only the exact system identities for a bounded
  # period; shell-cloaked Start never qualifies for Escape input.
  $observeDeadline = [DateTime]::UtcNow.AddSeconds(30)
  if ($Deadline -lt $observeDeadline) { $observeDeadline = $Deadline }
  do {
    $start = Read-StartupWindow 'Start' ([CIStartup.Native]::FindWindow('Windows.UI.Core.CoreWindow', 'Start'))
    $focus = Read-StartupWindow 'foreground' ([CIStartup.Native]::GetForegroundWindow())
    $canonicalStart = $start.Name -eq 'StartMenuExperienceHost' -and $start.Session -eq $jobSession -and
      [string]::Equals($start.Path, $startPath, [StringComparison]::OrdinalIgnoreCase) -and
      $start.Class -ceq 'Windows.UI.Core.CoreWindow' -and $start.Title -ceq 'Start' -and
      $start.Visible -and $start.CloakResult -eq 0
    $knownStart = $canonicalStart -and $start.Cloaked -eq 0
    $focusIsStart = $focus.Window -eq $start.Window -and $focus.Id -eq $start.Id -and
      $focus.Class -ceq 'Windows.UI.Core.CoreWindow' -and $focus.Title -ceq 'Start' -and
      $focus.Visible -and $focus.CloakResult -eq 0 -and $focus.Cloaked -eq 0
    $waitingFocusIsStart = $focus.Window -eq $start.Window -and $focus.Id -eq $start.Id -and
      $focus.Class -ceq 'Windows.UI.Core.CoreWindow' -and $focus.Title -ceq 'Start' -and
      $focus.Visible -and $focus.CloakResult -eq 0 -and $focus.Cloaked -eq 2
    # This image reports both visible and shell-cloaked Search as
    # foreground. Permit only the exact same-session system identity.
    $focusIsSearch = $focus.Name -eq 'SearchHost' -and $focus.Session -eq $jobSession -and
      [string]::Equals($focus.Path, $searchPath, [StringComparison]::OrdinalIgnoreCase) -and
      $focus.Class -ceq 'Windows.UI.Core.CoreWindow' -and $focus.Title -ceq 'Search' -and
      $focus.Visible -and $focus.CloakResult -eq 0 -and $focus.Cloaked -in @(0, 2)
    $awaitUncloaking = $canonicalStart -and $start.Cloaked -eq 2 -and
      ($waitingFocusIsStart -or $focusIsSearch -or $focus.Window -eq [IntPtr]::Zero) -and [DateTime]::UtcNow -lt $observeDeadline
    if ($awaitUncloaking) { Start-Sleep -Milliseconds 200 }
  } while ($awaitUncloaking)
  if ($knownStart -and ($focusIsStart -or $focusIsSearch)) {
    Write-Host "known Start signature: session=$jobSession path=$($start.Path); verified focus Start=$focusIsStart Search=$focusIsSearch; sending Escape to foreground HWND=$($focus.Window)"
    if ([DateTime]::UtcNow -ge $Deadline) { throw 'Start preparation exceeded the capture deadline; no input sent' }
    $sent = [CIStartup.Native]::EscapeIfForeground($focus.Window, $start.Window)
    if ($sent -ne 2) { throw "Start Escape input failed: inserted $sent of 2 events" }
    $startDeadline = [DateTime]::UtcNow.AddSeconds(3)
    if ($Deadline -lt $startDeadline) { $startDeadline = $Deadline }
    [uint32]$startCloaked = 0
    do {
      Start-Sleep -Milliseconds 100
      $startVisible = [CIStartup.Native]::IsWindowVisible($start.Window)
      $cloakResult = [CIStartup.Native]::DwmGetWindowAttribute($start.Window, 14, [ref]$startCloaked, 4)
      $dismissed = -not $startVisible -or ($cloakResult -eq 0 -and $startCloaked -ne 0)
    } while (-not $dismissed -and [DateTime]::UtcNow -lt $startDeadline)
    Write-Host "Start Escape outcome: HWND=$($start.Window) dismissed=$dismissed visible=$startVisible cloakResult=$cloakResult cloaked=$startCloaked foreground=$([CIStartup.Native]::GetForegroundWindow())"
    if (-not $dismissed) { throw 'known Start menu did not dismiss within the startup deadline' }
  } else {
    Write-Host "Start/focus signature mismatch: knownStart=$knownStart focusIsStart=$focusIsStart focusIsSearch=$focusIsSearch; no key input sent"
  }
}
