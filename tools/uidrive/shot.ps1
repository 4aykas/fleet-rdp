# Saves a PNG of a process's main window, DPI-aware, through PrintWindow, so it
# works without an interactive desktop. Prints the size in physical pixels.
#   pwsh tools/uidrive/shot.ps1 -ProcessId $p.Id -Out C:\scratch\maker.png
param([Parameter(Mandatory)][int]$ProcessId, [Parameter(Mandatory)][string]$Out)
Add-Type -AssemblyName System.Drawing
if (-not ('UidShot' -as [type])) { Add-Type @"
using System; using System.Runtime.InteropServices;
public static class UidShot {
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint f);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  public struct RECT { public int L, T, R, B; }
}
"@ }
[UidShot]::SetProcessDPIAware() | Out-Null
$h = (Get-Process -Id $ProcessId).MainWindowHandle
$r = New-Object UidShot+RECT; [UidShot]::GetWindowRect($h, [ref]$r) | Out-Null
$bmp = New-Object System.Drawing.Bitmap ($r.R - $r.L), ($r.B - $r.T)
$g = [System.Drawing.Graphics]::FromImage($bmp); $dc = $g.GetHdc()
[UidShot]::PrintWindow($h, $dc, 2) | Out-Null  # PW_RENDERFULLCONTENT
$g.ReleaseHdc($dc); $bmp.Save($Out); $g.Dispose(); $bmp.Dispose()
"$($r.R - $r.L)x$($r.B - $r.T)"
