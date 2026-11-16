#!/usr/bin/env bash
# Emulator smoke test for the Android app (branch builds only).
# Prints screenshots into the log between markers so they can be inspected.
set -u
APK="dist/Chord Key Changer.apk"
PKG=com.albert.chordkey

shot() {
  adb exec-out screencap -p > "/tmp/$1.png"
  python3 -c "
from PIL import Image; im=Image.open('/tmp/$1.png').convert('RGB')
im=im.resize((360, int(im.height*360/im.width))); im.save('/tmp/$1.jpg', quality=55)"
  echo "===SHOT $1 BEGIN==="; base64 -w 100 "/tmp/$1.jpg"; echo "===SHOT $1 END==="
}

tap_attr() { # tap the first node whose XML matches regex $1
  adb shell uiautomator dump /sdcard/ui.xml >/dev/null
  adb pull /sdcard/ui.xml /tmp/ui.xml >/dev/null
  python3 - "$1" <<'PY'
import re, sys, subprocess
xml = open('/tmp/ui.xml', encoding='utf-8').read()
for node in re.findall(r'<node [^>]*>', xml):
    if re.search(sys.argv[1], node):
        b = re.search(r'bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', node)
        x = (int(b.group(1)) + int(b.group(3))) // 2; y = (int(b.group(2)) + int(b.group(4))) // 2
        subprocess.run(['adb', 'shell', 'input', 'tap', str(x), str(y)]); print('tapped', sys.argv[1], x, y); break
else:
    print('NOT FOUND', sys.argv[1])
PY
}

tap_text() { # tap the first view whose text starts with $1
  adb shell uiautomator dump /sdcard/ui.xml >/dev/null
  adb pull /sdcard/ui.xml /tmp/ui.xml >/dev/null
  python3 - "$1" <<'PY'
import re, sys, subprocess
xml = open('/tmp/ui.xml', encoding='utf-8').read()
for m in re.finditer(r'text="([^"]*)"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', xml):
    if m.group(1).lower().startswith(sys.argv[1].lower()):
        x = (int(m.group(2)) + int(m.group(4))) // 2; y = (int(m.group(3)) + int(m.group(5))) // 2
        subprocess.run(['adb', 'shell', 'input', 'tap', str(x), str(y)]); print('tapped', sys.argv[1], x, y); break
else:
    print('NOT FOUND', sys.argv[1])
PY
}

adb install -r "$APK" || exit 1
adb logcat -c
adb shell am start -W -n $PKG/.MainActivity
sleep 6
shot launch

# Share lyrics text in, then build the Word chart (opens the system Save dialog).
printf 'title: 测试\nkey: C\nintro:\n## Verse\n[C]我来到[G]神面前\n[Am]你爱的那个[G]天使\n' > /tmp/t.txt
adb push /tmp/t.txt /data/local/tmp/t.txt >/dev/null
adb shell 'am start -W -a android.intent.action.SEND -t text/plain --es android.intent.extra.TEXT "$(cat /data/local/tmp/t.txt)" -n com.albert.chordkey/.MainActivity'
sleep 3
shot shared
tap_text "Create Word"
sleep 5
shot save_dialog
tap_text "Save"
sleep 4
shot saved
tap_text "Close"
sleep 1

# Change Key: open the chart just saved, pick E, convert, save.
tap_text "Change Key"
sleep 1
tap_text "Choose chart"
sleep 4
shot picker
tap_text "C Key - 测试"
sleep 4
shot loaded
tap_text "E"
sleep 1
tap_text "Convert"
sleep 4
tap_text "Save"
sleep 4
shot converted
# Read with AI with a fake key: proves screenshots load and the API is reachable
# (expects "the API key was rejected").
adb shell input keyevent KEYCODE_BACK
sleep 1
python3 -c "
from PIL import Image, ImageDraw
im=Image.new('RGB',(900,1800),'white'); d=ImageDraw.Draw(im)
for i in range(30): d.text((40,40+i*58),'C        G        Am       F', fill='black')
im.save('/tmp/chord.png')"
adb push /tmp/chord.png /sdcard/Pictures/chord.png >/dev/null
adb shell am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE -d file:///sdcard/Pictures/chord.png >/dev/null
adb shell content call --uri content://media --method scan_volume --arg external_primary >/dev/null 2>&1
sleep 3
tap_text "Make Chart"
sleep 1
tap_text "Add screenshots"
sleep 5
shot img_picker
tap_attr 'content-desc="(Photo|Image)[^"]*"'
sleep 2
tap_text "Add"
sleep 2
tap_text "Done"
sleep 6
shot img_added
adb shell input swipe 160 420 160 150 400
sleep 1
tap_attr 'class="android.widget.EditText"[^>]*pass''word="true"'
sleep 1
adb shell input text "sk-ant-fake-key-for-test"
sleep 1
tap_text "Read with AI"
sleep 25
shot ai_result

echo "===LOGCAT==="
adb logcat -d | grep -E "FATAL|AndroidRuntime|chordkey|GoLog" | head -60
if adb logcat -d | grep -q "FATAL EXCEPTION"; then echo "APP CRASHED"; exit 1; fi
echo "SMOKE OK"
