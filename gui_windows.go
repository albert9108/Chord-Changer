//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------- win32 procs

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassExW   = user32.NewProc("RegisterClassExW")
	pCreateWindowExW    = user32.NewProc("CreateWindowExW")
	pDefWindowProcW     = user32.NewProc("DefWindowProcW")
	pGetMessageW        = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessageW   = user32.NewProc("DispatchMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pMessageBoxW        = user32.NewProc("MessageBoxW")
	pShowWindow         = user32.NewProc("ShowWindow")
	pUpdateWindow       = user32.NewProc("UpdateWindow")
	pLoadCursorW        = user32.NewProc("LoadCursorW")
	pLoadIconW          = user32.NewProc("LoadIconW")
	pGetDC              = user32.NewProc("GetDC")
	pReleaseDC          = user32.NewProc("ReleaseDC")
	pSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiCtx   = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
	pAdjustWindowRect   = user32.NewProc("AdjustWindowRect")
	pInvalidateRect     = user32.NewProc("InvalidateRect")
	pBeginPaint         = user32.NewProc("BeginPaint")
	pEndPaint           = user32.NewProc("EndPaint")
	pGetClientRect      = user32.NewProc("GetClientRect")
	pGetWindowRect      = user32.NewProc("GetWindowRect")
	pSetCursor          = user32.NewProc("SetCursor")
	pTrackMouseEvent    = user32.NewProc("TrackMouseEvent")
	pSetWindowPos       = user32.NewProc("SetWindowPos")
	pDrawTextW          = user32.NewProc("DrawTextW")
	pGetKeyState        = user32.NewProc("GetKeyState")
	pSetFocus           = user32.NewProc("SetFocus")
	pSetWindowTextW     = user32.NewProc("SetWindowTextW")
	pGetWindowTextW     = user32.NewProc("GetWindowTextW")
	pGetWindowTextLenW  = user32.NewProc("GetWindowTextLengthW")
	pSendMessageW       = user32.NewProc("SendMessageW")
	pPostMessageW       = user32.NewProc("PostMessageW")
	pOpenClipboard      = user32.NewProc("OpenClipboard")
	pCloseClipboard     = user32.NewProc("CloseClipboard")
	pGetClipboardData   = user32.NewProc("GetClipboardData")
	pIsClipFmtAvailable = user32.NewProc("IsClipboardFormatAvailable")

	pGetDeviceCaps         = gdi32.NewProc("GetDeviceCaps")
	pCreateFontW           = gdi32.NewProc("CreateFontW")
	pSetTextColor          = gdi32.NewProc("SetTextColor")
	pSetBkMode             = gdi32.NewProc("SetBkMode")
	pCreateCompatibleDC    = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection      = gdi32.NewProc("CreateDIBSection")
	pSelectObject          = gdi32.NewProc("SelectObject")
	pDeleteObject          = gdi32.NewProc("DeleteObject")
	pDeleteDC              = gdi32.NewProc("DeleteDC")
	pBitBlt                = gdi32.NewProc("BitBlt")
	pGdiFlush              = gdi32.NewProc("GdiFlush")
	pSetTextCharacterExtra = gdi32.NewProc("SetTextCharacterExtra")
	pSetBkColor            = gdi32.NewProc("SetBkColor")
	pCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pCreateActCtxW    = kernel32.NewProc("CreateActCtxW")
	pActivateActCtx   = kernel32.NewProc("ActivateActCtx")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalSize       = kernel32.NewProc("GlobalSize")

	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

	pShellExecuteW   = shell32.NewProc("ShellExecuteW")
	pDragAcceptFiles = shell32.NewProc("DragAcceptFiles")
	pDragQueryFileW  = shell32.NewProc("DragQueryFileW")
	pDragFinish      = shell32.NewProc("DragFinish")

	pInitCommonControlsEx  = comctl32.NewProc("InitCommonControlsEx")
	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	pRegGetValueW          = advapi32.NewProc("RegGetValueW")
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000

	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmSettingChange = 0x001A
	wmSetCursor     = 0x0020
	wmKeyDown       = 0x0100
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmDropFiles     = 0x0233
	wmMouseLeave    = 0x02A3
	wmDpiChanged    = 0x02E0
	wmSetFont       = 0x0030
	wmCtlColorEdit  = 0x0133
	wmAIDone        = 0x8001 // WM_APP+1

	wsChild       = 0x40000000
	wsVScroll     = 0x00200000
	wsTabStop     = 0x00010000
	esMultiline   = 0x0004
	esPassword    = 0x0020
	esAutoVScroll = 0x0040
	esAutoHScroll = 0x0080
	esNoHideSel   = 0x0100
	esWantReturn  = 0x1000
	emSetSel      = 0x00B1
	emReplaceSel  = 0x00C2
	emSetLimit    = 0x00C5
	emSetMargins  = 0x00D3

	vkTab    = 0x09
	vkReturn = 0x0D
	vkShift  = 0x10
	vkSpace  = 0x20
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28

	dtLeft         = 0x0000
	dtCenter       = 0x0001
	dtRight        = 0x0002
	dtVCenter      = 0x0004
	dtSingleLine   = 0x0020
	dtNoPrefix     = 0x0800
	dtEndEllipsis  = 0x8000
	dtPathEllipsis = 0x4000

	mbOK        = 0x0
	mbYesNo     = 0x4
	mbIconInfo  = 0x40
	mbIconError = 0x10
	mbIconWarn  = 0x30
	idYes       = 6

	ofnFileMustExist   = 0x1000
	ofnPathMustExist   = 0x0800
	ofnOverwritePrompt = 0x0002
	ofnExplorer        = 0x00080000
	ofnNoChangeDir     = 0x0008
	ofnHideReadOnly    = 0x0004
	ofnAllowMulti      = 0x0200

	srcCopy = 0x00CC0020
	idcHand = 32649
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

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type trackMouseEvent struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
}

type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}

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
	r := syscall.StringToUTF16(s)
	for i := range r {
		if r[i] == '|' {
			r[i] = 0
		}
	}
	return &r[0]
}

// tryCall skips APIs that are missing on older Windows versions.
func tryCall(p *syscall.LazyProc, a ...uintptr) uintptr {
	if p.Find() != nil {
		return 0
	}
	r, _, _ := p.Call(a...)
	return r
}

