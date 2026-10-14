//go:build windows

// fleet-rdp-maker: a native Win32 window that builds RDP launchers for people
// outside FleetView. The window holds no logic of its own: it copies its
// controls into a form (form.go), calls the maker package and shows the
// results. Nothing persists: no registry, no settings file, no flags.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/4aykas/fleet-rdp/cert"
	"github.com/4aykas/fleet-rdp/maker"
	"github.com/4aykas/fleet-rdp/payload"
)

const (
	className   = "FleetRdpMaker"
	windowTitle = "FleetView RDP launcher maker"
	windowStyle = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox
)

// Control IDs. Inputs and buttons from 100, labels from 200.
const (
	idServer = 100 + iota
	idLogin
	idPassword
	idShow
	idMore
	idName
	idMonitors
	idCert
	idCheck
	idPeople
	idAdd
	idRemove
	idCreate
	idStatus
	idFolder
)

const (
	idLblServer = 200 + iota
	idLblLogin
	idLblPassword
	idLblName
	idLblMonitors
	idLblCert
	idLblPeople
)

const (
	textMore  = "More options ▸"
	textFewer = "Fewer options ▾"
)

// monitorModes maps the Monitors combo's index to the payload value.
var monitorModes = []string{payload.MonitorsAuto, payload.MonitorsOne, payload.MonitorsAll}

// ctlDef is one control, in creation order, which is also the Tab order.
type ctlDef struct {
	id      int
	class   string
	text    string
	style   uint32
	exStyle uint32
}

const (
	stLabel  = ssLeft | ssCenterImage
	stEdit   = wsTabStop | esAutoHScroll
	stButton = wsTabStop | bsPushButton
)

var controls = []ctlDef{
	{idLblServer, "STATIC", "Server", stLabel, 0},
	{idServer, "EDIT", "", stEdit, wsExClientEdge},
	{idLblLogin, "STATIC", "Login", stLabel, 0},
	{idLogin, "EDIT", "", stEdit, wsExClientEdge},
	{idLblPassword, "STATIC", "Password", stLabel, 0},
	{idPassword, "EDIT", "", stEdit | esPassword, wsExClientEdge},
	{idShow, "BUTTON", "Show", stButton, 0},
	{idMore, "BUTTON", textMore, stButton, 0},
	{idLblName, "STATIC", "Name", stLabel, 0},
	{idName, "EDIT", "", stEdit, wsExClientEdge},
	{idLblMonitors, "STATIC", "Monitors", stLabel, 0},
	{idMonitors, "COMBOBOX", "", wsTabStop | wsVScroll | cbsDropDownList, 0},
	{idLblCert, "STATIC", "Certificate SHA-256", stLabel, 0},
	{idCert, "EDIT", "", stEdit, wsExClientEdge},
	{idCheck, "BUTTON", "Check", stButton, 0},
	{idLblPeople, "STATIC", "People", ssLeft, 0},
	{idPeople, "LISTBOX", "", wsTabStop | lbsNotify | lbsNoIntegralHeight | wsVScroll | wsBorder, 0},
	{idAdd, "BUTTON", "Add person", stButton, 0},
	{idRemove, "BUTTON", "Remove", stButton, 0},
	{idCreate, "BUTTON", "Create launcher…", wsTabStop | bsDefPushButton, 0},
	{idStatus, "STATIC", "", stLabel | ssEndEllipsis, 0},
	{idFolder, "BUTTON", "Show in folder", stButton, 0},
}

// The layout, in 96-DPI pixels.
const (
	margin = 12
	rowH   = 23
	rowGap = 8
	lblW   = 130
	editW  = 300
	btnW   = 90
	btnGap = 6

	xLabel  = margin
	xEdit   = xLabel + lblW
	xButton = xEdit + editW + btnGap
	clientW = xButton + btnW + margin

	createW  = 130
	peopleH  = 3*rowH + 2*rowGap
	comboDrp = 4 * rowH // the Monitors drop-down list below the field
)

type cell struct{ id, x, dy, w, h int }

type row struct {
	extra bool // in the "More options" group
	h     int
	cells []cell
}

