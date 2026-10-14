# Finishes a Save As dialog opened by a process: prints the suggested name, then
# saves to -Path or presses Cancel. The maker picks ZIP or a single launcher from
# the extension in -Path (.zip or .exe). -Path must be absolute:
# a bare name lands in the dialog's last-used folder, which is a real user
# folder (test launchers with test passwords ended up there on 2026-10-14).
#   pwsh tools/uidrive/saveas.ps1 -ProcessId $p.Id -Path C:\scratch\anna.zip
#   pwsh tools/uidrive/saveas.ps1 -ProcessId $p.Id -Cancel
param([Parameter(Mandatory)][int]$ProcessId, [string]$Path, [switch]$Cancel)
if (-not $Cancel -and -not [System.IO.Path]::IsPathFullyQualified($Path)) { throw "-Path must be absolute: '$Path'" }
if (-not ('UidSave' -as [type])) { Add-Type @"
using System; using System.Collections.Generic; using System.Runtime.InteropServices; using System.Text;
public static class UidSave {
  public delegate bool EnumProc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc f, IntPtr l);
  [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr p, EnumProc f, IntPtr l);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr h);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr h, uint m, IntPtr w, string l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr h, uint m, IntPtr w, StringBuilder l);
  [DllImport("user32.dll")] public static extern IntPtr SendMessageW(IntPtr h, uint m, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern bool PostMessageW(IntPtr h, uint m, IntPtr w, IntPtr l);
  public static string Cls(IntPtr h) { var s = new StringBuilder(256); GetClassNameW(h, s, 256); return s.ToString(); }
  public static string Txt(IntPtr h) { var s = new StringBuilder(1024); SendMessageW(h, 0x000D, (IntPtr)1024, s); return s.ToString(); }
  // The dialog is a top-level #32770 owned by the process; UI Automation shows it inside the main window instead.
  public static IntPtr FindDialog(uint pid) { IntPtr found = IntPtr.Zero; EnumWindows((h, l) => { uint p; GetWindowThreadProcessId(h, out p); if (p == pid && Cls(h) == "#32770") { found = h; return false; } return true; }, IntPtr.Zero); return found; }
  public static List<IntPtr> Kids(IntPtr d) { var r = new List<IntPtr>(); EnumChildWindows(d, (h, l) => { r.Add(h); return true; }, IntPtr.Zero); return r; }
}
"@ }
$d = [UidSave]::FindDialog([uint32]$ProcessId)
if ($d -eq [IntPtr]::Zero) { throw 'no Save As dialog' }
$name = $null; $save = $null; $cancelBtn = $null
foreach ($k in [UidSave]::Kids($d)) {
  $c = [UidSave]::Cls($k); $id = [UidSave]::GetDlgCtrlID($k)
  if ($c -eq 'Edit' -and $id -eq 1001) { $name = $k }
  if ($c -eq 'Button' -and $id -eq 1) { $save = $k }
  if ($c -eq 'Button' -and $id -eq 2) { $cancelBtn = $k }
}
"suggested: " + [UidSave]::Txt($name)
if ($Cancel) { [UidSave]::PostMessageW($cancelBtn, 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero) | Out-Null }
else {
  # Typed as WM_CHARs: WM_SETTEXT changes the box but the dialog never hears of it
  # and saves under the suggested name in its last-used folder.
  [UidSave]::SendMessageW($name, 0x00B1, [IntPtr]::Zero, [IntPtr](-1)) | Out-Null  # EM_SETSEL all
  [UidSave]::SendMessageW($name, 0x0102, [IntPtr]8, [IntPtr]::Zero) | Out-Null  # WM_CHAR backspace clears the selection
  foreach ($ch in $Path.ToCharArray()) { [UidSave]::SendMessageW($name, 0x0102, [IntPtr][int]$ch, [IntPtr]::Zero) | Out-Null }
  if ([UidSave]::Txt($name) -ne $Path) { throw "name box reads '$([UidSave]::Txt($name))', not -Path" }
  [UidSave]::PostMessageW($save, 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero) | Out-Null
}
Start-Sleep -Milliseconds 1500
