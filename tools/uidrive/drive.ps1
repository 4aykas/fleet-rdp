# Drives a window by control ID with Win32 messages (UI Automation does not see
# the maker's controls reliably). Control IDs are the id* constants in
# cmd/maker/main_windows.go. Order: set texts, click buttons, then read texts.
#   pwsh tools/uidrive/drive.ps1 -ProcessId $p.Id -Set @{100='host:3389'; 101='anna'} -Click 112 -Read 113
# A click that opens Save As returns at once; finish it with saveas.ps1.
param([Parameter(Mandatory)][int]$ProcessId, [hashtable]$Set = @{}, [int[]]$Click = @(), [int[]]$Read = @())
if (-not ('UidDrive' -as [type])) { Add-Type @"
using System; using System.Runtime.InteropServices; using System.Text;
public static class UidDrive {
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr h, uint m, IntPtr w, string l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessageW(IntPtr h, uint m, IntPtr w, StringBuilder l);
  [DllImport("user32.dll")] public static extern bool PostMessageW(IntPtr h, uint m, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h, int id);
}
"@ }
$h = (Get-Process -Id $ProcessId).MainWindowHandle
foreach ($id in $Set.Keys) { [UidDrive]::SendMessageW([UidDrive]::GetDlgItem($h, [int]$id), 0x000C, [IntPtr]::Zero, [string]$Set[$id]) | Out-Null }  # WM_SETTEXT
foreach ($id in $Click) {
  # BM_CLICK is posted, not sent, so a click that opens a modal dialog does not block here.
  [UidDrive]::PostMessageW([UidDrive]::GetDlgItem($h, $id), 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero) | Out-Null
  Start-Sleep -Milliseconds 700
}
foreach ($id in $Read) {
  $sb = New-Object System.Text.StringBuilder 1024
  [UidDrive]::SendMessageW([UidDrive]::GetDlgItem($h, $id), 0x000D, [IntPtr]1024, $sb) | Out-Null  # WM_GETTEXT
  "${id}: $sb"
}