// ------------------------------------------------------- software renderer
//
// GDI+ takes its geometry as 32-bit floats, and Go's syscall.Proc.Call passes
// every argument in an integer register, so float arguments arrive corrupted
// on windows/amd64. Instead we own the pixels: the back buffer is a 32-bit
// top-down DIB section, shapes are rasterised here in Go with signed-distance
// antialiasing, and text is left to GDI (ClearType) painting into the same
// buffer. One BitBlt puts the finished frame on screen.

type rgbC struct{ r, g, b float64 }

func hexc(v uint32) rgbC {
	return rgbC{float64((v >> 16) & 0xff), float64((v >> 8) & 0xff), float64(v & 0xff)}
}

// mix returns a blended towards b by t (0..1).
func mix(a, b rgbC, t float64) rgbC {
	return rgbC{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// colref packs a colour the way GDI wants it: 0x00BBGGRR.
func colref(c rgbC) uintptr {
	return uintptr(uint32(c.b+0.5)<<16 | uint32(c.g+0.5)<<8 | uint32(c.r+0.5))
}

type canvas struct {
	px   []uint32
	w, h int
}

func (c *canvas) clear(col rgbC) {
	v := 0xff000000 | uint32(col.r+0.5)<<16 | uint32(col.g+0.5)<<8 | uint32(col.b+0.5)
	for i := range c.px {
		c.px[i] = v
	}
}

func (c *canvas) blend(x, y int, col rgbC, a float64) {
	if a <= 0 || x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	if a > 1 {
		a = 1
	}
	i := y*c.w + x
	d := c.px[i]
	dr := float64((d >> 16) & 0xff)
	dg := float64((d >> 8) & 0xff)
	db := float64(d & 0xff)
	c.px[i] = 0xff000000 |
		uint32(dr+(col.r-dr)*a+0.5)<<16 |
		uint32(dg+(col.g-dg)*a+0.5)<<8 |
		uint32(db+(col.b-db)*a+0.5)
}

// sdRound is the signed distance from (px,py) to a rounded rectangle:
// negative inside, positive outside, in pixels.
func sdRound(px, py, x, y, w, h, rad float64) float64 {
	if rad > w/2 {
		rad = w / 2
	}
	if rad > h/2 {
		rad = h / 2
	}
	cx, cy := x+w/2, y+h/2
	qx := math.Abs(px-cx) - (w/2 - rad)
	qy := math.Abs(py-cy) - (h/2 - rad)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rad
}

func (c *canvas) span(x, y, w, h, pad float64) (int, int, int, int) {
	x0 := int(math.Floor(x - pad))
	y0 := int(math.Floor(y - pad))
	x1 := int(math.Ceil(x + w + pad))
	y1 := int(math.Ceil(y + h + pad))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.w-1 {
		x1 = c.w - 1
	}
	if y1 > c.h-1 {
		y1 = c.h - 1
	}
	return x0, y0, x1, y1
}

func (c *canvas) fillRound(x, y, w, h, rad float64, col rgbC, alpha float64) {
	x0, y0, x1, y1 := c.span(x, y, w, h, 2)
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			cov := 0.5 - sdRound(float64(px)+0.5, float64(py)+0.5, x, y, w, h, rad)
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			c.blend(px, py, col, cov*alpha)
		}
	}
}

// strokeRound outlines a rounded rect with a line of width th centred on the
// path. dash > 0 draws a dashed outline with that on/off length.
func (c *canvas) strokeRound(x, y, w, h, rad, th float64, col rgbC, alpha, dash float64) {
	x0, y0, x1, y1 := c.span(x, y, w, h, th+2)
	cx, cy := x+w/2, y+h/2
	rx, ry := w/2-rad, h/2-rad
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			cov := (0.5 - math.Abs(sdRound(fx, fy, x, y, w, h, rad))) + th/2
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			if dash > 0 {
				ex := math.Abs(fx-cx) - rx
				ey := math.Abs(fy-cy) - ry
				var t float64
				switch {
				case ex > 0 && ey > 0: // corner arc
					t = fx + fy
				case ex > 0: // left/right edge
					t = fy
				default: // top/bottom edge
					t = fx
				}
				if math.Mod(math.Abs(t), dash*2) > dash {
					continue
				}
			}
			c.blend(px, py, col, cov*alpha)
		}
	}
}

// shadow lays a soft drop shadow under a rounded rect. spread is the blur
// distance in pixels; the falloff is squared so it reads as a real shadow
// rather than a flat halo.
func (c *canvas) shadow(x, y, w, h, rad, spread, alpha float64, col rgbC) {
	x0, y0, x1, y1 := c.span(x, y, w, h, spread+2)
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			d := sdRound(float64(px)+0.5, float64(py)+0.5, x, y, w, h, rad)
			var cov float64
			if d <= 0 {
				cov = 1
			} else {
				cov = 1 - d/spread
			}
			if cov <= 0 {
				continue
			}
			c.blend(px, py, col, cov*cov*alpha)
		}
	}
}

func (c *canvas) fillRect(x, y, w, h float64, col rgbC, alpha float64) {
	c.fillRound(x, y, w, h, 0, col, alpha)
}

// ------------------------------------------------------------------- theme

type theme struct {
	dark                             bool
	bg, surface, border, text, muted rgbC
	accent, accentHi, onAccent       rgbC
	chip, chipBorder, chipHot        rgbC
	dropBg, dropBorder               rgbC
	shadowCol                        rgbC
	shadowA                          float64
}

func lightTheme() theme {
	return theme{
		dark:       false,
		bg:         hexc(0xF4F5F7),
		surface:    hexc(0xFFFFFF),
		border:     hexc(0xE1E5EA),
		text:       hexc(0x14181F),
		muted:      hexc(0x6B7280),
		accent:     hexc(0x2563EB),
		accentHi:   hexc(0x1D4ED8),
		onAccent:   hexc(0xFFFFFF),
		chip:       hexc(0xFFFFFF),
		chipBorder: hexc(0xD8DEE6),
		chipHot:    hexc(0xEEF2FF),
		dropBg:     hexc(0xFBFCFE),
		dropBorder: hexc(0x9DB2E8),
		shadowCol:  hexc(0x0B1220),
		shadowA:    0.10,
	}
}