var rows = []row{
	{false, rowH, []cell{{idLblServer, xLabel, 0, lblW, rowH}, {idServer, xEdit, 0, editW, rowH}}},
	{false, rowH, []cell{{idLblLogin, xLabel, 0, lblW, rowH}, {idLogin, xEdit, 0, editW, rowH}}},
	{false, rowH, []cell{{idLblPassword, xLabel, 0, lblW, rowH}, {idPassword, xEdit, 0, editW, rowH}, {idShow, xButton, 0, btnW, rowH}}},
	{false, rowH, []cell{{idMore, xLabel, 0, lblW, rowH}}},
	{true, rowH, []cell{{idLblName, xLabel, 0, lblW, rowH}, {idName, xEdit, 0, editW, rowH}}},
	{true, rowH, []cell{{idLblMonitors, xLabel, 0, lblW, rowH}, {idMonitors, xEdit, 0, editW, rowH + comboDrp}}},
	{true, rowH, []cell{{idLblCert, xLabel, 0, lblW, rowH}, {idCert, xEdit, 0, editW, rowH}, {idCheck, xButton, 0, btnW, rowH}}},
	{true, peopleH, []cell{
		{idLblPeople, xLabel, 4, lblW, rowH - 4}, {idPeople, xEdit, 0, editW, peopleH},
		{idAdd, xButton, 0, btnW, rowH}, {idRemove, xButton, rowH + rowGap, btnW, rowH},
	}},
	{false, rowH, []cell{{idCreate, xButton + btnW - createW, 0, createW, rowH}}},
	{false, rowH, []cell{{idStatus, xLabel, 0, xButton - btnGap - xLabel, rowH}, {idFolder, xButton, 0, btnW, rowH}}},
}

// ui is the window's whole state. Only the main (locked) thread touches it,
// except check, which Check's goroutine fills under its mutex.
var ui struct {
	hwnd      windows.HWND
	ctl       map[int]windows.HWND
	font      uintptr
	dpi       uint32
	expanded  bool
	shown     bool         // the password is visible
	focus     windows.HWND // the focused control while the window is inactive
	f         form
	template  []byte
	savedPath string
	check     struct {
		sync.Mutex
		host string
		port int
		der  []byte
		err  error
	}
}

func main() {
	runtime.LockOSThread()
	// GetSaveFileNameW wants a single-threaded apartment.
	if err := windows.CoInitializeEx(0, coinitApartmentThreaded); err == nil {
		defer windows.CoUninitialize()
	}
	tmpl, tmplErr := embeddedTemplate()
	ui.template = tmpl

	icc := initCommonControlsEx{size: uint32(unsafe.Sizeof(initCommonControlsEx{})), icc: iccStandardClasses}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	inst, _, _ := pGetModuleHandleW.Call(0)
	icon, _, _ := pLoadImageW.Call(inst, uintptr(unsafe.Pointer(utf16z("APP"))), imageIcon, 0, 0, lrDefaultSize)
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	brush, _, _ := pGetSysColorBrush.Call(colorBtnFace)
	wc := wndClassEx{
		wndProc:    syscall.NewCallback(wndProc),
		instance:   windows.Handle(inst),
		icon:       windows.Handle(icon),
		cursor:     windows.Handle(cursor),
		background: windows.Handle(brush),
		className:  utf16z(className),
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		fatal("RegisterClassExW", err)
	}

	h, _, err := pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(utf16z(className))), uintptr(unsafe.Pointer(utf16z(windowTitle))),
		windowStyle, cwUseDefault, cwUseDefault, 600, 400, 0, 0, inst, 0)
	if h == 0 {
		fatal("CreateWindowExW", err)
	}
	ui.hwnd = windows.HWND(h)
	ui.ctl = map[int]windows.HWND{}
	ui.dpi = windowDpi(ui.hwnd)
	ui.font = newFont(ui.dpi)

	hiddenAtStart := map[int]bool{idFolder: true}
	for _, r := range rows {
		if r.extra {
			for _, c := range r.cells {
				hiddenAtStart[c.id] = true
			}
		}
	}
	for _, c := range controls {
		style := uintptr(wsChild | c.style)
		if !hiddenAtStart[c.id] {
			style |= wsVisible
		}
		ch, _, err := pCreateWindowExW.Call(uintptr(c.exStyle),
			uintptr(unsafe.Pointer(utf16z(c.class))), uintptr(unsafe.Pointer(utf16z(c.text))),
			style, 0, 0, 0, 0, h, uintptr(c.id), inst, 0)
		if ch == 0 {
			fatal("CreateWindowExW "+c.class, err)
		}
		ui.ctl[c.id] = windows.HWND(ch)
	}
	for _, s := range []string{"Auto", "One", "All"} {
		send(ui.ctl[idMonitors], cbAddString, 0, uintptr(unsafe.Pointer(utf16z(s))))
	}
	send(ui.ctl[idMonitors], cbSetCurSel, 0, 0)

	if tmplErr != nil {
		status(tmplErr.Error())
		enable(idCreate, false)
	}
	layout(ui.dpi)
	pShowWindow.Call(h, swShow)
	pSetFocus.Call(uintptr(ui.ctl[idServer]))

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		// Tab, Shift+Tab and Enter (the default button) work as in a dialog.
		if d, _, _ := pIsDialogMessageW.Call(h, uintptr(unsafe.Pointer(&m))); d != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if ui.font != 0 {
		pDeleteObject.Call(ui.font)
	}
}

