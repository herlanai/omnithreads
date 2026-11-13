#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-/tmp/omnithreads-go-cache}"
target_arch="${1:-arm64}"
case "$target_arch" in arm64|amd64) ;; *) echo "Unsupported architecture: $target_arch" >&2; exit 1 ;; esac
if [ "$(go env GOHOSTOS)" != "darwin" ]; then
  echo 'This packager requires macOS.' >&2
  exit 1
fi
npm run build:web
CGO_ENABLED=0 go run ./scripts/appicon -source "$PWD/assets/app-icon.svg" -out "$PWD/assets/AppIcon.icns" -preview "$PWD/assets/app-icon.png"
app_dir='build/OmniThreads.app'
if [ "$target_arch" = amd64 ]; then app_dir='build/intel/OmniThreads.app'; fi
mkdir -p "$app_dir/Contents/MacOS" "$app_dir/Contents/Resources/licenses"
CGO_ENABLED=0 GOOS=darwin GOARCH="$target_arch" go build -trimpath -o "$app_dir/Contents/MacOS/omnithreads" ./cmd/omnithreads
cp assets/AppIcon.icns "$app_dir/Contents/Resources/OmniThreadsIcon.icns"
cp THIRD_PARTY_NOTICES.md "$app_dir/Contents/Resources/licenses/"
node scripts/licenses.mjs "$app_dir/Contents/Resources/licenses"
cat > "$app_dir/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>ai.omnithreads.desktop</string>
<key>CFBundleName</key><string>OmniThreads</string>
<key>CFBundleDisplayName</key><string>OmniThreads</string>
<key>CFBundleExecutable</key><string>omnithreads</string>
<key>CFBundleIconFile</key><string>OmniThreadsIcon</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.1.0</string>
<key>CFBundleVersion</key><string>2</string>
<key>NSHighResolutionCapable</key><true/>
<key>LSMinimumSystemVersion</key><string>14.0</string>
</dict></plist>
PLIST
codesign --force --deep --sign - "$app_dir"
dmg_python='.runtime/dmg-tools/bin/python'
if [ ! -x "$dmg_python" ]; then
  python3 -m venv .runtime/dmg-tools
fi
"$dmg_python" -m pip install --disable-pip-version-check -q -r scripts/dmg-requirements.txt
"$dmg_python" scripts/build-dmg.py --arch "$target_arch"
printf 'Built: %s\n' "$PWD/$app_dir" "$PWD/build/OmniThreads-darwin-$target_arch.dmg"
