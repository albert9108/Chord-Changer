//go:build windows

package main

// "Make Chart" page: chart text box, chord screenshots, Read with AI, Create Word.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func createEdits() {
	mk := func(style uintptr, id uintptr) uintptr {
		h, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(u16("EDIT"))), 0,
			wsChild|wsTabStop|style, 0, 0, 10, 10, hMain, id, hInst, 0)
		return h
	}
	hEdit = mk(wsVScroll|esMultiline|esAutoVScroll|esWantReturn|esNoHideSel, 101)
	hKeyEdit = mk(esPassword|esAutoHScroll, 102)
	pSendMessageW.Call(hEdit, emSetLimit, 1<<20, 0)
	pSendMessageW.Call(hEdit, wmSetFont, fEdit, 1)
	pSendMessageW.Call(hKeyEdit, wmSetFont, fKey2, 1)
	setText(hKeyEdit, settings.APIKey)
	placeEdits()
}

func makeEditBrush() {
	if hEditBrush != 0 {
		pDeleteObject.Call(hEditBrush)
	}
	hEditBrush, _, _ = pCreateSolidBrush.Call(colref(th.surface))
}

func setPage(p int) {
	page = p
	focus, hot, pressed = itemNone, itemNone, itemNone
	show := uintptr(0)
	if p == pageChart {
		show = 5 // SW_SHOW
	}
	for _, h := range []uintptr{hEdit, hKeyEdit} {
		pShowWindow.Call(h, show)
	}
	if p == pageChart {
		status = "Paste the lyrics, add chord screenshots, then Read with AI."
		pSetFocus.Call(hEdit)
	} else {
		status = "Drop a chord chart on the window, or click Browse."
		if loaded() {
			status = "Pick the new key, then Convert."
		}
		pSetFocus.Call(hMain)
	}
	repaint()
}

func getText(h uintptr) string {
	n, _, _ := pGetWindowTextLenW.Call(h)
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return strings.ReplaceAll(syscall.UTF16ToString(buf), "\r\n", "\n")
}

func crlf(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}

func setText(h uintptr, s string) {
	pSetWindowTextW.Call(h, uintptr(unsafe.Pointer(u16(crlf(s)))))
}

// replaceAllUndoable swaps the box's text but keeps Ctrl+Z able to bring the
// old text back.
func replaceAllUndoable(h uintptr, s string) {
	pSendMessageW.Call(h, emSetSel, 0, ^uintptr(0))
	pSendMessageW.Call(h, emReplaceSel, 1, uintptr(unsafe.Pointer(u16(crlf(s)))))
}

func addImage(name string, im aiImage) {
	chartImgs = append(chartImgs, im)
	chartImgNames = append(chartImgNames, name)
}

func onDropped(paths []string) {
	var imgs, charts []string
	for _, p := range paths {
		switch {
		case imageMediaType(p) != "":
			imgs = append(imgs, p)
		case strings.EqualFold(filepath.Ext(p), ".txt"):
			if b, err := os.ReadFile(p); err == nil {
				if page != pageChart {
					setPage(pageChart)
				}
				replaceAllUndoable(hEdit, string(b))
			}
		default:
			charts = append(charts, p)
		}
	}
	if len(imgs) > 0 {
		if page != pageChart {
			setPage(pageChart)
		}
		addImageFiles(imgs)
		return
	}
	if len(charts) > 0 {
		if page != pageKey {
			setPage(pageKey)
		}
		loadFile(charts[0])
	}
}

func addImageFiles(paths []string) {
	var bad []string
	for _, p := range paths {
		im, err := loadAIImage(p)
		if err != nil {
			bad = append(bad, filepath.Base(p)+": "+err.Error())
			continue
		}
		addImage(filepath.Base(p), im)
	}
	status = fmt.Sprintf("%d screenshot(s) ready. Click Read with AI.", len(chartImgs))
	repaint()
	if len(bad) > 0 {
		msgBox("Some files were skipped:\n• "+strings.Join(bad, "\n• "), "Chord Key Changer", mbIconWarn)
	}
}

func onAddImages() {
	buf := make([]uint16, 65536)
	ofn := openFileName{
		hwndOwner:   hMain,
		lpstrFilter: u16multi("Images (*.png, *.jpg, *.webp, *.gif)|*.png;*.jpg;*.jpeg;*.webp;*.gif|"),
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  u16("Choose chord screenshots"),
		Flags: ofnExplorer | ofnPathMustExist | ofnFileMustExist | ofnNoChangeDir |
			ofnHideReadOnly | ofnAllowMulti,
	}
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return
	}
	// One file: full path. Several: folder \0 name \0 name \0\0.
	var parts []string
	start := 0
	for i, c := range buf {
		if c == 0 {
			if i == start {
				break
			}
			parts = append(parts, syscall.UTF16ToString(buf[start:i]))
			start = i + 1
		}
	}
	var paths []string
	if len(parts) == 1 {
		paths = parts
	} else if len(parts) > 1 {
		for _, n := range parts[1:] {
			paths = append(paths, filepath.Join(parts[0], n))
		}
	}
	addImageFiles(paths)
}