// fatal is for start-up failures only, before there is a window to show them.
func fatal(what string, err error) {
	t := utf16z(windowTitle)
	text := utf16z(what + " failed: " + err.Error())
	windows.MessageBox(0, text, t, windows.MB_ICONERROR|windows.MB_OK)
	os.Exit(1)
}

func windowDpi(h windows.HWND) uint32 {
	if h != 0 {
		if d, _, _ := pGetDpiForWindow.Call(uintptr(h)); d != 0 {
			return uint32(d)
		}
	}
	if d, _, _ := pGetDpiForSystem.Call(); d != 0 {
		return uint32(d)
	}
	return 96
}

// newFont is Segoe UI 9 pt at this DPI.
func newFont(dpi uint32) uintptr {
	height := -int32(9 * dpi / 72)
	f, _, _ := pCreateFontW.Call(uintptr(height), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16z("Segoe UI"))))
	return f
}

// layout places every control from rows at this DPI, skipping the extra
// group while it is collapsed, and sizes the window to fit.
func layout(dpi uint32) {
	ui.dpi = dpi
	sc := func(v int) uintptr { return uintptr((v*int(dpi) + 48) / 96) }
	for _, h := range ui.ctl {
		send(h, wmSetFont, ui.font, 1)
	}
	y := margin
	for _, r := range rows {
		if r.extra && !ui.expanded {
			continue
		}
		for _, c := range r.cells {
			pMoveWindow.Call(uintptr(ui.ctl[c.id]), sc(c.x), sc(y+c.dy), sc(c.w), sc(c.h), 1)
		}
		y += r.h + rowGap
	}
	y += margin - rowGap

	rc := rect{right: int32(sc(clientW)), bottom: int32(sc(y))}
	pAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&rc)), windowStyle, 0, 0, uintptr(dpi))
	pSetWindowPos.Call(uintptr(ui.hwnd), 0, 0, 0, uintptr(rc.right-rc.left), uintptr(rc.bottom-rc.top),
		swpNoMove|swpNoZOrder|swpNoActivate)
}

func wndProc(hwnd windows.HWND, m uint32, w, l uintptr) uintptr {
	switch m {
	case wmCommand:
		if w>>16&0xFFFF == 0 { // BN_CLICKED
			if click(int(w & 0xFFFF)) {
				return 0
			}
		}
	case dmGetDefID:
		return idCreate | dcHasDefID<<16
	case wmCheckDone:
		checkDone()
		return 0
	case wmActivate:
		// A plain window does not keep its focused child across activation.
		if w&0xFFFF == waInactive {
			f, _, _ := pGetFocus.Call()
			ui.focus = windows.HWND(f)
		} else if ui.focus != 0 {
			pSetFocus.Call(uintptr(ui.focus))
			return 0
		}
	case wmDpiChanged:
		dpi := uint32(w >> 16 & 0xFFFF)
		suggested := *(**rect)(unsafe.Pointer(&l))
		pSetWindowPos.Call(uintptr(hwnd), 0, uintptr(suggested.left), uintptr(suggested.top), 0, 0,
			swpNoSize|swpNoZOrder|swpNoActivate)
		old := ui.font
		ui.font = newFont(dpi)
		layout(dpi)
		if old != 0 {
			pDeleteObject.Call(old)
		}
		return 0
	case wmClose:
		pDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(m), w, l)
	return r
}

