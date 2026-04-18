#!/bin/bash
# Record a demo GIF of RuSwitch in action.
# Uses AppleScript for typing (respects keyboard layout) and ffmpeg for screen capture.
set -e

OUTPUT_DIR="$(cd "$(dirname "$0")/.." && pwd)/build"
mkdir -p "$OUTPUT_DIR"
MOV_FILE="/tmp/ruswitch_demo.mov"
GIF_FILE="$OUTPUT_DIR/demo.gif"

echo "=== RuSwitch Demo Recorder ==="

# Check RuSwitch
if ! pgrep -f "Applications/RuSwitch" > /dev/null; then
    echo "ERROR: RuSwitch is not running"
    exit 1
fi

# Check ffmpeg screen capture permission
echo "Checking ffmpeg devices..."
ffmpeg -f avfoundation -list_devices true -i "" 2>&1 | grep -A5 "AVFoundation video devices" | head -10

# Force English keyboard layout
osascript -e 'tell application "System Events" to tell process "TextInputMenuAgent" to return' 2>/dev/null || true

# Open TextEdit fresh
osascript <<'AS'
tell application "TextEdit"
    activate
    try
        close every document saving no
    end try
    make new document
    set bounds of front window to {200, 150, 900, 550}
end tell
AS
sleep 1.5

# Bring TextEdit to front and increase font
osascript <<'AS'
tell application "System Events"
    tell process "TextEdit"
        set frontmost to true
        delay 0.3
        -- Increase font size 7 times
        repeat 7 times
            keystroke "+" using command down
            delay 0.05
        end repeat
    end tell
end tell
AS
sleep 0.5

# Start recording BEFORE typing (screen 1, entire display, crop later)
echo "Recording screen..."
SCREEN_DEV=$(ffmpeg -f avfoundation -list_devices true -i "" 2>&1 | grep -oE '\[([0-9]+)\] Capture screen 0' | head -1 | grep -oE '[0-9]+' | head -1)
if [ -z "$SCREEN_DEV" ]; then
    echo "ERROR: Can't find screen capture device. Grant Screen Recording permission."
    exit 1
fi
echo "Using screen device: $SCREEN_DEV"

ffmpeg -y -f avfoundation -framerate 20 -capture_cursor 0 \
    -i "${SCREEN_DEV}:none" \
    -t 18 \
    -vf "crop=1400:800:400:300,scale=700:-1" \
    -c:v libx264 -pix_fmt yuv420p \
    "$MOV_FILE" &>/tmp/ffmpeg_capture.log &
FFMPEG_PID=$!
sleep 2

# Type demo via AppleScript (respects layout — we assume EN is current)
osascript <<'AS' &
tell application "System Events"
    tell process "TextEdit"
        set frontmost to true
        delay 0.3

        -- Demo 1: "ghbdtn" space → "привет "
        keystroke "ghbdtn"
        delay 0.4
        keystroke " "
        delay 1.5

        -- Demo 2: "vbh" space → "мир "
        -- Need to switch back to EN first (RuSwitch switched to RU)
        keystroke space using {control down}
        delay 0.4
        keystroke "vbh"
        delay 0.3
        keystroke " "
        delay 1.5

        -- New line
        keystroke space using {control down}
        delay 0.4
        keystroke return
        delay 0.3

        -- Demo 3: "rfr ltkf?" → "как дела?"
        keystroke "rfr"
        delay 0.3
        keystroke " "
        delay 1

        keystroke space using {control down}
        delay 0.4
        keystroke "ltkf"
        delay 0.2
        keystroke "?"
        delay 1.5

        -- Demo 4: fuzzy — "gjljk;bv" → "продолжим"
        keystroke space using {control down}
        delay 0.4
        keystroke return
        delay 0.3
        keystroke "gjljk;bv"
        delay 0.3
        keystroke " "
        delay 2
    end tell
end tell
AS
TYPE_PID=$!

# Wait for recording to finish
wait $FFMPEG_PID 2>/dev/null || true
wait $TYPE_PID 2>/dev/null || true
sleep 1

if [ ! -s "$MOV_FILE" ]; then
    echo "ERROR: Recording failed. Check /tmp/ffmpeg_capture.log"
    tail -20 /tmp/ffmpeg_capture.log
    exit 1
fi

echo "Recording size: $(du -h $MOV_FILE | cut -f1)"
echo "Converting to GIF..."

# Convert to GIF with good quality
ffmpeg -y -i "$MOV_FILE" \
    -vf "fps=12,scale=700:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer" \
    -loop 0 \
    "$GIF_FILE" 2>/tmp/ffmpeg_gif.log

# Close TextEdit
osascript -e 'tell application "TextEdit" to close every document saving no' 2>/dev/null || true

SIZE=$(du -h "$GIF_FILE" | cut -f1)
echo "=== Done ==="
echo "GIF: $GIF_FILE ($SIZE)"
echo "MOV: $MOV_FILE"
