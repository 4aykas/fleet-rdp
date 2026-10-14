//go:build windows

package main

import (
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	comdlg32 = windows.NewLazySystemDLL("comdlg32.dll")
	comctl32 = windows.NewLazySystemDLL("comctl32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pDestroyWindow            = user32.NewProc("DestroyWindow")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pIsDialogMessageW         = user32.NewProc("IsDialogMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pPostMessageW             = user32.NewProc("PostMessageW")
	pSendMessageW             = user32.NewProc("SendMessageW")
	pShowWindow               = user32.NewProc("ShowWindow")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pMoveWindow               = user32.NewProc("MoveWindow")
	pEnableWindow             = user32.NewProc("EnableWindow")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pIsWindowEnabled          = user32.NewProc("IsWindowEnabled")
	pSetFocus                 = user32.NewProc("SetFocus")
	pGetFocus                 = user32.NewProc("GetFocus")
	pSetWindowTextW           = user32.NewProc("SetWindowTextW")
	pGetWindowTextW           = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	pInvalidateRect           = user32.NewProc("InvalidateRect")
	pLoadImageW               = user32.NewProc("LoadImageW")
	pLoadCursorW              = user32.NewProc("LoadCursorW")
	pGetDpiForWindow          = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem          = user32.NewProc("GetDpiForSystem")
	pAdjustWindowRectExForDpi = user32.NewProc("AdjustWindowRectExForDpi")
	pGetSysColorBrush         = user32.NewProc("GetSysColorBrush")
	pCreateFontW              = gdi32.NewProc("CreateFontW")
	pDeleteObject             = gdi32.NewProc("DeleteObject")
	pGetSaveFileNameW         = comdlg32.NewProc("GetSaveFileNameW")
	pCommDlgExtendedError     = comdlg32.NewProc("CommDlgExtendedError")
	pInitCommonControlsEx     = comctl32.NewProc("InitCommonControlsEx")
	pGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
)

const (
	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsTabStop      = 0x00010000
	wsVScroll      = 0x00200000
	wsBorder       = 0x00800000
	wsExClientEdge = 0x00000200

	esAutoHScroll       = 0x0080
	esPassword          = 0x0020
	bsPushButton        = 0x0000
	bsDefPushButton     = 0x0001
	cbsDropDownList     = 0x0003
	lbsNotify           = 0x0001
	lbsNoIntegralHeight = 0x0100
	ssLeft              = 0x0000
	ssCenterImage       = 0x0200 // single line, centred vertically in the row
	ssEndEllipsis       = 0x4000 // SS_ENDELLIPSIS for the status line

	wmDestroy    = 0x0002
	wmActivate   = 0x0006
	wmClose      = 0x0010
	wmSetFont    = 0x0030
	wmCommand    = 0x0111
	wmDpiChanged = 0x02E0
	wmApp        = 0x8000
	wmCheckDone  = wmApp + 1
	dmGetDefID   = 0x0400 // WM_USER: IsDialogMessageW asks which button Enter presses
	dcHasDefID   = 0x534B

	waInactive = 0

	emSetPasswordChar = 0x00CC
	cbAddString       = 0x0143
	cbGetCurSel       = 0x0147
	cbSetCurSel       = 0x014E
	lbAddString       = 0x0180
	lbResetContent    = 0x0184
	lbGetCurSel       = 0x0188

	swHide        = 0
	swShow        = 5
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	cwUseDefault  = 0x80000000
	imageIcon     = 1
	lrDefaultSize = 0x0040
	idcArrow      = 32512
	colorBtnFace  = 15

	iccStandardClasses = 0x00004000

	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
	ofnPathMustExist   = 0x00000800
	ofnExplorer        = 0x00080000

	coinitApartmentThreaded = 0x2
)

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type point struct{ x, y int32 }

// msg is MSG (48 bytes on amd64).
type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type rect struct{ left, top, right, bottom int32 }

// initCommonControlsEx is INITCOMMONCONTROLSEX.
type initCommonControlsEx struct {
	size uint32
	icc  uint32
}

// openFileName is OPENFILENAMEW (Vista+ layout with pvReserved, dwReserved,
// FlagsEx; 152 bytes on amd64).
type openFileName struct {
	structSize    uint32
	owner         windows.HWND
	instance      windows.Handle
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	dwReserved    uint32
	flagsEx       uint32
}

func utf16z(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

// multiZ encodes a double-NUL-terminated list (the Save As filter).
func multiZ(parts ...string) *uint16 {
	var u []uint16
	for _, p := range parts {
		u = append(u, utf16.Encode([]rune(p))...)
		u = append(u, 0)
	}
	u = append(u, 0)
	return &u[0]
}

func send(h windows.HWND, m uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(m), w, l)
	return r
}

func getText(h windows.HWND) string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf)
}

func setText(h windows.HWND, s string) {
	pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(utf16z(s))))
}
