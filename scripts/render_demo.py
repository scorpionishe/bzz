#!/usr/bin/env python3
"""
Generate a demo GIF showing Bzz in action.
Each scene types a word in wrong layout, then instantly swaps to the correct one.
"""
import os
import sys
from PIL import Image, ImageDraw, ImageFont

# Output paths
OUT_DIR = os.path.join(os.path.dirname(__file__), "..", "docs")
OUT_PATH = os.path.join(OUT_DIR, "demo.gif")
os.makedirs(OUT_DIR, exist_ok=True)

# Canvas
W, H = 720, 360
BG = (30, 30, 34)           # dark gray
WINDOW_BG = (45, 45, 50)    # slightly lighter window
TITLEBAR = (55, 55, 62)
TEXT_FG = (235, 235, 240)
CURSOR = (100, 200, 255)
HINT = (120, 120, 130)
BADGE_GREEN = (80, 200, 120)
BADGE_RED = (240, 100, 100)

# Fonts — try common macOS fonts, fall back
def load_font(size, mono=False):
    candidates = [
        "/System/Library/Fonts/SFNSMono.ttf",
        "/System/Library/Fonts/Monaco.ttf",
        "/System/Library/Fonts/Helvetica.ttc",
        "/System/Library/Fonts/HelveticaNeue.ttc",
    ] if mono else [
        "/System/Library/Fonts/Helvetica.ttc",
        "/System/Library/Fonts/HelveticaNeue.ttc",
        "/System/Library/Fonts/SFNS.ttf",
    ]
    for c in candidates:
        if os.path.exists(c):
            try:
                return ImageFont.truetype(c, size)
            except Exception:
                continue
    return ImageFont.load_default()

FONT_TEXT = load_font(30, mono=True)
FONT_TITLE = load_font(14, mono=False)
FONT_HINT = load_font(16, mono=False)
FONT_BADGE = load_font(14, mono=False)

def make_frame(text, cursor=True, badge=None):
    """Render a single frame. `badge` is ('OK', color) to show a status pill."""
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)

    # Window chrome
    d.rectangle([(40, 40), (W - 40, H - 40)], fill=WINDOW_BG, outline=(70, 70, 75), width=1)
    d.rectangle([(40, 40), (W - 40, 70)], fill=TITLEBAR)
    # Traffic lights
    d.ellipse([(52, 50), (64, 62)], fill=(255, 95, 86))
    d.ellipse([(72, 50), (84, 62)], fill=(255, 189, 46))
    d.ellipse([(92, 50), (104, 62)], fill=(39, 201, 63))
    d.text((W // 2 - 30, 49), "TextEdit", fill=(200, 200, 210), font=FONT_TITLE)

    # Text area
    tx, ty = 70, 110
    d.text((tx, ty), text, fill=TEXT_FG, font=FONT_TEXT)

    # Caret after text
    if cursor:
        bbox = d.textbbox((tx, ty), text, font=FONT_TEXT)
        caret_x = bbox[2] + 2
        d.line([(caret_x, ty + 4), (caret_x, ty + 38)], fill=CURSOR, width=2)

    # Bottom hint
    d.text((70, H - 70), "Type on English layout — Bzz fixes it automatically", fill=HINT, font=FONT_HINT)

    # Status badge
    if badge:
        label, color = badge
        pw = 90
        ph = 26
        px = W - 40 - pw - 10
        py = 45
        d.rectangle([(px, py), (px + pw, py + ph)], fill=color)
        # Centered text
        tw = d.textlength(label, font=FONT_BADGE)
        d.text((px + (pw - tw) / 2, py + 5), label, fill=(20, 20, 20), font=FONT_BADGE)

    return img

def scene(wrong, correct, post_hold=8, wrong_hold=6, correct_hold=14):
    """
    Produce frames for: progressive typing of `wrong`, hold, swap to `correct`, hold.
    Returns list of (image, duration_ms).
    """
    frames = []

    # Typing animation — one char at a time
    accumulated = ""
    for ch in wrong:
        accumulated += ch
        frames.append((make_frame(accumulated, badge=("typing", BADGE_RED)), 90))

    # Hold on wrong
    for _ in range(wrong_hold):
        frames.append((make_frame(wrong, badge=("typing", BADGE_RED)), 50))

    # Swap to correct (instant, like Bzz does)
    frames.append((make_frame(correct, badge=("fixed!", BADGE_GREEN)), 40))

    # Hold on correct
    for _ in range(correct_hold):
        frames.append((make_frame(correct, badge=("fixed!", BADGE_GREEN)), 60))

    # Short pause between scenes
    for _ in range(post_hold):
        frames.append((make_frame(correct, badge=None), 40))

    return frames

# Build the storyboard
storyboard = []
storyboard += scene("ghbdtn ", "привет ")
storyboard += scene("rfr ltkf? ", "как дела? ")
storyboard += scene("gjljk;bv ", "продолжим ")  # fuzzy fix (typo: missing р)

# Save as GIF
images = [f[0] for f in storyboard]
durations = [f[1] for f in storyboard]

images[0].save(
    OUT_PATH,
    save_all=True,
    append_images=images[1:],
    duration=durations,
    loop=0,
    optimize=True,
)

size_kb = os.path.getsize(OUT_PATH) / 1024
print(f"Wrote {OUT_PATH} ({size_kb:.0f} KB, {len(images)} frames)")
