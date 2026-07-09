#!/usr/bin/env python3
"""Generate the Bzz app icon: the purple "bzz" wordmark on a white macOS squircle.

The wordmark is set in Futura Bold — its round "b" bowl and clean geometric "z"
match the brand logo. Rendered as an SVG (crisp at every size) to a 1024x1024 PNG
via rsvg-convert; the Makefile `icon` target then builds the .icns from it.
"""
import subprocess
import sys

OUTPUT = "/tmp/bzz_icon_src.png"
SVG_PATH = "/tmp/bzz_icon.svg"

PURPLE = "#6C4CE6"
FONT_FAMILY = "Futura"          # a standard macOS font

# macOS Big Sur icon grid: artwork sits in an 824x824 squircle centered in a
# 1024x1024 canvas (≈100px transparent margin), matching the system dock/Launchpad.
CANVAS = 1024
TILE = 824
TILE_OFF = (CANVAS - TILE) / 2          # 100
CORNER = 185                            # ≈0.2237 * 824 — Big Sur continuous corner

# wordmark placement (tuned to sit optically centered in the tile)
FONT_SIZE = 300
LETTER_SPACING = -12
BASELINE_Y = 612                        # baseline; block is centered by eye
CENTER_X = CANVAS / 2


def build_svg():
    return f'''<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="{CANVAS}" height="{CANVAS}" viewBox="0 0 {CANVAS} {CANVAS}">
  <defs>
    <linearGradient id="tile" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#ffffff"/>
      <stop offset="1" stop-color="#ececf1"/>
    </linearGradient>
    <filter id="shadow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur in="SourceAlpha" stdDeviation="14"/>
      <feOffset dy="10" result="off"/>
      <feColorMatrix in="off" type="matrix"
        values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 0.22 0"/>
    </filter>
  </defs>

  <!-- soft drop shadow behind the tile -->
  <rect x="{TILE_OFF}" y="{TILE_OFF}" width="{TILE}" height="{TILE}" rx="{CORNER}" ry="{CORNER}"
        fill="#000" filter="url(#shadow)"/>
  <!-- the white squircle card -->
  <rect x="{TILE_OFF}" y="{TILE_OFF}" width="{TILE}" height="{TILE}" rx="{CORNER}" ry="{CORNER}"
        fill="url(#tile)" stroke="#e2e2e8" stroke-width="1"/>

  <text x="{CENTER_X}" y="{BASELINE_Y}" text-anchor="middle"
        font-family="{FONT_FAMILY}" font-weight="bold" font-size="{FONT_SIZE}"
        letter-spacing="{LETTER_SPACING}" fill="{PURPLE}">bzz</text>
</svg>'''


def main():
    svg = build_svg()
    with open(SVG_PATH, "w") as f:
        f.write(svg)
    try:
        subprocess.run(
            ["rsvg-convert", "-w", str(CANVAS), "-h", str(CANVAS),
             SVG_PATH, "-o", OUTPUT],
            check=True,
        )
    except FileNotFoundError:
        sys.exit("error: rsvg-convert not found (brew install librsvg)")
    print(f"Icon generated: {OUTPUT} (from {SVG_PATH})")


if __name__ == "__main__":
    main()