// clipboardPNG reads a bitmap from the clipboard (e.g. Win+Shift+S) as PNG.
func clipboardPNG() ([]byte, error) {
	const cfDIB = 8
	if r, _, _ := pIsClipFmtAvailable.Call(cfDIB); r == 0 {
		return nil, fmt.Errorf("there is no picture on the clipboard — take a screenshot with Win+Shift+S first")
	}
	if r, _, _ := pOpenClipboard.Call(hMain); r == 0 {
		return nil, fmt.Errorf("could not open the clipboard")
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfDIB)
	if h == 0 {
		return nil, fmt.Errorf("could not read the clipboard picture")
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return nil, fmt.Errorf("could not read the clipboard picture")
	}
	defer pGlobalUnlock.Call(h)
	size, _, _ := pGlobalSize.Call(h)
	data := unsafe.Slice((*byte)(unsafe.Add(nil, ptr)), int(size)) // GlobalLock memory, read before unlock
	return dibToPNG(data)
}

func onPasteImage() {
	b, err := clipboardPNG()
	if err != nil {
		msgBox(err.Error(), "Chord Key Changer", mbIconInfo)
		return
	}
	if len(b) > aiMaxImage {
		msgBox("The pasted picture is larger than 5 MB — take a smaller screenshot.", "Chord Key Changer", mbIconWarn)
		return
	}
	pastedCount++
	addImage(fmt.Sprintf("Pasted %d", pastedCount), aiImage{"image/png", b})
	status = fmt.Sprintf("%d screenshot(s) ready. Click Read with AI.", len(chartImgs))
	repaint()
}

func onReadAI() {
	if aiBusy || len(chartImgs) == 0 {
		return
	}
	key := strings.TrimSpace(getText(hKeyEdit))
	if key == "" {
		msgBox("Enter your Claude API key in the API key box first.\n\nCreate one at console.anthropic.com → API Keys.",
			"Chord Key Changer", mbIconInfo)
		pSetFocus.Call(hKeyEdit)
		return
	}
	if key != settings.APIKey {
		settings.APIKey = key
		if err := saveSettings(settings); err != nil {
			status = "Could not save the API key: " + err.Error()
		}
	}
	lyrics := getText(hEdit)
	imgs := append([]aiImage(nil), chartImgs...)
	model := settings.Model
	aiBusy = true
	status = "Reading the screenshots with AI… (usually under a minute)"
	focus = itemNone
	repaint()
	go func() {
		text, err := ReadChartWithAI(key, model, lyrics, imgs)
		aiMu.Lock()
		aiResult, aiErr = text, err
		aiMu.Unlock()
		pPostMessageW.Call(hMain, wmAIDone, 0, 0)
	}()
}

func onAIDone() {
	aiMu.Lock()
	text, err := aiResult, aiErr
	aiMu.Unlock()
	aiBusy = false
	if err != nil {
		status = "AI reading failed."
		repaint()
		msgBox("Could not read the screenshots:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	replaceAllUndoable(hEdit, text)
	status = "Done — check the chord positions, fix anything (Ctrl+Z undoes), then Create Word."
	repaint()
}

func onCreateChart() {
	c := ParseChart(getText(hEdit))
	lyricLines := 0
	for _, l := range c.Lines {
		if l.Kind == "lyric" {
			lyricLines++
		}
	}
	if lyricLines == 0 {
		msgBox("There are no lyric lines yet. Paste the lyrics (and Read with AI) first.",
			"Chord Key Changer", mbIconInfo)
		return
	}
	b, warns, err := BuildChartDocx(c)
	if err != nil {
		msgBox("Could not build the chart:\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	out := fileDialog(true, "Save the chart as", "Word files (*.docx)|*.docx|",
		lastChartDir, ChartFileName(c), "docx")
	if out == "" {
		return
	}
	if err := os.WriteFile(out, b, 0644); err != nil {
		msgBox("Could not save the file (is it open in Word?)\n\n"+err.Error(), "Chord Key Changer", mbIconError)
		return
	}
	lastChartDir = filepath.Dir(out)
	status = "Saved " + filepath.Base(out)
	repaint()
	text := "Saved:\n" + out
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
