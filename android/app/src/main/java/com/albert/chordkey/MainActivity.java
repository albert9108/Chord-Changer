package com.albert.chordkey;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.Intent;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Bundle;
import android.provider.OpenableColumns;
import android.text.InputType;
import android.text.TextUtils;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.GridLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.ArrayList;
import java.util.List;

import chordkey.Chart;
import chordkey.Chordkey;
import chordkey.Converted;
import chordkey.Images;

/**
 * Two pages, like the Windows app:
 * - Make Chart: lyrics + chord screenshots -> Read with AI -> Word chart
 * - Change Key: open a PDF/Word chart -> pick a key -> converted copy
 * All chart work is done by the shared Go engine (chordkey.aar).
 */
public class MainActivity extends Activity {

    private static final int REQ_IMAGES = 1;
    private static final int REQ_SAVE = 2;
    private static final int REQ_OPEN_CHART = 3;

    private static final String MIME_DOCX =
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
    private static final String MIME_PDF = "application/pdf";

    private static final int BLUE = 0xFF2563EB;
    private static final int TEXT = 0xFF111827;
    private static final int MUTED = 0xFF6B7280;
    private static final int SURFACE = 0xFFFFFFFF;
    private static final int BORDER = 0xFFD1D5DB;

    private SharedPreferences prefs;

    // Make Chart
    private final Images images = new Images();
    private final List<String> imageNames = new ArrayList<>();
    private EditText chartText, apiKey;
    private TextView imageInfo;
    private Button readAI, createWord, addImages, clearImages;
    private boolean busy;

    // Change Key
    private byte[] chartData;
    private String chartName, chartKey;
    private String selectedKey;
    private TextView fileInfo;
    private Button convertBtn;
    private final List<Button> keyButtons = new ArrayList<>();

    // shared
    private Button tabChart, tabKey;
    private View chartPage, keyPage;
    private TextView status;
    private byte[] pendingSave;
    private String pendingMime, pendingNotes;

    // ------------------------------------------------------------ layout helpers

