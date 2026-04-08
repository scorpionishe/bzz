#!/usr/bin/env python3
"""Generate a simple 512x512 PNG icon for RuSwitch (RS on steel-blue background)."""
import struct
import zlib
import os

OUTPUT = "/tmp/ruswitch_icon_src.png"


def png_chunk(chunk_type: bytes, data: bytes) -> bytes:
    c = chunk_type + data
    return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c) & 0xFFFFFFFF)


def make_png(size: int, bg: list, fg: list, pixels: set) -> bytes:
    sig = b"\x89PNG\r\n\x1a\n"
    ihdr = png_chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 2, 0, 0, 0))
    raw = b""
    for y in range(size):
        raw += b"\x00"
        for x in range(size):
            raw += bytes(fg if (x, y) in pixels else bg)
    idat = png_chunk(b"IDAT", zlib.compress(raw, 9))
    iend = png_chunk(b"IEND", b"")
    return sig + ihdr + idat + iend


def draw_rect(px: set, x0: int, y0: int, x1: int, y1: int, size: int):
    for y in range(max(0, y0), min(size, y1)):
        for x in range(max(0, x0), min(size, x1)):
            px.add((x, y))


R_BITMAP = [
    "11110",
    "10001",
    "10001",
    "11110",
    "10100",
    "10010",
    "10001",
]

S_BITMAP = [
    "01110",
    "10001",
    "10000",
    "01110",
    "00001",
    "10001",
    "01110",
]

SIZE = 512
BG = [70, 130, 180]   # steel blue
FG = [255, 255, 255]  # white

cell = SIZE // 12
oy = SIZE // 5
px: set = set()

ox_R = SIZE // 10
for ri, row in enumerate(R_BITMAP):
    for ci, ch in enumerate(row):
        if ch == "1":
            draw_rect(px, ox_R + ci * cell, oy + ri * cell,
                      ox_R + ci * cell + cell, oy + ri * cell + cell, SIZE)

ox_S = SIZE // 2 + SIZE // 12
for ri, row in enumerate(S_BITMAP):
    for ci, ch in enumerate(row):
        if ch == "1":
            draw_rect(px, ox_S + ci * cell, oy + ri * cell,
                      ox_S + ci * cell + cell, oy + ri * cell + cell, SIZE)

png_data = make_png(SIZE, BG, FG, px)
with open(OUTPUT, "wb") as f:
    f.write(png_data)
print(f"Icon generated: {OUTPUT} ({len(png_data)} bytes)")
