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

tap_text() { # tap the first view whose text starts with $1
  adb shell uiautomator dump /sdcard/ui.xml >/dev/null
  adb pull /sdcard/ui.xml /tmp/ui.xml >/dev/null
  python3 - "$1" <<'PY'
import re, sys, subprocess
xml = open('/tmp/ui.xml', encoding='utf-8').read()
for m in re.finditer(r'text="([^"]*)"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', xml):
    if m.group(1).startswith(sys.argv[1]):
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
adb shell "am start -W -a android.intent.action.SEND -t text/plain --es android.intent.extra.TEXT 'title: Test\nkey: C\nintro:\n## Verse\n[C]la la [G]la' -n $PKG/.MainActivity"
sleep 3
shot shared
adb shell input keyevent KEYCODE_BACK   # hide keyboard if shown
sleep 1
tap_text "Create Word"
sleep 5
shot save_dialog
adb shell input keyevent KEYCODE_BACK
sleep 2

tap_text "Change Key"
sleep 2
shot change_key

echo "===LOGCAT==="
adb logcat -d | grep -E "FATAL|AndroidRuntime|chordkey|GoLog" | head -60
if adb logcat -d | grep -q "FATAL EXCEPTION"; then echo "APP CRASHED"; exit 1; fi
echo "SMOKE OK"