// click handles a button; false for IDs that are not buttons.
func click(id int) bool {
	switch id {
	case idShow:
		togglePassword()
	case idMore:
		toggleMore()
	case idAdd:
		addPerson()
	case idRemove:
		removePerson()
	case idCheck:
		startCheck()
	case idCreate:
		create()
	case idFolder:
		showInFolder()
	default:
		return false
	}
	return true
}

func status(s string) { setText(ui.ctl[idStatus], s) }

func enable(id int, on bool) {
	var b uintptr
	if on {
		b = 1
	}
	pEnableWindow.Call(uintptr(ui.ctl[id]), b)
}

func show(id int, on bool) {
	cmd := uintptr(swHide)
	if on {
		cmd = swShow
	}
	pShowWindow.Call(uintptr(ui.ctl[id]), cmd)
}

func togglePassword() {
	ui.shown = !ui.shown
	pw := ui.ctl[idPassword]
	if ui.shown {
		send(pw, emSetPasswordChar, 0, 0)
		setText(ui.ctl[idShow], "Hide")
	} else {
		send(pw, emSetPasswordChar, 0x25CF, 0)
		setText(ui.ctl[idShow], "Show")
	}
	pInvalidateRect.Call(uintptr(pw), 0, 1)
}

func toggleMore() {
	ui.expanded = !ui.expanded
	for _, r := range rows {
		if r.extra {
			for _, c := range r.cells {
				show(c.id, ui.expanded)
			}
		}
	}
	if ui.expanded {
		setText(ui.ctl[idMore], textFewer)
	} else {
		setText(ui.ctl[idMore], textMore)
	}
	layout(ui.dpi)
}

// readForm copies the controls into the window's one form, which keeps People.
func readForm() *form {
	f := &ui.f
	f.Server = getText(ui.ctl[idServer])
	f.Login = getText(ui.ctl[idLogin])
	f.Password = getText(ui.ctl[idPassword])
	f.Name = getText(ui.ctl[idName])
	f.Cert = getText(ui.ctl[idCert])
	f.Monitors = payload.MonitorsAuto
	if i := int(int32(send(ui.ctl[idMonitors], cbGetCurSel, 0, 0))); i >= 0 && i < len(monitorModes) {
		f.Monitors = monitorModes[i]
	}
	return f
}

// fillPeople rebuilds the listbox from the form's People (logins only).
func fillPeople() {
	lb := ui.ctl[idPeople]
	send(lb, lbResetContent, 0, 0)
	for _, p := range ui.f.People {
		send(lb, lbAddString, 0, uintptr(unsafe.Pointer(utf16z(p.User))))
	}
}

func addPerson() {
	f := readForm()
	if err := f.addPerson(); err != nil {
		status(err.Error())
		return
	}
	fillPeople()
	setText(ui.ctl[idLogin], "")
	setText(ui.ctl[idPassword], "")
	status("")
	pSetFocus.Call(uintptr(ui.ctl[idLogin]))
}

func removePerson() {
	i := int(int32(send(ui.ctl[idPeople], lbGetCurSel, 0, 0)))
	ui.f.removePerson(i)
	fillPeople()
}

