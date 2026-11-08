//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")

	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pGetMessageW          = user32.NewProc("GetMessageW")
	pIsDialogMessageW     = user32.NewProc("IsDialogMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessageW     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pSendMessageW         = user32.NewProc("SendMessageW")
	pMessageBoxW          = user32.NewProc("MessageBoxW")
	pSetWindowTextW       = user32.NewProc("SetWindowTextW")
	pEnableWindow         = user32.NewProc("EnableWindow")
	pShowWindow           = user32.NewProc("ShowWindow")
	pUpdateWindow         = user32.NewProc("UpdateWindow")
	pLoadCursorW          = user32.NewProc("LoadCursorW")
	pLoadIconW            = user32.NewProc("LoadIconW")
	pGetDC                = user32.NewProc("GetDC")
	pReleaseDC            = user32.NewProc("ReleaseDC")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pAdjustWindowRect     = user32.NewProc("AdjustWindowRect")
	pGetSysColorBrush     = user32.NewProc("GetSysColorBrush")
	pGetDeviceCaps        = gdi32.NewProc("GetDeviceCaps")
	pCreateFontW          = gdi32.NewProc("CreateFontW")
	pSetTextColor         = gdi32.NewProc("SetTextColor")
	pSetBkMode            = gdi32.NewProc("SetBkMode")
	pGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	pCreateActCtxW        = kernel32.NewProc("CreateActCtxW")
	pActivateActCtx       = kernel32.NewProc("ActivateActCtx")
	pGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
	pShellExecuteW        = shell32.NewProc("ShellExecuteW")
	pInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
)

const (
	wsOverlapped       = 0x00000000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsMinimizeBox      = 0x00020000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsTabStop          = 0x00010000
	wsVScroll          = 0x00200000
	wsExClientEdge     = 0x00000200
	esReadOnly         = 0x0800
	esAutoHScroll      = 0x0080
	bsDefPush          = 0x0001
	cbsDropDownList    = 0x0003
	wmDestroy          = 0x0002
	wmCommand          = 0x0111
	wmSetFont          = 0x0030
	wmCtlColorStatic   = 0x0138
	cbAddString        = 0x0143
	cbSetCurSel        = 0x014E
	cbGetCurSel        = 0x0147
	cbResetContent     = 0x014B
	mbOK               = 0x0
	mbYesNo            = 0x4
	mbIconInfo         = 0x40
	mbIconError        = 0x10
	mbIconWarn         = 0x30
	idYes              = 6
	ofnFileMustExist   = 0x1000
	ofnPathMustExist   = 0x0800
	ofnOverwritePrompt = 0x0002
	ofnExplorer        = 0x00080000
	ofnNoChangeDir     = 0x0008
	ofnHideReadOnly    = 0x0004

	idBrowse  = 101
	idCombo   = 102
	idConvert = 103
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }
type msgT struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	priv    uint32
}
type rect struct{ l, t, r, b int32 }

type openFileName struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}

type actCtx struct {
	cbSize                 uint32
	dwFlags                uint32
	lpSource               *uint16
	wProcessorArchitecture uint16
	wLangId                uint16
	lpAssemblyDirectory    *uint16
	lpResourceName         *uint16
	lpApplicationName      *uint16
	hModule                uintptr
}

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// filter strings contain embedded NULs
func u16multi(s string) *uint16 {
	r := syscall.StringToUTF16(s) // adds final NUL
	for i := range r {
		if r[i] == '|' {
			r[i] = 0
		}
	}
	return &r[0]
}

var (
	hInst                                            uintptr
	hMain, hFile, hKey, hSrc, hCombo, hConv, hStatus uintptr
	fontN, fontB                                     uintptr
	dpi                                              int32 = 96
	curPath                                          string
	curData                                          []byte
	curKey                                           KeyInfo
)

// Version is set at build time (-X main.Version=v1.2.3).
var Version = "dev"

func sc(v int32) int32 { return v * dpi / 96 }

func msgBox(text, title string, flags uintptr) uintptr {
	r, _, _ := pMessageBoxW.Call(hMain, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags)
	return r
}

func setText(h uintptr, s string) { pSetWindowTextW.Call(h, uintptr(unsafe.Pointer(u16(s)))) }

func child(class, text string, style uintptr, x, y, w, h int32, id uintptr, exStyle uintptr, font uintptr) uintptr {
	hw, _, _ := pCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(text))),
		wsChild|wsVisible|style, uintptr(sc(x)), uintptr(sc(y)), uintptr(sc(w)), uintptr(sc(h)), hMain, id, hInst, 0)
	pSendMessageW.Call(hw, wmSetFont, font, 1)
	return hw
}