func darkTheme() theme {
	return theme{
		dark:       true,
		bg:         hexc(0x1B1D21),
		surface:    hexc(0x24272C),
		border:     hexc(0x343941),
		text:       hexc(0xF2F4F7),
		muted:      hexc(0x9BA3AF),
		accent:     hexc(0x3B82F6),
		accentHi:   hexc(0x60A5FA),
		onAccent:   hexc(0xFFFFFF),
		chip:       hexc(0x2A2E34),
		chipBorder: hexc(0x3A4049),
		chipHot:    hexc(0x313846),
		dropBg:     hexc(0x202329),
		dropBorder: hexc(0x4A5876),
		shadowCol:  hexc(0x000000),
		shadowA:    0.38,
	}
}

func systemDark() bool {
	if pRegGetValueW.Find() != nil {
		return false
	}
	val := uint32(1)
	sz := uint32(4)
	r, _, _ := pRegGetValueW.Call(
		0x80000001, // HKEY_CURRENT_USER
		uintptr(unsafe.Pointer(u16(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`))),
		uintptr(unsafe.Pointer(u16("AppsUseLightTheme"))),
		0x00000010, // RRF_RT_REG_DWORD
		0,
		uintptr(unsafe.Pointer(&val)),
		uintptr(unsafe.Pointer(&sz)),
	)
	return r == 0 && val == 0
}

// applyChrome themes the title bar and asks for Win11 rounded corners.
// Both attributes are ignored on Windows versions that predate them.
func applyChrome(hwnd uintptr, dark bool) {
	var b int32
	if dark {
		b = 1
	}
	tryCall(pDwmSetWindowAttribute, hwnd, 20, uintptr(unsafe.Pointer(&b)), 4) // USE_IMMERSIVE_DARK_MODE
	tryCall(pDwmSetWindowAttribute, hwnd, 19, uintptr(unsafe.Pointer(&b)), 4) // pre-20H1 name
	pref := int32(2)                                                          // DWMWCP_ROUND
	tryCall(pDwmSetWindowAttribute, hwnd, 33, uintptr(unsafe.Pointer(&pref)), 4)
}

// ------------------------------------------------------------ layout & state

// Logical client size at 96 DPI. The window is fixed-size, so every rect
// below is a constant scaled by the current DPI; nothing reflows.
const (
	winW = 620.0
	winH = 500.0
	padX = 24.0
)

const (
	itemNone     = -1
	itemBrowse   = 12
	itemConvert  = 13
	itemTabKey   = 20
	itemTabChart = 21
	itemAddImg   = 22
	itemPasteImg = 23
	itemClearImg = 24
	itemReadAI   = 25
	itemCreate   = 26
)

const (
	pageKey   = 0
	pageChart = 1
)

type rectF struct{ x, y, w, h float64 }

func (r rectF) hit(x, y float64) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

var (
	hInst uintptr
	hMain uintptr
	dpi   int32 = 96

	th        theme
	curPath   string
	curData   []byte
	curKey    KeyInfo
	detectErr string
	selKey    = -1
	status    = "Drop a chord chart on the window, or click Browse."

	hot      = itemNone
	focus    = itemNone
	pressed  = itemNone
	tracking bool

	rCard, rBrowse, rConvert rectF
	rChips                   [12]rectF
	rTabs                    [2]rectF

	// chart maker page
	page                                      = pageKey
	rEditBox, rAddImg, rPasteImg, rClearImg   rectF
	rReadAI, rCreate, rKeyBox                 rectF
	hEdit, hKeyEdit, hEditBrush, fEdit, fKey2 uintptr
	chartImgs                                 []aiImage
	chartImgNames                             []string
	pastedCount                               int
	settings                                  appSettings
	lastChartDir                              string

	aiMu     sync.Mutex
	aiBusy   bool
	aiResult string
	aiErr    error

	memDC, memBmp, oldBmp uintptr
	cv                    canvas
	bufW, bufH            int

	fTitle, fSub, fName, fLabel, fKey, fChip, fBtn, fStatus, fSection uintptr
	hCurArrow, hCurHand                                               uintptr
)

// Version is set at build time (-X main.Version=v1.2.3).
var Version = "dev"

func scf(v float64) float64 { return v * float64(dpi) / 96 }
func sc(v float64) int32    { return int32(scf(v) + 0.5) }

func layout() {
	rCard = rectF{scf(padX), scf(94), scf(winW - 2*padX), scf(104)}
	rBrowse = rectF{scf(winW - padX - 126), scf(212), scf(126), scf(36)}
	const gap, chipH = 10.0, 42.0
	chipW := (winW - 2*padX - 5*gap) / 6
	for i := 0; i < 12; i++ {
		col, row := float64(i%6), float64(i/6)
		rChips[i] = rectF{
			scf(padX + col*(chipW+gap)),
			scf(286 + row*(chipH+gap)),
			scf(chipW), scf(chipH),
		}
	}
	rConvert = rectF{scf(padX), scf(398), scf(210), scf(46)}

	for i := range rTabs {
		rTabs[i] = rectF{scf(winW - padX - 232 + float64(i)*116), scf(20), scf(116), scf(32)}
	}
	rEditBox = rectF{scf(padX), scf(110), scf(winW - 2*padX), scf(236)}
	rAddImg = rectF{scf(padX), scf(374), scf(120), scf(34)}
	rPasteImg = rectF{scf(padX + 128), scf(374), scf(120), scf(34)}
	rClearImg = rectF{scf(padX + 256), scf(374), scf(70), scf(34)}
	rReadAI = rectF{scf(winW - padX - 160), scf(374), scf(160), scf(34)}
	rCreate = rectF{scf(padX), scf(414), scf(190), scf(36)}
	rKeyBox = rectF{scf(300), scf(416), scf(winW - padX - 300), scf(32)}
	placeEdits()
}

// placeEdits puts the native edit boxes inside their painted frames.
func placeEdits() {
	in := func(h uintptr, r rectF, ix, iy float64) {
		if h == 0 {
			return
		}
		pSetWindowPos.Call(h, 0, uintptr(int32(r.x+scf(ix))), uintptr(int32(r.y+scf(iy))),
			uintptr(int32(r.w-2*scf(ix))), uintptr(int32(r.h-2*scf(iy))), 0x0004|0x0010)
	}
	in(hEdit, rEditBox, 8, 6)
	in(hKeyEdit, rKeyBox, 10, 7)
}

func mkFont(tenths int32, weight uintptr) uintptr {
	h := uintptr(-(tenths*dpi)/720) & 0xffffffff
	f, _, _ := pCreateFontW.Call(h, 0, 0, 0, weight, 0, 0, 0,
		1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16("Segoe UI"))))
	return f
}

func makeFonts() {
	freeBuffer()
	for _, f := range []uintptr{fTitle, fSub, fName, fLabel, fKey, fChip, fBtn, fStatus, fSection} {
		if f != 0 {
			pDeleteObject.Call(f)
		}
	}
	fTitle = mkFont(158, 600)
	fSub = mkFont(92, 400)
	fName = mkFont(110, 600)
	fLabel = mkFont(90, 400)
	fKey = mkFont(240, 700)
	fChip = mkFont(112, 600)
	fBtn = mkFont(108, 600)
	fStatus = mkFont(90, 400)
	fSection = mkFont(80, 700)
	for _, f := range []uintptr{fEdit, fKey2} {
		if f != 0 {
			pDeleteObject.Call(f)
		}
	}
	fEdit = mkFontFace(105, 400, "Microsoft YaHei UI")
	fKey2 = mkFontFace(100, 400, "Segoe UI")
	for _, p := range [][2]uintptr{{hEdit, fEdit}, {hKeyEdit, fKey2}} {
		if p[0] != 0 {
			pSendMessageW.Call(p[0], wmSetFont, p[1], 1)
		}
	}
}

func mkFontFace(tenths int32, weight uintptr, face string) uintptr {
	h := uintptr(-(tenths*dpi)/720) & 0xffffffff
	f, _, _ := pCreateFontW.Call(h, 0, 0, 0, weight, 0, 0, 0,
		1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16(face))))
	return f
}

// ----------------------------------------------------------------- painting

func freeBuffer() {
	if memDC == 0 {
		return
	}
	pSelectObject.Call(memDC, oldBmp)
	pDeleteObject.Call(memBmp)
	pDeleteDC.Call(memDC)
	memDC, memBmp, oldBmp = 0, 0, 0
	cv = canvas{}
	bufW, bufH = 0, 0
}

func ensureBuffer(hdc uintptr, w, h int) {
	if memDC != 0 && bufW == w && bufH == h {
		return
	}
	freeBuffer()
	memDC, _, _ = pCreateCompatibleDC.Call(hdc)
	bi := bitmapInfoHeader{Width: int32(w), Height: int32(-h), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi))
	var bits unsafe.Pointer
	memBmp, _, _ = pCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if memBmp == 0 || bits == nil {
		pDeleteDC.Call(memDC)
		memDC = 0
		cv = canvas{}
		bufW, bufH = 0, 0
		return
	}
	oldBmp, _, _ = pSelectObject.Call(memDC, memBmp)
	cv = canvas{px: unsafe.Slice((*uint32)(bits), w*h), w: w, h: h}
	bufW, bufH = w, h
}

// resizeToContent re-applies the fixed client size at the current DPI,
// keeping the window where it is.
func resizeToContent(hwnd uintptr) {
	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	rc := rect{0, 0, sc(winW), sc(winH)}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	var wr rect
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	pSetWindowPos.Call(hwnd, 0, uintptr(wr.l), uintptr(wr.t),
		uintptr(rc.r-rc.l), uintptr(rc.b-rc.t), 0x0004|0x0010) // NOZORDER|NOACTIVATE
}

func drawText(hdc uintptr, s string, font uintptr, col rgbC, r rectF, flags uintptr, extra int32) {
	if s == "" {
		return
	}
	pSelectObject.Call(hdc, font)
	pSetTextColor.Call(hdc, colref(col))
	pSetBkMode.Call(hdc, 1) // TRANSPARENT
	if extra != 0 {
		pSetTextCharacterExtra.Call(hdc, uintptr(extra))
	}
	rc := rect{int32(r.x), int32(r.y), int32(r.x + r.w), int32(r.y + r.h)}
	t := syscall.StringToUTF16(s)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1),
		uintptr(unsafe.Pointer(&rc)), flags)
	if extra != 0 {
		pSetTextCharacterExtra.Call(hdc, 0)
	}
}

func ring(c *canvas, r rectF, rad float64) {
	o := scf(3)
	c.strokeRound(r.x-o, r.y-o, r.w+2*o, r.h+2*o, rad+o, scf(2), th.accent, 0.9, 0)
}

func docIcon(c *canvas, r rectF, col rgbC) {
	c.fillRound(r.x, r.y, r.w, r.h, scf(4), col, 0.14)
	c.strokeRound(r.x, r.y, r.w, r.h, scf(4), scf(1.3), col, 0.55, 0)
	for i := 0; i < 3; i++ {
		y := r.y + r.h*0.40 + float64(i)*scf(6)
		c.fillRound(r.x+r.w*0.22, y, r.w*0.56, scf(2), scf(1), col, 0.55)
	}
}

func loaded() bool { return curPath != "" }

func keyed() bool { return curKey.Key != "" }

func enabled(id int) bool {
	switch id {
	case itemNone:
		return false
	case itemTabKey, itemTabChart:
		return true
	}
	if page == pageChart {
		switch id {
		case itemAddImg, itemPasteImg, itemCreate:
			return !aiBusy
		case itemClearImg, itemReadAI:
			return !aiBusy && len(chartImgs) > 0
		}
		return false
	}
	if id == itemBrowse {
		return true
	}
	if id >= 0 && id < 12 || id == itemConvert {
		return keyed()
	}
	return false
}

func chipLabel(i int) string {
	if strings.HasSuffix(curKey.Key, "m") {
		return NAMES[i] + "m"
	}
	return NAMES[i]
}

func paint(hdc uintptr) {
	c := &cv
	c.clear(th.bg)
	c.fillRect(0, scf(78), float64(c.w), scf(1), th.border, 1)

	// page tabs (segmented control)
	seg := rectF{rTabs[0].x, rTabs[0].y, rTabs[1].x + rTabs[1].w - rTabs[0].x, rTabs[0].h}
	c.fillRound(seg.x, seg.y, seg.w, seg.h, scf(9), th.chip, 1)
	c.strokeRound(seg.x, seg.y, seg.w, seg.h, scf(9), scf(1), th.chipBorder, 1, 0)
	for i, r := range rTabs {
		id := itemTabKey + i
		if page == i {
			c.fillRound(r.x+scf(3), r.y+scf(3), r.w-scf(6), r.h-scf(6), scf(7), th.accent, 1)
		} else if hot == id {
			c.fillRound(r.x+scf(3), r.y+scf(3), r.w-scf(6), r.h-scf(6), scf(7), th.chipHot, 1)
		}
		if focus == id {
			ring(c, r, scf(9))
		}
	}

	if page == pageKey {
		paintKeyShapes(c)
	} else {
		paintChartShapes(c)
	}
	c.fillRect(0, scf(456), float64(c.w), scf(1), th.border, 1)

	// ---- text (GDI paints on top of the shapes) ----
	const sl = dtSingleLine | dtVCenter | dtNoPrefix
	drawText(hdc, "Chord Key Changer", fTitle, th.text,
		rectF{scf(padX), scf(16), scf(300), scf(30)}, dtLeft|sl, 0)
	sub := "Transpose a chart without touching its layout."
	if page == pageChart {
		sub = "Make a new chart from lyrics and chord screenshots."
	}
	drawText(hdc, sub, fSub, th.muted,
		rectF{scf(padX), scf(48), scf(340), scf(20)}, dtLeft|sl|dtEndEllipsis, 0)
	for i, label := range []string{"Change Key", "Make Chart"} {
		tc := th.text
		if page == i {
			tc = th.onAccent
		}
		drawText(hdc, label, fBtn, tc, rTabs[i], dtCenter|sl, 0)
	}

	if page == pageKey {
		paintKeyText(hdc)
	} else {
		paintChartText(hdc)
	}

	drawText(hdc, status, fStatus, th.muted,
		rectF{scf(padX), scf(464), scf(winW - 2*padX - 110), scf(22)}, dtLeft|sl|dtEndEllipsis, 0)
	drawText(hdc, Version, fStatus, th.muted,
		rectF{scf(winW - padX - 100), scf(464), scf(100), scf(22)}, dtRight|sl, 0)
}

func paintKeyShapes(c *canvas) {
	ok := loaded()
	rad := scf(12)
	c.shadow(rCard.x, rCard.y+scf(3), rCard.w, rCard.h, rad, scf(10), th.shadowA, th.shadowCol)
	if ok {
		c.fillRound(rCard.x, rCard.y, rCard.w, rCard.h, rad, th.surface, 1)
		c.strokeRound(rCard.x, rCard.y, rCard.w, rCard.h, rad, scf(1), th.border, 1, 0)
		docIcon(c, rectF{scf(48), scf(116), scf(26), scf(32)}, th.accent)
	} else {
		c.fillRound(rCard.x, rCard.y, rCard.w, rCard.h, rad, th.dropBg, 1)
		c.strokeRound(rCard.x, rCard.y, rCard.w, rCard.h, rad, scf(1.6), th.dropBorder, 1, scf(6))
		docIcon(c, rectF{scf(winW/2 - 14), scf(108), scf(28), scf(34)}, th.accent)
	}

	bb := th.surface
	if hot == itemBrowse {
		bb = th.chipHot
	}
	if pressed == itemBrowse {
		bb = mix(th.chipHot, th.accent, 0.12)
	}
	c.fillRound(rBrowse.x, rBrowse.y, rBrowse.w, rBrowse.h, scf(9), bb, 1)
	c.strokeRound(rBrowse.x, rBrowse.y, rBrowse.w, rBrowse.h, scf(9), scf(1), th.chipBorder, 1, 0)
	if focus == itemBrowse {
		ring(c, rBrowse, scf(9))
	}

	cur := -1
	if keyed() {
		cur = KeyRoot(curKey.Key)
	}
	for i := 0; i < 12; i++ {
		r, cr := rChips[i], scf(9)
		switch {
		case !keyed():
			c.fillRound(r.x, r.y, r.w, r.h, cr, th.chip, 0.55)
			c.strokeRound(r.x, r.y, r.w, r.h, cr, scf(1), th.chipBorder, 0.5, 0)
		case i == selKey:
			a := th.accent
			if hot == i {
				a = th.accentHi
			}
			c.shadow(r.x, r.y+scf(2), r.w, r.h, cr, scf(6), 0.25, th.accent)
			c.fillRound(r.x, r.y, r.w, r.h, cr, a, 1)
		default:
			bg := th.chip
			if hot == i {
				bg = th.chipHot
			}
			c.fillRound(r.x, r.y, r.w, r.h, cr, bg, 1)
			c.strokeRound(r.x, r.y, r.w, r.h, cr, scf(1), th.chipBorder, 1, 0)
		}
		if i == cur && i != selKey {
			d := scf(5)
			c.fillRound(r.x+r.w-scf(11), r.y+scf(7), d, d, d/2, th.accent, 0.85)
		}
		if focus == i {
			ring(c, r, cr)
		}
	}

	cr := scf(11)
	if enabled(itemConvert) {
		a := th.accent
		if hot == itemConvert {
			a = th.accentHi
		}
		if pressed == itemConvert {
			a = mix(th.accent, hexc(0x000000), 0.14)
		}
		c.shadow(rConvert.x, rConvert.y+scf(3), rConvert.w, rConvert.h, cr, scf(9), 0.30, th.accent)
		c.fillRound(rConvert.x, rConvert.y, rConvert.w, rConvert.h, cr, a, 1)
	} else {
		c.fillRound(rConvert.x, rConvert.y, rConvert.w, rConvert.h, cr, th.chip, 1)
		c.strokeRound(rConvert.x, rConvert.y, rConvert.w, rConvert.h, cr, scf(1), th.chipBorder, 1, 0)
	}
	if focus == itemConvert {
		ring(c, rConvert, cr)
	}

}

func paintKeyText(hdc uintptr) {
	ok := loaded()
	cur := -1
	if keyed() {
		cur = KeyRoot(curKey.Key)
	}
	const sl = dtSingleLine | dtVCenter | dtNoPrefix
	if ok {
		drawText(hdc, filepath.Base(curPath), fName, th.text,
			rectF{scf(86), scf(114), scf(486), scf(22)}, dtLeft|sl|dtEndEllipsis, 0)
		drawText(hdc, "CURRENT KEY", fSection, th.muted,
			rectF{scf(86), scf(142), scf(200), scf(16)}, dtLeft|sl, sc(0.7))
		kv, kc, src := curKey.Key, th.accent, "detected from "+curKey.Source
		if !keyed() {
			kv, kc, src = "\u2014", th.muted, detectErr
		}
		drawText(hdc, kv, fKey, kc,
			rectF{scf(86), scf(158), scf(140), scf(34)}, dtLeft|sl, 0)
		drawText(hdc, src, fLabel, th.muted,
			rectF{scf(208), scf(168), scf(364), scf(20)}, dtLeft|sl|dtEndEllipsis, 0)
	} else {
		drawText(hdc, "Drop a chord chart here", fName, th.text,
			rectF{rCard.x, scf(150), rCard.w, scf(22)}, dtCenter|sl, 0)
		drawText(hdc, "PDF or Word (.docx)  —  or click Browse", fLabel, th.muted,
			rectF{rCard.x, scf(172), rCard.w, scf(20)}, dtCenter|sl, 0)
	}

	bl := "Browse…"
	if ok {
		bl = "Change file…"
	}
	drawText(hdc, bl, fBtn, th.text, rBrowse, dtCenter|sl, 0)
	drawText(hdc, "Accepts .pdf and .docx", fLabel, th.muted,
		rectF{scf(padX), scf(220), scf(320), scf(20)}, dtLeft|sl, 0)
	drawText(hdc, "CHANGE TO", fSection, th.muted,
		rectF{scf(padX), scf(260), scf(320), scf(18)}, dtLeft|sl, sc(0.9))

	for i := 0; i < 12; i++ {
		tc := th.text
		switch {
		case !keyed():
			tc = th.muted
		case i == selKey:
			tc = th.onAccent
		}
		drawText(hdc, chipLabel(i), fChip, tc, rChips[i], dtCenter|sl, 0)
	}

	ct, cl := th.muted, "Convert…"
	if enabled(itemConvert) {
		ct = th.onAccent
		if selKey >= 0 && selKey != cur {
			cl = "Convert to " + chipLabel(selKey) + "…"
		}
	}
	drawText(hdc, cl, fBtn, ct, rConvert, dtCenter|sl, 0)

}

// button paints a rounded button; primary ones are filled with the accent.
func button(c *canvas, r rectF, id int, primary bool) {
	cr := scf(10)
	switch {
	case !enabled(id):
		c.fillRound(r.x, r.y, r.w, r.h, cr, th.chip, 0.6)
		c.strokeRound(r.x, r.y, r.w, r.h, cr, scf(1), th.chipBorder, 0.6, 0)
	case primary:
		a := th.accent
		if hot == id {
			a = th.accentHi
		}
		if pressed == id {
			a = mix(th.accent, hexc(0x000000), 0.14)
		}
		c.shadow(r.x, r.y+scf(3), r.w, r.h, cr, scf(8), 0.28, th.accent)
		c.fillRound(r.x, r.y, r.w, r.h, cr, a, 1)
	default:
		bg := th.surface
		if hot == id {
			bg = th.chipHot
		}
		if pressed == id {
			bg = mix(th.chipHot, th.accent, 0.12)
		}
		c.fillRound(r.x, r.y, r.w, r.h, cr, bg, 1)
		c.strokeRound(r.x, r.y, r.w, r.h, cr, scf(1), th.chipBorder, 1, 0)
	}
	if focus == id {
		ring(c, r, cr)
	}
}

func buttonText(hdc uintptr, r rectF, id int, primary bool, label string) {
	tc := th.text
	switch {
	case !enabled(id):
		tc = th.muted
	case primary:
		tc = th.onAccent
	}
	drawText(hdc, label, fBtn, tc, r, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, 0)
}

func paintChartShapes(c *canvas) {
	for _, r := range []rectF{rEditBox, rKeyBox} {
		c.fillRound(r.x, r.y, r.w, r.h, scf(10), th.surface, 1)
		c.strokeRound(r.x, r.y, r.w, r.h, scf(10), scf(1), th.border, 1, 0)
	}
	button(c, rAddImg, itemAddImg, false)
	button(c, rPasteImg, itemPasteImg, false)
	button(c, rClearImg, itemClearImg, false)
	button(c, rReadAI, itemReadAI, len(chartImgs) > 0)
	button(c, rCreate, itemCreate, true)
}

func paintChartText(hdc uintptr) {
	const sl = dtSingleLine | dtVCenter | dtNoPrefix
	drawText(hdc, "CHART TEXT", fSection, th.muted,
		rectF{scf(padX), scf(90), scf(200), scf(16)}, dtLeft|sl, sc(0.9))
	drawText(hdc, "Paste lyrics, or edit:  [C]我来到[D]神面前", fLabel, th.muted,
		rectF{scf(240), scf(88), scf(winW - padX - 240), scf(20)}, dtRight|sl|dtEndEllipsis, 0)

	drawText(hdc, "CHORD SCREENSHOTS", fSection, th.muted,
		rectF{scf(padX), scf(352), scf(160), scf(16)}, dtLeft|sl, sc(0.9))
	info := "none yet — drop images on the window, or paste a screenshot"
	if n := len(chartImgNames); n > 0 {
		info = fmt.Sprintf("%d added: %s", n, strings.Join(chartImgNames, ", "))
	}
	drawText(hdc, info, fLabel, th.muted,
		rectF{scf(170), scf(350), scf(winW - padX - 170), scf(20)}, dtLeft|sl|dtEndEllipsis, 0)

	buttonText(hdc, rAddImg, itemAddImg, false, "Add images…")
	buttonText(hdc, rPasteImg, itemPasteImg, false, "Paste image")
	buttonText(hdc, rClearImg, itemClearImg, false, "Clear")
	ai := "Read with AI"
	if aiBusy {
		ai = "Reading…"
	}
	buttonText(hdc, rReadAI, itemReadAI, len(chartImgs) > 0, ai)
	buttonText(hdc, rCreate, itemCreate, true, "Create Word…")
	drawText(hdc, "API key", fLabel, th.muted,
		rectF{scf(226), rKeyBox.y, scf(70), rKeyBox.h}, dtLeft|sl, 0)
}

func onPaint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var rc rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w, h := int(rc.r-rc.l), int(rc.b-rc.t)
	if w > 0 && h > 0 {
		ensureBuffer(hdc, w, h)
		if memDC != 0 {
			pGdiFlush.Call()
			paint(memDC)
			pBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, srcCopy)
		}
	}
	pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func repaint() { pInvalidateRect.Call(hMain, 0, 0) }

// -------------------------------------------------------------- interaction

func tabOrder() []int {
	o := []int{itemTabKey, itemTabChart}
	if page == pageChart {
		return append(o, itemAddImg, itemPasteImg, itemClearImg, itemReadAI, itemCreate)
	}
	o = append(o, itemBrowse)
	for i := 0; i < 12; i++ {
		o = append(o, i)
	}
	return append(o, itemConvert)
}

func hitTest(x, y float64) int {
	for i, r := range rTabs {
		if r.hit(x, y) {
			return itemTabKey + i
		}
	}
	if page == pageChart {
		for _, b := range []struct {
			r  rectF
			id int
		}{{rAddImg, itemAddImg}, {rPasteImg, itemPasteImg}, {rClearImg, itemClearImg},
			{rReadAI, itemReadAI}, {rCreate, itemCreate}} {
			if b.r.hit(x, y) {
				return b.id
			}
		}
		return itemNone
	}
	if rConvert.hit(x, y) {
		return itemConvert
	}
	if rBrowse.hit(x, y) {
		return itemBrowse
	}
	for i := 0; i < 12; i++ {
		if rChips[i].hit(x, y) {
			return i
		}
	}
	return itemNone
}

func moveFocus(dir int) {
	order := tabOrder()
	idx := -1
	for i, id := range order {
		if id == focus {
			idx = i
			break
		}
	}
	n := len(order)
	for k := 1; k <= n; k++ {
		j := ((idx+dir*k)%n + n) % n
		if enabled(order[j]) {
			focus = order[j]
			repaint()
			return
		}
	}
}

func activate(id int) {
	switch {
	case id == itemTabKey:
		setPage(pageKey)
	case id == itemTabChart:
		setPage(pageChart)
	case id == itemAddImg:
		safe(onAddImages)
	case id == itemPasteImg:
		safe(onPasteImage)
	case id == itemClearImg:
		chartImgs, chartImgNames, pastedCount = nil, nil, 0
		status = "Screenshots cleared."
		repaint()
	case id == itemReadAI:
		safe(onReadAI)
	case id == itemCreate:
		safe(onCreateChart)
	case id == itemBrowse:
		safe(onBrowse)
	case id == itemConvert:
		safe(onConvert)
	case id >= 0 && id < 12 && keyed():
		selKey = id
		if id == KeyRoot(curKey.Key) {
			status = "That is the chart's current key — pick a different one."
		} else {
			status = "Ready: " + curKey.Key + " → " + chipLabel(id) + ". Click Convert."
		}
		repaint()
	}
}

func applyTheme() {
	nt := lightTheme()
	if systemDark() {
		nt = darkTheme()
	}
	if nt.dark == th.dark && hMain != 0 {
		return
	}
	th = nt
	makeEditBrush()
	if hMain != 0 {
		applyChrome(hMain, th.dark)
		repaint()
		for _, h := range []uintptr{hEdit, hKeyEdit} {
			if h != 0 {
				pInvalidateRect.Call(h, 0, 1)
			}
		}
	}
}

// ---------------------------------------------------------------- file work

func msgBox(text, title string, flags uintptr) uintptr {
	r, _, _ := pMessageBoxW.Call(hMain, uintptr(unsafe.Pointer(u16(text))),
		uintptr(unsafe.Pointer(u16(title))), flags)
	return r
}

func safe(f func()) {
	defer func() {
		if r := recover(); r != nil {
			msgBox(fmt.Sprintf("Something went wrong:\n%v", r), "Chord Key Changer", mbIconError)
		}
	}()
	f()
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

func loadFile(p string) {
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".pdf" && ext != ".docx" {
		status = "Not a supported file — use a PDF or Word (.docx) chart."
		msgBox("Only PDF and Word (.docx) chord charts can be converted.",
			"Chord Key Changer", mbIconWarn)
		repaint()
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		msgBox("Could not open the file:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	curPath, curData = p, data
	focus, hot = itemNone, itemNone
	k, err := Detect(p, data)
	if err != nil {
		curKey, selKey = KeyInfo{}, -1
		detectErr = err.Error()
		status = "Couldn't work out the key: " + detectErr
		repaint()
		return
	}
	curKey, detectErr = k, ""
	selKey = KeyRoot(k.Key)
	status = "Detected " + k.Key + " from " + k.Source + ". Pick the new key, then Convert."
	repaint()
}

func onBrowse() {
	p := fileDialog(false, "Choose a chord chart",
		"Chord charts (*.pdf, *.docx)|*.pdf;*.docx|PDF files (*.pdf)|*.pdf|Word files (*.docx)|*.docx|",
		"", "", "")
	if p != "" {
		loadFile(p)
	}
}

func onConvert() {
	if !keyed() || selKey < 0 || selKey >= 12 {
		return
	}
	cur := KeyRoot(curKey.Key)
	if selKey == cur {
		msgBox("The chart is already in this key. Pick a different key.",
			"Chord Key Changer", mbIconInfo)
		return
	}
	newKey := chipLabel(selKey)
	ext := strings.ToLower(filepath.Ext(curPath))
	filter := "PDF files (*.pdf)|*.pdf|"
	if ext == ".docx" {
		filter = "Word files (*.docx)|*.docx|"
	}
	out := fileDialog(true, "Save the "+newKey+" key version as", filter,
		filepath.Dir(curPath), OutputName(curPath, newKey), strings.TrimPrefix(ext, "."))
	if out == "" {
		return
	}
	if strings.EqualFold(filepath.Clean(out), filepath.Clean(curPath)) {
		msgBox("Please save under a different name so the original isn't overwritten.",
			"Chord Key Changer", mbIconWarn)
		return
	}
	status = "Converting…"
	repaint()
	pUpdateWindow.Call(hMain)

	b, cnt, warns, err := Convert(curPath, curData, selKey-cur)
	if err != nil {
		status = "Conversion failed."
		repaint()
		msgBox("Conversion failed:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	if err := os.WriteFile(out, b, 0644); err != nil {
		status = "Could not save."
		repaint()
		msgBox("Could not save the file (is it open in another program?)\n\n"+err.Error(),
			"Chord Key Changer", mbIconError)
		return
	}
	status = fmt.Sprintf("Done: %s → %s, %d chord(s) changed.", curKey.Key, newKey, cnt)
	repaint()

	text := fmt.Sprintf("Saved:\n%s\n\n%s → %s, %d chord(s) changed.", out, curKey.Key, newKey, cnt)
	icon := uintptr(mbIconInfo)
	if len(warns) > 0 {
		text += "\n\nPlease check:\n• " + strings.Join(warns, "\n• ")
		icon = mbIconWarn
	}
	text += "\n\nOpen the file now?"
	if msgBox(text, "Chord Key Changer", mbYesNo|icon) == idYes {
		pShellExecuteW.Call(hMain, uintptr(unsafe.Pointer(u16("open"))),
			uintptr(unsafe.Pointer(u16(out))), 0, 0, 1)
	}
}

// ------------------------------------------------------------------ wndproc

func mousePos(lParam uintptr) (float64, float64) {
	return float64(int16(lParam & 0xffff)), float64(int16((lParam >> 16) & 0xffff))
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmEraseBkgnd:
		return 1

	case wmPaint:
		onPaint(hwnd)
		return 0

	case wmMouseMove:
		x, y := mousePos(lParam)
		h := hitTest(x, y)
		if !enabled(h) {
			h = itemNone
		}
		if h != hot {
			hot = h
			repaint()
		}
		if !tracking {
			tme := trackMouseEvent{dwFlags: 2, hwndTrack: hwnd} // TME_LEAVE
			tme.cbSize = uint32(unsafe.Sizeof(tme))
			pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			tracking = true
		}
		return 0

	case wmMouseLeave:
		tracking = false
		if hot != itemNone {
			hot = itemNone
			repaint()
		}
		return 0

	case wmSetCursor:
		if hot != itemNone {
			pSetCursor.Call(hCurHand)
			return 1
		}

	case wmLButtonDown:
		pSetFocus.Call(hwnd)
		x, y := mousePos(lParam)
		h := hitTest(x, y)
		if enabled(h) {
			pressed, focus = h, h
		} else {
			pressed, focus = itemNone, itemNone
		}
		repaint()
		return 0

	case wmLButtonUp:
		x, y := mousePos(lParam)
		p := pressed
		pressed = itemNone
		repaint()
		if p != itemNone && hitTest(x, y) == p && enabled(p) {
			activate(p)
		}
		return 0

	case wmKeyDown:
		switch wParam {
		case vkTab:
			d := 1
			if st, _, _ := pGetKeyState.Call(vkShift); st&0x8000 != 0 {
				d = -1
			}
			moveFocus(d)
			return 0
		case vkReturn, vkSpace:
			if enabled(focus) {
				activate(focus)
			}
			return 0
		case vkLeft, vkRight, vkUp, vkDown:
			if page == pageKey && focus >= 0 && focus < 12 && keyed() {
				n := focus
				switch wParam {
				case vkLeft:
					n--
				case vkRight:
					n++
				case vkUp:
					n -= 6
				case vkDown:
					n += 6
				}
				if n >= 0 && n < 12 {
					focus = n
					activate(n)
				}
				return 0
			}
		}

	case wmDropFiles:
		var paths []string
		n, _, _ := pDragQueryFileW.Call(wParam, 0xFFFFFFFF, 0, 0)
		for i := uintptr(0); i < n; i++ {
			buf := make([]uint16, 4096)
			if r, _, _ := pDragQueryFileW.Call(wParam, i,
				uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r > 0 {
				paths = append(paths, syscall.UTF16ToString(buf))
			}
		}
		pDragFinish.Call(wParam)
		safe(func() { onDropped(paths) })
		return 0

	case wmCtlColorEdit:
		pSetTextColor.Call(wParam, colref(th.text))
		pSetBkColor.Call(wParam, colref(th.surface))
		return hEditBrush

	case wmAIDone:
		onAIDone()
		return 0

	case wmSettingChange:
		applyTheme()
		return 0

	case wmDpiChanged:
		if d := int32(wParam & 0xffff); d > 0 {
			dpi = d
		}
		makeFonts()
		layout()
		resizeToContent(hwnd)
		repaint()
		return 0

	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func main() {
	runtime.LockOSThread()
	// Per-monitor DPI v2 where it exists (Win10 1703+), else the legacy call.
	if tryCall(pSetProcessDpiCtx, ^uintptr(3)) == 0 { // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
		pSetProcessDPIAware.Call()
	}
	enableVisualStyles()
	hInst, _, _ = pGetModuleHandleW.Call(0)

	dc, _, _ := pGetDC.Call(0)
	if d, _, _ := pGetDeviceCaps.Call(dc, 88); d > 0 { // LOGPIXELSX
		dpi = int32(d)
	}
	pReleaseDC.Call(0, dc)

	th = lightTheme()
	if systemDark() {
		th = darkTheme()
	}
	makeEditBrush()
	settings = loadSettings()
	makeFonts()
	layout()

	hCurArrow, _, _ = pLoadCursorW.Call(0, 32512)
	hCurHand, _, _ = pLoadCursorW.Call(0, idcHand)
	ico, _, _ := pLoadIconW.Call(0, 32512)

	cls := u16("ChordKeyChangerWnd")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: hInst,
		hCursor: hCurArrow, hIcon: ico, hIconSm: ico, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	rc := rect{0, 0, sc(winW), sc(winH)}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	hMain, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(u16("Chord Key Changer"))), style,
		0x80000000, 0x80000000, uintptr(rc.r-rc.l), uintptr(rc.b-rc.t), 0, 0, hInst, 0)

	if d := tryCall(pGetDpiForWindow, hMain); d > 0 && int32(d) != dpi {
		dpi = int32(d)
		makeFonts()
		layout()
		rc = rect{0, 0, sc(winW), sc(winH)}
		pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
		pSetWindowPos.Call(hMain, 0, 0, 0, uintptr(rc.r-rc.l), uintptr(rc.b-rc.t), 0x0002|0x0004|0x0010)
	}

	createEdits()
	applyChrome(hMain, th.dark)
	pDragAcceptFiles.Call(hMain, 1)
	pShowWindow.Call(hMain, 1)
	pUpdateWindow.Call(hMain)

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
