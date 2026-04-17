#!/bin/bash
# Record a demo GIF of RuSwitch in action
set -e

OUTPUT_DIR="$(cd "$(dirname "$0")/.." && pwd)/build"
mkdir -p "$OUTPUT_DIR"
MOV_FILE="/tmp/ruswitch_demo.mov"
GIF_FILE="$OUTPUT_DIR/demo.gif"

echo "=== RuSwitch Demo Recorder ==="

# Check RuSwitch is running
if ! pgrep -f "Applications/RuSwitch" > /dev/null; then
    echo "ERROR: RuSwitch is not running!"
    exit 1
fi

# Open TextEdit
osascript -e '
tell application "TextEdit"
    activate
    make new document
    set bounds of front window to {200, 150, 800, 450}
end tell'
sleep 1

# Increase font
for i in 1 2 3 4 5 6; do
    osascript -e 'tell application "System Events" to tell process "TextEdit" to keystroke "+" using command down'
    sleep 0.1
done
sleep 0.5

# Click in document
cliclick c:500,300
sleep 0.3

# Start recording (full screen, we'll crop)
echo "Recording..."
screencapture -v -R200,150,600,300 "$MOV_FILE" &
SC_PID=$!
sleep 1.5

# Type demo
type_char() {
    cliclick "t:$1"
    sleep 0.1
}

type_word() {
    local word="$1"
    for (( i=0; i<${#word}; i++ )); do
        type_char "${word:$i:1}"
    done
}

# Demo 1: "привет мир"
echo "  Typing: ghbdtn..."
type_word "ghbdtn"
cliclick kp:space
sleep 1.5

echo "  Typing: vbh..."
type_word "vbh"
cliclick kp:space
sleep 1.5

# New line
cliclick kp:return
sleep 0.5

# Demo 2: "как дела?"
echo "  Typing: rfr ltkf?..."
type_word "rfr"
cliclick kp:space
sleep 1

type_word "ltkf"
type_char "?"
sleep 2

# Stop recording
kill $SC_PID 2>/dev/null
wait $SC_PID 2>/dev/null || true
sleep 1

if [ ! -f "$MOV_FILE" ]; then
    echo "Recording failed. Trying alternative method..."
    # Alternative: use ffmpeg directly
    ffmpeg -y -f avfoundation -framerate 30 -i "1:none" -t 1 /tmp/test_capture.mov 2>&1 | head -5
    echo "Check screen recording permissions in System Settings > Privacy > Screen Recording"
    exit 1
fi

echo "Converting to GIF..."
ffmpeg -y -i "$MOV_FILE" \
    -vf "fps=12,scale=600:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer" \
    -loop 0 \
    "$GIF_FILE" 2>/tmp/ffmpeg_gif.log

# Close TextEdit
osascript -e 'tell application "TextEdit" to close front document saving no' 2>/dev/null

SIZE=$(du -h "$GIF_FILE" | cut -f1)
echo "=== Done! ==="
echo "GIF: $GIF_FILE ($SIZE)"