func enableVisualStyles() {
	manifest := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
<dependency><dependentAssembly><assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/></dependentAssembly></dependency>
</assembly>`
	f := filepath.Join(os.TempDir(), "chordkey-styles.manifest")
	if os.WriteFile(f, []byte(manifest), 0644) != nil {
		return
	}
	ac := actCtx{lpSource: u16(f)}
	ac.cbSize = uint32(unsafe.Sizeof(ac))
	h, _, _ := pCreateActCtxW.Call(uintptr(unsafe.Pointer(&ac)))
	if h != 0 && h != ^uintptr(0) {
		var cookie uintptr
		pActivateActCtx.Call(h, uintptr(unsafe.Pointer(&cookie)))
	}
	icc := struct{ size, flags uint32 }{8, 0x4000 | 0xFF}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
}

func fileDialog(save bool, title, filter, initDir, initName, defExt string) string {
	buf := make([]uint16, 4096)
	copy(buf, syscall.StringToUTF16(initName))
	ofn := openFileName{
		hwndOwner:   hMain,
		lpstrFilter: u16multi(filter),
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  u16(title),
		Flags:       ofnExplorer | ofnPathMustExist | ofnNoChangeDir | ofnHideReadOnly,
	}
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	if initDir != "" {
		ofn.lpstrInitialDir = u16(initDir)
	}
	if defExt != "" {
		ofn.lpstrDefExt = u16(defExt)
	}
	var r uintptr
	if save {
		ofn.Flags |= ofnOverwritePrompt
		r, _, _ = pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	} else {
		ofn.Flags |= ofnFileMustExist
		r, _, _ = pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	}
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func fillCombo(minor bool, sel int) {
	pSendMessageW.Call(hCombo, cbResetContent, 0, 0)
	for _, n := range NAMES {
		if minor {
			n += "m"
		}
		pSendMessageW.Call(hCombo, cbAddString, 0, uintptr(unsafe.Pointer(u16(n))))
	}
	pSendMessageW.Call(hCombo, cbSetCurSel, uintptr(sel), 0)
}

func onBrowse() {
	p := fileDialog(false, "Choose a chord chart", "Chord charts (*.pdf, *.docx)|*.pdf;*.docx|PDF files (*.pdf)|*.pdf|Word files (*.docx)|*.docx|", "", "", "")
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		msgBox("Could not open the file:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	curPath, curData = p, data
	setText(hFile, p)
	k, err := Detect(p, data)
	if err != nil {
		curKey = KeyInfo{}
		setText(hKey, "?")
		setText(hSrc, "")
		setText(hStatus, "Couldn't find the key: "+err.Error())
		pEnableWindow.Call(hConv, 0)
		return
	}
	curKey = k
	setText(hKey, k.Key)
	setText(hSrc, "("+k.Source+")")
	fillCombo(strings.HasSuffix(k.Key, "m"), KeyRoot(k.Key))
	setText(hStatus, "Pick the new key, then click Convert.")
	pEnableWindow.Call(hConv, 1)
}

func onConvert() {
	if curPath == "" || curKey.Key == "" {
		return
	}
	sel, _, _ := pSendMessageW.Call(hCombo, cbGetCurSel, 0, 0)
	if int(sel) < 0 || int(sel) >= 12 {
		return
	}
	n := int(sel) - KeyRoot(curKey.Key)
	if ((n%12)+12)%12 == 0 {
		msgBox("The chart is already in this key. Pick a different key.", "Chord Key Changer", mbIconInfo)
		return
	}
	newKey := NAMES[sel]
	if strings.HasSuffix(curKey.Key, "m") {
		newKey += "m"
	}
	ext := strings.ToLower(filepath.Ext(curPath))
	filter := "PDF files (*.pdf)|*.pdf|"
	if ext == ".docx" {
		filter = "Word files (*.docx)|*.docx|"
	}
	out := fileDialog(true, "Save the "+newKey+" key version as", filter, filepath.Dir(curPath), OutputName(curPath, newKey), strings.TrimPrefix(ext, "."))
	if out == "" {
		return
	}
	if strings.EqualFold(filepath.Clean(out), filepath.Clean(curPath)) {
		msgBox("Please save under a different name so the original isn't overwritten.", "Chord Key Changer", mbIconWarn)
		return
	}
	setText(hStatus, "Converting...")
	b, cnt, warns, err := Convert(curPath, curData, n)
	if err != nil {
		setText(hStatus, "Conversion failed.")
		msgBox("Conversion failed:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	if err := os.WriteFile(out, b, 0644); err != nil {
		setText(hStatus, "Could not save.")
		msgBox("Could not save the file (is it open in another program?)\n\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	setText(hStatus, fmt.Sprintf("Done: %s → %s, %d chord(s) changed.", curKey.Key, newKey, cnt))
	text := fmt.Sprintf("Saved:\n%s\n\n%s → %s, %d chord(s) changed.", out, curKey.Key, newKey, cnt)
	icon := uintptr(mbIconInfo)
	if len(warns) > 0 {
		text += "\n\nPlease check:\n• " + strings.Join(warns, "\n• ")
		icon = mbIconWarn
	}
	text += "\n\nOpen the file now?"
	if msgBox(text, "Chord Key Changer", mbYesNo|icon) == idYes {
		pShellExecuteW.Call(hMain, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(out))), 0, 0, 1)
	}
}

func safe(f func()) {
	defer func() {
		if r := recover(); r != nil {
			msgBox(fmt.Sprintf("Something went wrong:\n%v", r), "Chord Key Changer", mbIconError)
		}
	}()
	f()
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		id := wParam & 0xffff
		code := (wParam >> 16) & 0xffff
		if code == 0 {
			switch id {
			case idBrowse:
				safe(onBrowse)
				return 0
			case idConvert:
				safe(onConvert)
				return 0
			}
		}
	case wmCtlColorStatic:
		if lParam == hKey {
			pSetTextColor.Call(wParam, 0x00C07000) // BGR: blue like the chords
			pSetBkMode.Call(wParam, 1)
			br, _, _ := pGetSysColorBrush.Call(15)
			return br
		}
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()
	enableVisualStyles()
	hInst, _, _ = pGetModuleHandleW.Call(0)
	dc, _, _ := pGetDC.Call(0)
	if d, _, _ := pGetDeviceCaps.Call(dc, 88); d > 0 {
		dpi = int32(d)
	}
	pReleaseDC.Call(0, dc)

	mkFont := func(pt int32, weight uintptr) uintptr {
		f, _, _ := pCreateFontW.Call(uintptr(-pt*dpi/72)&0xffffffff, 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16("Segoe UI"))))
		return f
	}
	fontN = mkFont(10, 400)
	fontB = mkFont(18, 700)

	cls := u16("ChordKeyChangerWnd")
	cur, _, _ := pLoadCursorW.Call(0, 32512)
	ico, _, _ := pLoadIconW.Call(0, 32512)
	bg, _, _ := pGetSysColorBrush.Call(15)
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: hInst, hCursor: cur, hIcon: ico, hIconSm: ico,
		hbrBackground: bg, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	rc := rect{0, 0, sc(540), sc(262)}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	hMain, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(u16("Chord Key Changer "+Version))),
		style, 0x80000000, 0x80000000, uintptr(rc.r-rc.l), uintptr(rc.b-rc.t), 0, 0, hInst, 0)

	child("STATIC", "Chord chart (PDF or Word):", 0, 18, 16, 300, 20, 0, 0, fontN)
	hFile = child("EDIT", "", esReadOnly|esAutoHScroll|wsTabStop, 18, 40, 384, 26, 0, wsExClientEdge, fontN)
	child("BUTTON", "Browse...", wsTabStop, 412, 39, 110, 28, idBrowse, 0, fontN)
	child("STATIC", "Current key:", 0, 18, 92, 110, 22, 0, 0, fontN)
	hKey = child("STATIC", "—", 0, 132, 82, 110, 38, 0, 0, fontB)
	hSrc = child("STATIC", "", 0, 246, 92, 280, 22, 0, 0, fontN)
	child("STATIC", "Change to:", 0, 18, 140, 110, 22, 0, 0, fontN)
	hCombo = child("COMBOBOX", "", cbsDropDownList|wsTabStop|wsVScroll, 132, 136, 110, 400, idCombo, 0, fontN)
	hConv = child("BUTTON", "Convert...", wsTabStop|bsDefPush, 132, 180, 150, 34, idConvert, 0, fontN)
	hStatus = child("STATIC", "Click Browse to choose a chord chart.", 0, 18, 230, 504, 22, 0, 0, fontN)
	fillCombo(false, 0)
	pEnableWindow.Call(hConv, 0)

	pShowWindow.Call(hMain, 1)
	pUpdateWindow.Call(hMain)

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if ok, _, _ := pIsDialogMessageW.Call(hMain, uintptr(unsafe.Pointer(&m))); ok != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
