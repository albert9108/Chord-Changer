# Chord Key Changer — User Guide

**Version:** v1.1.0 · **Platform:** Windows 10 / 11 (64-bit) · **Install:** none, single `.exe`

Two tabs:

- **Change Key** — changes the key of a worship song chord chart (**PDF** or **Word .docx**) in one click. Only the chord letters change; fonts, colours, layout, lyrics and jianpu numbers stay exactly the same.
- **Make Chart** — makes a new chart in the house layout from lyrics and chord screenshots (see section 7).

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

## 7. Make Chart (new chart from screenshots)

1. Click **Make Chart** at the top right.
2. Paste the lyrics into **Chart text** (Traditional is fine — it is converted to Simplified).
3. Add the chord screenshots, any website format (chords above lyrics, guitar TAB, jianpu):
   - drag image files onto the window, or **Add images…**, or
   - take a screenshot with **Win+Shift+S** and click **Paste image**.
4. Paste your **Claude API key** in the API key box (once — it is remembered).
5. Click **Read with AI** (10–40 seconds). The chart text appears in the box.
6. Check the chord positions and fix anything by typing. **Ctrl+Z** brings back the text from before.
7. **Create Word…** → save. The name is filled in, e.g. `G Key - 海屿你.docx`.

The Word file can go straight into **Change Key** for other keys.

### Chart text format

```
title: 海屿你
tempo: 75
key: G
intro: | G | D/F# | Em | C |
## Verse
[G]从不主动示弱
[D/F#]我们的过去 [Em]分分合合太多
===
## Chorus
[C]我不知为何 [D]疯狂对你执着
[D]求你别离开我　[G]
```

| Write | Means |
|---|---|
| `[C]` before a syllable | a bar starts on that syllable with chord C |
| `[Em7 D]` | two chords in one bar |
| `## Verse` | boxed section name |
| `===` | start the second column here |
| `(…)` line | small grey note |
| `intro:` with `//` | a second intro row, e.g. `\| C \| D \| // \| Am7 \| D \|` |

Rules the AI follows: Simplified Chinese; one chord per bar (no repeats of a held chord); sharps except Bb; capo charts keep the shape key; at most 13 Chinese characters a line.

### API key and cost

- Get a key at **console.anthropic.com → API Keys**. Usage is billed per request — roughly a few US cents per song, no monthly fee.
- The key is saved in `%APPDATA%\Chord Key Changer\settings.json` (plain text in your Windows profile). The `ANTHROPIC_API_KEY` environment variable also works.
- Needs internet only for **Read with AI**; typing a chart and **Create Word** work offline.

---

## 8. Version history

| Version | Date | Changes |
|---|---|---|
| v1.1.0 | 2026-11-11 | **Make Chart** tab: chart text → Word chart in the house layout; Read with AI from chord screenshots (files, drag-drop or clipboard) |
| v1.0.0 | 2026-11-08 | First release: PDF + DOCX, key detection, Open/Save dialogs, Bb-only-flat spelling |

---

## 9. For maintainers

- **Source:** Go, standard library only. Files: `gui_windows.go` (window/dialogs), `chartui_windows.go` (Make Chart page), `chart.go` (chart text → .docx), `ai.go` (Claude API + settings), `dib.go` (clipboard image → PNG), `transpose.go` (chords, key detection, conversion), `pdf.go` (PDF reader/writer).
- **Build:** `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui -X main.Version=v1.0.1" -o "Chord Key Changer.exe" .`
- **Release:** every push to `main` is tested, built and published automatically as `build-<n>`. A tag (`git tag v1.1.1 && git push origin v1.1.1`) also publishes a named release.
