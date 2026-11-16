#!/usr/bin/env bash
# Builds android/app/libs/chordkey.aar from the shared Go engine.
# Needs Go, gomobile on PATH, and ANDROID_HOME / ANDROID_NDK_HOME.
# Usage: android/build-aar.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-dev}"

W="$(mktemp -d)"
mkdir -p "$W/engine" "$W/chordkey"
for f in chart.go transpose.go pdf.go ai.go dib.go; do
  sed 's/^package main$/package engine/' "$f" > "$W/engine/$f"
done
grep -v '^//go:build ignore' android/gobind/api.go > "$W/chordkey/api.go"
printf 'module chordkeymobile\n\ngo 1.22\n' > "$W/go.mod"

(
  cd "$W"
  go get golang.org/x/mobile/bind@latest
  GOFLAGS=-mod=mod gomobile bind -target=android -androidapi 24 \
    -ldflags "-s -w -X chordkeymobile/chordkey.Version=$VERSION" \
    -o chordkey.aar ./chordkey
)
mkdir -p android/app/libs
cp "$W/chordkey.aar" android/app/libs/chordkey.aar
echo "built android/app/libs/chordkey.aar ($VERSION)"
