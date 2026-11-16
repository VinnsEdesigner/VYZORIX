#!/usr/bin/env python3
"""Render brand-mark directions where the detail is integrated, not floating.

A plain letter in a rounded square is the most saturated icon in software, and a
detail that floats below the form reads as a stray pixel at 16px. Every candidate
here integrates its detail into the silhouette so the whole shape is distinctive.

Run from the repo root:  python3 tooling/brand/directions.py
Writes /tmp/brand-preview/directions.png
"""
import io
import os

import cairosvg
from PIL import Image, ImageDraw, ImageFont

OUT = '/tmp/brand-preview/directions.png'
SIZES = [16, 24, 32, 48]
DARK_BG = (13, 13, 13)
FG = (229, 229, 229)
MUTED = (150, 150, 150)
ROSE_L, ROSE_D = '#fb7185', '#be123c'


def font(size):
    p = '/workspace/mega-webclient-ref/fonts/Inter-Regular.ttf'
    return ImageFont.truetype(p, size) if os.path.exists(p) else ImageFont.load_default()


def wrap(body, radius=6.0, fill='url(#g)'):
    return (f'  <rect x="1" y="1" width="22" height="22" rx="{radius}" fill="{fill}"/>\n{body}')


def svg(body):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="{ROSE_L}"/><stop offset="1" stop-color="{ROSE_D}"/>
  </linearGradient></defs>
{body}
</svg>'''


# --- Candidate directions -------------------------------------------------
# Each returns SVG body markup for a 24x24 tile.

DIRECTIONS = {
    '1 · plain V (current)':
        wrap('  <path d="M6.6 7.6 L12 13.8 L17.4 7.6" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <circle cx="12" cy="18.2" r="1.8" fill="#fff" opacity="0.95"/>'),

    '2 · signal trace (V is the waveform)':
        # The V is a continuous telemetry trace that dips through a node at the
        # vertex, so the line and the node are one stroke, not two objects.
        wrap('  <path d="M6.2 8.4 L9.4 12.2 L12 16.6 L14.6 12.2 L17.8 8.4" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <circle cx="12" cy="16.6" r="2.1" fill="#fff"/>'),

    '3 · doubled chevron (fleet echo)':
        wrap('  <path d="M6.4 7.2 L12 12.6 L17.6 7.2" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <path d="M8.4 14.2 L12 17.6 L15.6 14.2" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" opacity="0.75"/>'),

    '4 · notched V (single cut)':
        # A single diagonal cut through the V, so the silhouette has a signature.
        wrap('  <path d="M6.4 7.2 L12 13.6 L17.6 7.2" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <rect x="10.9" y="9.6" width="2.2" height="9.2" fill="url(#g)" transform="rotate(0 12 14)"/>'),

    '5 · V with node on the arm':
        # The node sits ON the right arm, so nothing floats.
        wrap('  <path d="M6.4 7.2 L12 13.6 L17.6 7.2" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <circle cx="15.6" cy="9.6" r="2.3" fill="#fff"/>'),

    '6 · angular V (faceted)':
        # Flat, angular strokes: a dev-tool, terminal feel.
        wrap('  <path d="M6.2 7.4 L12 14.4 L17.8 7.4" fill="none" stroke="#fff" stroke-width="2.6" stroke-linejoin="miter"/>\n'
             '  <path d="M9.4 18.6 H14.6" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="square" opacity="0.85"/>'),

    '7 · V as signal bars':
        # The V is implied by three descending bars, like a signal strength icon.
        wrap('  <path d="M7.4 7.6 L11 11.4" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/>\n'
             '  <path d="M16.6 7.6 L13 11.4" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/>\n'
             '  <path d="M12 12.4 L12 18.2" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/>'),

    '8 · terminal caret V':
        # A caret/chevron like a command prompt, with a block cursor: unmistakably
        # a developer tool.
        wrap('  <path d="M7.2 8.2 L11.6 12.6 L7.2 17" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>\n'
             '  <path d="M13.4 17 H17.6" fill="none" stroke="#fff" stroke-width="2.6" stroke-linecap="round"/>'),
}


def raster(body, size):
    png = cairosvg.svg2png(bytestring=svg(body).encode(), output_width=size)
    return Image.open(io.BytesIO(png)).convert('RGBA')


def ink_at(body, size):
    """White pixels that survive at `size` — a proxy for whether detail reads."""
    img = raster(body, size)
    return sum(1 for y in range(size) for x in range(size)
               if (lambda p: p[3] > 60 and p[0] > 225 and p[1] > 225)(img.getpixel((x, y))))


def main():
    f_label, f_head, f_small = font(14), font(16), font(12)
    pad, name_w, cell, row_h = 28, 300, 96, 108
    cols = len(SIZES)
    width = pad * 2 + name_w + cell * cols
    height = 70 + row_h * len(DIRECTIONS) + pad

    sheet = Image.new('RGB', (width, height), DARK_BG)
    d = ImageDraw.Draw(sheet)
    d.text((pad, 20), 'Vyzorix mark — directions with integrated detail', font=f_head, fill=FG)
    d.text((pad, 42), 'detail must survive 16px; anything floating reads as a stray pixel', font=f_small, fill=MUTED)

    for i, s in enumerate(SIZES):
        d.text((pad + name_w + cell * i + cell // 2 - 12, 52), f'{s}px', font=f_small, fill=MUTED)

    y = 70
    for name, body in DIRECTIONS.items():
        d.text((pad, y + row_h // 2 - 8), name, font=f_label, fill=FG)
        d.text((pad, y + row_h // 2 + 10), f'ink@16={ink_at(body, 16)}px', font=f_small, fill=MUTED)
        for i, s in enumerate(SIZES):
            img = raster(body, s)
            x = pad + name_w + cell * i + cell // 2
            cy = y + row_h // 2
            sheet.paste(img, (x - img.width // 2, cy - img.height // 2), img)
        d.line([(pad, y + row_h), (width - pad, y + row_h)], fill=(40, 40, 40))
        y += row_h

    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    sheet.save(OUT)
    print('wrote', OUT, sheet.size)


if __name__ == '__main__':
    main()
