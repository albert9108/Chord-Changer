# Chord Key Changer — User Guide

**Version:** v1.0.0 · **Platform:** Windows 10 / 11 (64-bit) · **Install:** none, single `.exe`

Changes the key of a worship song chord chart (**PDF** or **Word .docx**) in one click. Only the chord letters change; fonts, colours, layout, lyrics and jianpu numbers stay exactly the same.

---

## 1. Quick start

1. Double-click **`Chord Key Changer.exe`**.
2. Click **Browse...** and pick the chord chart (`.pdf` or `.docx`).
3. Check **Current key**: the app detects it automatically.
4. Pick the new key under **Change to**.
5. Click **Convert...**, choose where to save, then click **Save**.
6. Click **Yes** to open the new file, or **No** to finish.

The save name is filled in for you, e.g. `G Key - Song.pdf` becomes **`F Key - Song.pdf`**. You can change the name or folder before saving.

---

## 2. How the key is detected

The app checks these in order and shows which one it used:

| Order | Source | Example | Shown as |
|---|---|---|---|
| 1 | The `Key:` line on the chart | `Key: G` | *(from the "Key:" line)* |
| 2 | The file name | `G Key - 在祢没有难成的事.pdf` | *(from the file name)* |
| 3 | The first chord on the chart | `\|: Em7 ...` | *(guessed from the first chord — please check)* |

If it shows **"guessed"**, check that the key is right before converting.

---

## 3. Chord spelling rules

The 12 keys are always written as:

```
C   C#   D   D#   E   F   F#   G   G#   A   Bb   B
```

- **Bb** is the only flat.
- **Db → C#**, **Eb → D#**, **Ab → G#**.
- Slash chords and extensions are handled too: `C/G → Bb/F`, `Fmaj7 → D#maj7`, `Am7 → Gm7`, `(D/C) → (C/Bb)`.

### Common changes

| From → To | Semitones | Example |
|---|---|---|
| C → D | +2 | C, F, G7, Am → D, G, A7, Bm |
| G → F | −2 | G, C, D, Em → F, Bb, C, Dm |
| G → A | +2 | G, C, D, Em → A, D, E, F#m |
| D → C | −2 | D, G, A, Bm → C, F, G, Am |

---

## 4. What changes, what doesn't

| Changed | Not changed |
|---|---|
| Chord letters (`G`, `C/G`, `Em7`, `Fmaj7`…) | Fonts, sizes, colours |
| The `Key:` value in the header | Lyrics (Chinese / English) |
| Spacing after a chord that gets wider (e.g. `C` → `Bb`), so chords don't overlap | Jianpu numbers (`1 2 3…`), tempo, section labels |
| | Your original file: the app never overwrites it |

---

## 5. Troubleshooting

| Problem | Fix |
|---|---|
| **"Windows protected your PC"** | Click **More info → Run anyway**. It only appears because the app isn't digitally signed. |
| **The .exe vanishes after downloading** | Antivirus removed it. Open **Windows Security → Virus & threat protection → Protection history**, find *Chord Key Changer* and click **Allow**. Or extract it again from `Chord-Key-Changer-v1.0.0.zip` (right-click → **Extract All**). |
| **"Couldn't find the key"** | The chart has no `Key:` line and no key in the file name. Rename the file to start with the key, e.g. `G Key - Song.pdf`, then Browse again. |
| **"This PDF is encrypted/protected"** | Export the PDF from Word again without a password, or use the `.docx`. |
| **Warning: chord font is embedded without a symbol** | The PDF's font is missing `b` or `#`. Convert the **Word (.docx)** version instead. |
| **"Could not save the file"** | The target file is open in another program (e.g. a PDF reader). Close it, or save under a different name. |
| **A chord looks wrong or overlaps** | Send the original file for checking. |

---

## 6. Tips

- Always keep the original chart. Convert from the original, not from an already-converted copy, so the spacing stays tight.
- **Word (.docx)** is the safest source if you have it. PDF works for charts exported from Word.
- Chart has a capo note, e.g. `(capo E)`? The app leaves it unchanged. Update it by hand if needed.

---

## 7. Version history

| Version | Date | Changes |
|---|---|---|
| v1.0.0 | 2026-11-08 | First release: PDF + DOCX, key detection, Open/Save dialogs, Bb-only-flat spelling |

---

## 8. For maintainers

- **Source:** Go, standard library only. Files: `gui_windows.go` (window/dialogs), `transpose.go` (chords, key detection, conversion), `pdf.go` (PDF reader/writer).
- **Build:** `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui -X main.Version=v1.0.1" -o "Chord Key Changer.exe" .`
- **Release:** `git tag v1.0.1 && git push origin v1.0.1`. GitHub Actions builds the exe and publishes a new Release.