// startCheck fetches the server's certificate on a goroutine, which only
// stores the result and posts wmCheckDone; it never touches a control.
func startCheck() {
	host, port, err := checkTarget(getText(ui.ctl[idServer]))
	if err != nil {
		status(err.Error())
		return
	}
	enable(idCheck, false)
	status("Checking " + net.JoinHostPort(host, strconv.Itoa(port)) + "…")
	hwnd := ui.hwnd
	go func() {
		der, err := cert.Fetch(context.Background(), host, port)
		ui.check.Lock()
		ui.check.host, ui.check.port, ui.check.der, ui.check.err = host, port, der, err
		ui.check.Unlock()
		pPostMessageW.Call(uintptr(hwnd), wmCheckDone, 0, 0)
	}()
}

func checkDone() {
	ui.check.Lock()
	host, port, der, err := ui.check.host, ui.check.port, ui.check.der, ui.check.err
	ui.check.Unlock()
	enable(idCheck, true)
	if curHost, curPort, cerr := checkTarget(getText(ui.ctl[idServer])); cerr != nil || curHost != host || curPort != port {
		status(checkStale())
		return
	}
	if err != nil {
		// The fixed sentence is the message; the fetch error is not shown.
		status(checkFailed(host, port))
		return
	}
	sum := cert.Sha256(der)
	setText(ui.ctl[idCert], hex.EncodeToString(sum[:]))
	status(checkOK())
}

func create() {
	show(idFolder, false)
	ui.savedPath = ""
	if ui.template == nil {
		status(errNoTemplate.Error())
		return
	}
	f := readForm()
	spec, err := f.spec()
	if err != nil {
		status(err.Error())
		return
	}
	res, err := maker.Build(spec, ui.template)
	if err != nil {
		status(err.Error())
		return
	}

	path, ok, err := saveAs(spec)
	if err != nil {
		status(err.Error())
		return
	}
	if !ok {
		return // cancelled
	}
	final, exe, err := outputPath(path, len(spec.People))
	if err != nil {
		status(err.Error())
		return
	}
	var data []byte
	if exe {
		data = res.Files[0].Data
	} else if data, err = maker.Zip(res.Files); err != nil {
		status("Cannot make the ZIP: " + err.Error())
		return
	}
	if err := maker.WriteAtomic(final, data); err != nil {
		status(fmt.Sprintf("Cannot write %s: %v", final, err))
		return
	}

	status(savedMessage(final, res.DefaultIcon))
	ui.savedPath = final
	show(idFolder, true)
	ui.f.clearSecrets()
	setText(ui.ctl[idPassword], "")
	fillPeople()
}

// saveAs runs the Save As dialog. ok is false when the user cancelled.
func saveAs(spec maker.Spec) (path string, ok bool, err error) {
	filter := multiZ("ZIP archive (*.zip)", "*.zip")
	if len(spec.People) == 1 {
		filter = multiZ("ZIP archive (*.zip)", "*.zip", "Launcher (*.exe)", "*.exe")
	}
	var buf [windows.MAX_LONG_PATH]uint16
	copy(buf[:len(buf)-1], utf16.Encode([]rune(suggestedName(spec))))
	ofn := openFileName{
		owner:   ui.hwnd,
		filter:  filter,
		file:    &buf[0],
		maxFile: uint32(len(buf)),
		defExt:  utf16z("zip"),
		flags:   ofnExplorer | ofnOverwritePrompt | ofnPathMustExist | ofnNoChangeDir,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		if code, _, _ := pCommDlgExtendedError.Call(); code != 0 {
			return "", false, errors.New("Save As failed (code " + strconv.Itoa(int(code)) + ").")
		}
		return "", false, nil
	}
	return windows.UTF16ToString(buf[:]), true, nil
}

// showInFolder opens Explorer with the saved file selected. Explorer's exit
// code means nothing, so it is not waited for; a failure to start is ignored
// too, since the file is saved either way.
func showInFolder() {
	if ui.savedPath == "" {
		return
	}
	// The Windows directory, not PATH, so a planted explorer.exe is not run.
	explorer := "explorer.exe"
	if dir, err := windows.GetWindowsDirectory(); err == nil {
		explorer = filepath.Join(dir, "explorer.exe")
	}
	cmd := exec.Command(explorer)
	// Explorer wants /select,"path" with the quotes around the path only,
	// which Go's own argument quoting does not produce.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + explorer + `" /select,"` + ui.savedPath + `"`}
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}