    private int dp(float v) {
        return Math.round(TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v,
                getResources().getDisplayMetrics()));
    }

    private GradientDrawable box(int fill, int stroke, float radius) {
        GradientDrawable g = new GradientDrawable();
        g.setColor(fill);
        g.setCornerRadius(dp(radius));
        if (stroke != 0) g.setStroke(dp(1), stroke);
        return g;
    }

    private TextView label(String s) {
        TextView t = new TextView(this);
        t.setText(s);
        t.setTextColor(MUTED);
        t.setTextSize(12);
        t.setTypeface(Typeface.DEFAULT_BOLD);
        t.setLetterSpacing(0.08f);
        t.setPadding(0, dp(16), 0, dp(6));
        return t;
    }

    private Button button(String s, boolean primary) {
        Button b = new Button(this);
        b.setText(s);
        b.setAllCaps(false);
        b.setTextSize(15);
        b.setTypeface(Typeface.DEFAULT_BOLD);
        b.setStateListAnimator(null);
        b.setMinHeight(dp(48));
        styleButton(b, primary);
        return b;
    }

    private void styleButton(Button b, boolean primary) {
        b.setBackground(primary ? box(BLUE, 0, 10) : box(SURFACE, BORDER, 10));
        b.setTextColor(primary ? 0xFFFFFFFF : TEXT);
    }

    private EditText field(String hint) {
        EditText e = new EditText(this);
        e.setHint(hint);
        e.setTextColor(TEXT);
        e.setHintTextColor(0xFF9CA3AF);
        e.setTextSize(15);
        e.setBackground(box(SURFACE, BORDER, 10));
        e.setPadding(dp(12), dp(10), dp(12), dp(10));
        return e;
    }

    private LinearLayout.LayoutParams lp(int w, int h, float weight) {
        LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(w, h, weight);
        return p;
    }

    private LinearLayout row(View... views) {
        LinearLayout r = new LinearLayout(this);
        r.setOrientation(LinearLayout.HORIZONTAL);
        for (int i = 0; i < views.length; i++) {
            LinearLayout.LayoutParams p = lp(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1);
            if (i > 0) p.leftMargin = dp(8);
            r.addView(views[i], p);
        }
        return r;
    }

    private LinearLayout.LayoutParams topGap(int gapDp) {
        LinearLayout.LayoutParams p = lp(ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT, 0);
        p.topMargin = dp(gapDp);
        return p;
    }

    // ------------------------------------------------------------ build UI

    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        prefs = getSharedPreferences("settings", MODE_PRIVATE);

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(dp(16), dp(12), dp(16), dp(8));

        TextView title = new TextView(this);
        title.setText("Chord Key Changer");
        title.setTextColor(TEXT);
        title.setTextSize(22);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        root.addView(title);

        tabChart = button("Make Chart", true);
        tabKey = button("Change Key", false);
        tabChart.setOnClickListener(v -> showPage(true));
        tabKey.setOnClickListener(v -> showPage(false));
        root.addView(row(tabChart, tabKey), topGap(10));

        FrameLayout pages = new FrameLayout(this);
        chartPage = buildChartPage();
        keyPage = buildKeyPage();
        pages.addView(chartPage);
        pages.addView(keyPage);
        root.addView(pages, lp(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1));

        status = new TextView(this);
        status.setTextColor(MUTED);
        status.setTextSize(13);
        status.setPadding(0, dp(6), 0, 0);
        root.addView(status);

        setContentView(root);
        showPage(true);
        handleIncoming(getIntent());
    }

    private View buildChartPage() {
        LinearLayout p = new LinearLayout(this);
        p.setOrientation(LinearLayout.VERTICAL);

        p.addView(label("CHART TEXT  —  paste lyrics, or edit  [C]我来到[D]神面前"));
        chartText = field("Paste the lyrics here");
        chartText.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_FLAG_MULTI_LINE
                | InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS);
        chartText.setGravity(Gravity.TOP | Gravity.START);
        chartText.setMinLines(10);
        chartText.setHorizontallyScrolling(false);
        chartText.setText(prefs.getString("draft", ""));
        p.addView(chartText);

        p.addView(label("CHORD SCREENSHOTS"));
        imageInfo = new TextView(this);
        imageInfo.setTextColor(MUTED);
        imageInfo.setTextSize(13);
        p.addView(imageInfo);
        addImages = button("Add screenshots", false);
        clearImages = button("Clear", false);
        addImages.setOnClickListener(v -> pickImages());
        clearImages.setOnClickListener(v -> {
            images.clear();
            imageNames.clear();
            refresh();
            setStatus("Screenshots cleared.");
        });
        p.addView(row(addImages, clearImages), topGap(8));

        p.addView(label("CLAUDE API KEY"));
        apiKey = field("sk-ant-…  (console.anthropic.com → API Keys)");
        apiKey.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        apiKey.setSingleLine(true);
        apiKey.setText(prefs.getString("api_key", ""));
        p.addView(apiKey);

        readAI = button("Read with AI", true);
        readAI.setOnClickListener(v -> onReadAI());
        createWord = button("Create Word", true);
        createWord.setOnClickListener(v -> onCreateWord());
        return withBottomBar(p, row(readAI, createWord));
    }

    /** Scrolling content with the action buttons fixed underneath, always in reach. */
    private View withBottomBar(View content, View bar) {
        ScrollView s = new ScrollView(this);
        s.setFillViewport(true);
        s.addView(content);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.addView(s, lp(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1));
        page.addView(bar, topGap(10));
        return page;
    }

    private View buildKeyPage() {
        LinearLayout p = new LinearLayout(this);
        p.setOrientation(LinearLayout.VERTICAL);

        p.addView(label("CHART"));
        Button choose = button("Choose chart (PDF / Word)", false);
        choose.setOnClickListener(v -> pickChart());
        p.addView(choose);
        fileInfo = new TextView(this);
        fileInfo.setTextColor(TEXT);
        fileInfo.setTextSize(15);
        fileInfo.setPadding(0, dp(10), 0, 0);
        fileInfo.setText("No chart chosen yet. You can also share a PDF/Word chart to this app.");
        p.addView(fileInfo);

        p.addView(label("CHANGE TO"));
        GridLayout grid = new GridLayout(this);
        grid.setColumnCount(4);
        for (String k : Chordkey.keyNames().split(" ")) {
            Button b = button(k, false);
            b.setOnClickListener(v -> {
                selectedKey = k;
                refresh();
            });
            GridLayout.LayoutParams gp = new GridLayout.LayoutParams(
                    GridLayout.spec(GridLayout.UNDEFINED, 1f), GridLayout.spec(GridLayout.UNDEFINED, 1f));
            gp.width = 0;
            gp.setMargins(dp(3), dp(3), dp(3), dp(3));
            grid.addView(b, gp);
            keyButtons.add(b);
        }
        p.addView(grid);

        convertBtn = button("Convert", true);
        convertBtn.setOnClickListener(v -> onConvert());
        return withBottomBar(p, convertBtn);
    }

    private void showPage(boolean chart) {
        chartPage.setVisibility(chart ? View.VISIBLE : View.GONE);
        keyPage.setVisibility(chart ? View.GONE : View.VISIBLE);
        styleButton(tabChart, chart);
        styleButton(tabKey, !chart);
        if (chart) {
            setStatus("Paste lyrics, add chord screenshots, then Read with AI.");
        } else {
            setStatus(chartData == null ? "Choose a chart, then pick the new key."
                    : "Pick the new key, then Convert.");
        }
        refresh();
    }

    private void refresh() {
        int n = imageNames.size();
        imageInfo.setText(n == 0 ? "None yet — add them here, or share screenshots to this app."
                : n + " added: " + TextUtils.join(", ", imageNames));
        readAI.setText(busy ? "Reading…" : "Read with AI");
        setEnabled(readAI, !busy && n > 0, true);
        setEnabled(createWord, !busy, true);
        setEnabled(addImages, !busy, false);
        setEnabled(clearImages, !busy && n > 0, false);

        for (Button b : keyButtons) {
            String k = b.getText().toString();
            boolean on = chartKey != null;
            b.setEnabled(on);
            b.setAlpha(on ? 1f : 0.45f);
            if (k.equals(selectedKey)) {
                b.setBackground(box(BLUE, 0, 10));
                b.setTextColor(0xFFFFFFFF);
            } else if (k.equals(chartKey)) {
                b.setBackground(box(0xFFDBEAFE, BLUE, 10));
                b.setTextColor(TEXT);
            } else {
                styleButton(b, false);
            }
        }
        setEnabled(convertBtn, chartKey != null && selectedKey != null && !selectedKey.equals(chartKey), true);
    }

    private void setEnabled(Button b, boolean on, boolean primary) {
        b.setEnabled(on);
        b.setAlpha(on ? 1f : 0.45f);
    }

    private void setStatus(String s) {
        status.setText(s + "    " + Chordkey.getVersion());
    }

    private void alert(String title, String msg) {
        new AlertDialog.Builder(this).setTitle(title).setMessage(msg)
                .setPositiveButton("OK", null).show();
    }

    @Override
    protected void onPause() {
        super.onPause();
        prefs.edit().putString("draft", chartText.getText().toString()).apply();
    }

    // ------------------------------------------------------------ incoming shares

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleIncoming(intent);
    }

    @SuppressWarnings("deprecation")
    private void handleIncoming(Intent in) {
        if (in == null) return;
        String action = in.getAction(), type = in.getType();
        if (type == null) return;
        if (Intent.ACTION_SEND_MULTIPLE.equals(action) && type.startsWith("image/")) {
            ArrayList<Uri> uris = in.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
            if (uris != null) {
                showPage(true);
                addImageUris(uris);
            }
        } else if (Intent.ACTION_SEND.equals(action)) {
            if (type.startsWith("text/")) {
                String t = in.getStringExtra(Intent.EXTRA_TEXT);
                if (t != null) {
                    showPage(true);
                    String cur = chartText.getText().toString();
                    chartText.setText(cur.isEmpty() ? t : cur + "\n" + t);
                    setStatus("Lyrics added.");
                }
                return;
            }
            Uri u = in.getParcelableExtra(Intent.EXTRA_STREAM);
            if (u == null) return;
            if (type.startsWith("image/")) {
                showPage(true);
                List<Uri> one = new ArrayList<>();
                one.add(u);
                addImageUris(one);
            } else {
                showPage(false);
                loadChart(u);
            }
        }
    }

    // ------------------------------------------------------------ screenshots

    private void pickImages() {
        Intent i = new Intent(Intent.ACTION_GET_CONTENT);
        i.setType("image/*");
        i.addCategory(Intent.CATEGORY_OPENABLE);
        i.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true);
        startActivityForResult(Intent.createChooser(i, "Choose chord screenshots"), REQ_IMAGES);
    }

    private byte[] readAll(Uri u) throws Exception {
        try (InputStream in = getContentResolver().openInputStream(u)) {
            if (in == null) throw new Exception("could not open the file");
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            byte[] buf = new byte[65536];
            int n;
            while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
            return out.toByteArray();
        }
    }

    private String displayName(Uri u) {
        try (Cursor c = getContentResolver().query(u, new String[]{OpenableColumns.DISPLAY_NAME},
                null, null, null)) {
            if (c != null && c.moveToFirst()) {
                String s = c.getString(0);
                if (s != null) return s;
            }
        } catch (Exception ignored) {
        }
        String s = u.getLastPathSegment();
        return s == null ? "file" : s;
    }

    private static byte[] jpeg(Bitmap b) {
        int q = 92;
        while (true) {
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            b.compress(Bitmap.CompressFormat.JPEG, q, out);
            if (out.size() < 4_500_000 || q <= 50) return out.toByteArray();
            q -= 10;
        }
    }

    /**
     * Phone screenshots are often very tall (scrolling captures). The AI shrinks
     * big pictures, which makes small chord letters unreadable, so tall ones are
     * cut into overlapping pieces and wide ones are scaled to 1600px.
     */
    private static List<byte[]> prepareImage(byte[] raw) throws Exception {
        Bitmap bmp = BitmapFactory.decodeByteArray(raw, 0, raw.length);
        if (bmp == null) throw new Exception("not a picture");
        if (bmp.getWidth() > 1600) {
            int h = Math.round(bmp.getHeight() * 1600f / bmp.getWidth());
            Bitmap s = Bitmap.createScaledBitmap(bmp, 1600, h, true);
            bmp.recycle();
            bmp = s;
        }
        List<byte[]> parts = new ArrayList<>();
        int w = bmp.getWidth(), h = bmp.getHeight();
        int tile = Math.round(w * 1.5f);
        if (h <= tile * 1.3f) {
            parts.add(jpeg(bmp));
        } else {
            int overlap = Math.round(tile * 0.12f);
            for (int y = 0; y < h; y += tile - overlap) {
                int th = Math.min(tile, h - y);
                if (th < overlap * 2 && y > 0) break;
                Bitmap piece = Bitmap.createBitmap(bmp, 0, y, w, th);
                parts.add(jpeg(piece));
                piece.recycle();
                if (y + th >= h) break;
            }
        }
        bmp.recycle();
        return parts;
    }

    private void addImageUris(List<Uri> uris) {
        busy = true;
        refresh();
        setStatus("Adding screenshots…");
        new Thread(() -> {
            List<String> bad = new ArrayList<>();
            for (Uri u : uris) {
                String name = displayName(u);
                try {
                    List<byte[]> parts = prepareImage(readAll(u));
                    for (int i = 0; i < parts.size(); i++) {
                        images.add("image/jpeg", parts.get(i));
                        String n = parts.size() > 1 ? name + " (" + (i + 1) + "/" + parts.size() + ")" : name;
                        runOnUiThread(() -> imageNames.add(n));
                    }
                } catch (Exception e) {
                    bad.add(name + ": " + e.getMessage());
                }
            }
            runOnUiThread(() -> {
                busy = false;
                refresh();
                setStatus(imageNames.size() + " screenshot(s) ready. Tap Read with AI.");
                if (!bad.isEmpty()) alert("Some pictures were skipped", "• " + TextUtils.join("\n• ", bad));
            });
        }).start();
    }

    // ------------------------------------------------------------ AI + Word

    private void onReadAI() {
        String key = apiKey.getText().toString().trim();
        if (key.isEmpty()) {
            alert("API key needed", "Paste your Claude API key first.\n\nCreate one at console.anthropic.com → API Keys.");
            apiKey.requestFocus();
            return;
        }
        prefs.edit().putString("api_key", key).apply();
        String lyrics = chartText.getText().toString();
        busy = true;
        refresh();
        setStatus("Reading the screenshots with AI… (usually under a minute)");
        new Thread(() -> {
            try {
                String text = Chordkey.readWithAI(key, lyrics, images);
                runOnUiThread(() -> {
                    busy = false;
                    String before = chartText.getText().toString();
                    chartText.setText(text);
                    refresh();
                    setStatus("Done — check the chords, fix anything, then Create Word.");
                    if (!before.trim().isEmpty()) {
                        prefs.edit().putString("before_ai", before).apply();
                        Toast.makeText(this, "Your previous text is kept — long-press Create Word to restore it.",
                                Toast.LENGTH_LONG).show();
                    }
                });
            } catch (Exception e) {
                runOnUiThread(() -> {
                    busy = false;
                    refresh();
                    setStatus("AI reading failed.");
                    alert("Could not read the screenshots", e.getMessage());
                });
            }
        }).start();
        createWord.setOnLongClickListener(v -> {
            String old = prefs.getString("before_ai", null);
            if (old == null) return false;
            new AlertDialog.Builder(this).setTitle("Restore previous text?")
                    .setMessage("This replaces the chart text with what was there before Read with AI.")
                    .setPositiveButton("Restore", (d, w) -> chartText.setText(old))
                    .setNegativeButton("Cancel", null).show();
            return true;
        });
    }

    private void onCreateWord() {
        try {
            Chart c = Chordkey.buildChart(chartText.getText().toString());
            saveAs(c.getData(), MIME_DOCX, c.getFileName(), c.getWarnings());
        } catch (Exception e) {
            alert("Could not build the chart", e.getMessage());
        }
    }

    // ------------------------------------------------------------ change key

    private void pickChart() {
        Intent i = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        i.addCategory(Intent.CATEGORY_OPENABLE);
        i.setType("*/*");
        i.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{MIME_PDF, MIME_DOCX});
        startActivityForResult(i, REQ_OPEN_CHART);
    }

    private void loadChart(Uri u) {
        setStatus("Reading the chart…");
        new Thread(() -> {
            try {
                byte[] data = readAll(u);
                String name = displayName(u);
                String lower = name.toLowerCase();
                if (!lower.endsWith(".pdf") && !lower.endsWith(".docx")) {
                    String t = getContentResolver().getType(u);
                    name += MIME_DOCX.equals(t) ? ".docx" : ".pdf";
                }
                String key = Chordkey.detectKey(name, data);
                String n = name;
                runOnUiThread(() -> {
                    chartData = data;
                    chartName = n;
                    chartKey = key;
                    selectedKey = null;
                    fileInfo.setText(n + "\nCurrent key: " + key);
                    refresh();
                    setStatus("Pick the new key, then Convert.");
                });
            } catch (Exception e) {
                runOnUiThread(() -> {
                    setStatus("Could not read the chart.");
                    alert("Could not read the chart", e.getMessage());
                });
            }
        }).start();
    }

    private void onConvert() {
        if (chartData == null || selectedKey == null) return;
        setStatus("Converting…");
        String to = selectedKey;
        new Thread(() -> {
            try {
                Converted c = Chordkey.convert(chartName, chartData, chartKey, to);
                String mime = chartName.toLowerCase().endsWith(".docx") ? MIME_DOCX : MIME_PDF;
                runOnUiThread(() -> {
                    setStatus("Converted " + c.getChanged() + " chord(s).");
                    saveAs(c.getData(), mime, c.getFileName(), c.getWarnings());
                });
            } catch (Exception e) {
                runOnUiThread(() -> {
                    setStatus("Conversion failed.");
                    alert("Conversion failed", e.getMessage());
                });
            }
        }).start();
    }

    // ------------------------------------------------------------ save / open / share

    private void saveAs(byte[] data, String mime, String name, String notes) {
        pendingSave = data;
        pendingMime = mime;
        pendingNotes = notes;
        Intent i = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        i.addCategory(Intent.CATEGORY_OPENABLE);
        i.setType(mime);
        i.putExtra(Intent.EXTRA_TITLE, name);
        startActivityForResult(i, REQ_SAVE);
    }

    private void afterSave(Uri u) {
        String name = displayName(u);
        setStatus("Saved " + name);
        String msg = "Saved " + name;
        if (pendingNotes != null && !pendingNotes.isEmpty()) {
            msg += "\n\nPlease check:\n• " + pendingNotes.replace("\n", "\n• ");
        }
        String mime = pendingMime;
        new AlertDialog.Builder(this).setTitle("Done").setMessage(msg)
                .setPositiveButton("Share", (d, w) -> {
                    Intent s = new Intent(Intent.ACTION_SEND);
                    s.setType(mime);
                    s.putExtra(Intent.EXTRA_STREAM, u);
                    s.setClipData(ClipData.newRawUri(name, u));
                    s.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                    startActivity(Intent.createChooser(s, "Share chart"));
                })
                .setNeutralButton("Open", (d, w) -> {
                    Intent v = new Intent(Intent.ACTION_VIEW);
                    v.setDataAndType(u, mime);
                    v.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                    try {
                        startActivity(v);
                    } catch (ActivityNotFoundException e) {
                        alert("No app to open it", "Install Microsoft Word, WPS Office or Google Docs to open Word files.");
                    }
                })
                .setNegativeButton("Close", null).show();
        pendingSave = null;
    }

    @Override
    protected void onActivityResult(int req, int res, Intent data) {
        super.onActivityResult(req, res, data);
        if (res != RESULT_OK || data == null) return;
        if (req == REQ_IMAGES) {
            List<Uri> uris = new ArrayList<>();
            ClipData clip = data.getClipData();
            if (clip != null) {
                for (int i = 0; i < clip.getItemCount(); i++) uris.add(clip.getItemAt(i).getUri());
            } else if (data.getData() != null) {
                uris.add(data.getData());
            }
            if (!uris.isEmpty()) addImageUris(uris);
        } else if (req == REQ_OPEN_CHART && data.getData() != null) {
            loadChart(data.getData());
        } else if (req == REQ_SAVE && data.getData() != null && pendingSave != null) {
            Uri u = data.getData();
            try (OutputStream out = getContentResolver().openOutputStream(u, "wt")) {
                if (out == null) throw new Exception("could not open the file for writing");
                out.write(pendingSave);
                afterSave(u);
            } catch (Exception e) {
                alert("Could not save", e.getMessage());
            }
        }
    }
}
