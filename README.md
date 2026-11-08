# Chord Key Changer

Windows app that changes the key of a worship song chord chart (**PDF** or **Word .docx**) without touching fonts, colours or layout.

## Use
Full guide: [USER_GUIDE.md](USER_GUIDE.md)

1. **Drag the chord chart onto the window**, or click **Browse...**.
2. The **current key** is detected from the `Key:` line (or the file name, or the first chord as a fallback) and marked with a dot.
3. Click the new key under **Change to** (arrow keys work too).
4. **Convert...** → choose where to save (name is pre-filled, e.g. `F Key - Song.pdf`).

The window follows the Windows light/dark app theme and scales with per-monitor DPI.

Download the latest `Chord Key Changer.exe` from **[Releases](../../releases)**. First run: Windows SmartScreen may warn because the exe is unsigned → *More info → Run anyway*.

## Chord spelling (house rule)
`C  C#  D  D#  E  F  F#  G  G#  A  Bb  B` — **Bb is the only flat**; Db→C#, Eb→D#, Ab→G#.

## What it changes
- Only text runs made purely of chord symbols (`| C/G`, `Am7`, `(D/C)`, `Key: G`…). Lyrics, jianpu numbers, section labels untouched.
- PDF: rewrites chord text, nudges following chords right if a chord gets wider (C→Bb), and fills in missing glyph widths (`b`, `#`) in the font's width table so nothing overlaps.
- If the chord font is an embedded subset missing a symbol, the app warns — convert the .docx instead.

## Build
Pure Go standard library, no dependencies.

```bash
# Windows exe (from any OS)
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o "Chord Key Changer.exe" .

# Command-line test harness (Linux/macOS): IN OUT TARGET_KEY
go build -o chordkey . && ./chordkey "G Key - Song.pdf" out.pdf F
```

## Release a new version
```bash
git tag v1.0.1 && git push origin v1.0.1
```
GitHub Actions builds the exe and attaches it to a new Release automatically.

## Files
| File | Purpose |
|---|---|
| `gui_windows.go` | Windows window and custom-drawn UI, Open/Save dialogs (Win32, no libraries) |
| `transpose.go` | Chord transposition, key detection, PDF + DOCX conversion |
| `pdf.go` | Minimal PDF reader/writer (xref tables/streams, object streams) |
| `cli.go` | Command-line harness for testing on non-Windows |
