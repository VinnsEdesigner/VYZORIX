#!/usr/bin/env python3
"""Render design directions for the Vyzorix mark, side by side, for review.

A mark is judged at the size it will actually be seen: a browser tab. This
renders each candidate at 16, 24, 32 and 48px on a dark surface and tiles them,
so the direction that survives 16px is obvious rather than guessed.

Run from the repo root:  python3 tooling/brand/variants.py
Writes /tmp/brand-preview/variants.png
"""
import io
import os

import cairosvg
from PIL import Image, ImageDraw, ImageFont

OUT = '/tmp/brand-preview/variants.png'
SIZES = [16, 24, 32, 48]
DARK_BG = (13, 13, 13)
FG = (229, 229, 229)
MUTED = (150, 150, 150)

ROSE_L = '#fb7185'
ROSE_D = '#be123c'


def font(size):
    p = '/workspace/mega-webclient-ref/fonts/Inter-Regular.ttf'
    return ImageFont.truetype(p, size) if os.path.exists(p) else ImageFont.load_default()


def tile(fill, inner):
    return (f'  <rect x="1" y="1" width="22" height="22" rx="6" fill="{fill}"/>\n{inner}')


def svg(body):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="{ROSE_L}"/><stop offset="1" stop-color="{ROSE_D}"/>
  </linearGradient></defs>
{body}
</svg>'''


def chevron(y0=8.0, y1=14.0, w=2.6, x0=6.9, x2=17.1):
    return (f'  <path d="M{x0} {y0} L12 {y1} L{x2} {y0}" fill="none" stroke="#fff" '
            f'stroke-width="{w}" stroke-linecap="round" stroke-linejoin="round"/>')


VARIANTS = {
    'A · chevron only': tile('url(#g)', chevron()),
    'B · chevron + pulse (current)': tile('url(#g)',
        chevron() + '\n  <path d="M5.6 18 H9 L10.4 15.8 L12 19.4 L13.6 16.6 L14.6 18 H18.4" '
        'fill="none" stroke="#fff" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" opacity="0.9"/>'),
    'C · chevron + node': tile('url(#g)',
        chevron() + '\n  <circle cx="12" cy="18.2" r="1.7" fill="#fff" opacity="0.95"/>'),
    'D · bold chevron': tile('url(#g)', chevron(y0=7.2, y1=14.6, w=3.2, x0=6.4, x2=17.6)),
    'E · chevron + baseline': tile('url(#g)',
        chevron(y0=7.4, y1=13.4) + '\n  <path d="M6.4 18.2 H17.6" fill="none" stroke="#fff" '
        'stroke-width="1.8" stroke-linecap="round" opacity="0.9"/>'),
    'F · V + signal dot pair': tile('url(#g)',
        chevron(y0=7.6, y1=13.6) + '\n  <circle cx="9.4" cy="17.8" r="1.4" fill="#fff"/>'
        '\n  <circle cx="12" cy="17.8" r="1.4" fill="#fff" opacity="0.7"/>'
        '\n  <circle cx="14.6" cy="17.8" r="1.4" fill="#fff" opacity="0.45"/>'),
    'G · notch chevron': tile('url(#g)',
        '  <path d="M6.9 7.6 L12 13.6 L17.1 7.6" fill="none" stroke="#fff" stroke-width="2.6" '
        'stroke-linecap="round" stroke-linejoin="round"/>\n'
        '  <path d="M9.2 17.4 L12 15.4 L14.8 17.4" fill="none" stroke="#fff" stroke-width="1.8" '
        'stroke-linecap="round" stroke-linejoin="round" opacity="0.85"/>'),
    'H · V over stacked bars': tile('url(#g)',
        chevron(y0=7.2, y1=13.2) + '\n  <rect x="7" y="16.4" width="10" height="1.5" rx="0.75" fill="#fff"/>'
        '\n  <rect x="8.8" y="18.4" width="6.4" height="1.5" rx="0.75" fill="#fff" opacity="0.75"/>'),
}


def raster(body, size):
    png = cairosvg.svg2png(bytestring=svg(body).encode(), output_width=size)
    return Image.open(io.BytesIO(png)).convert('RGBA')


def main():
    f_label = font(14)
    f_head = font(16)
    f_small = font(12)

    pad = 28
    name_w = 250
    cell = 96
    row_h = 108
    cols = len(SIZES)

    width = pad * 2 + name_w + cell * cols
    height = 70 + row_h * len(VARIANTS) + pad

    sheet = Image.new('RGB', (width, height), DARK_BG)
    d = ImageDraw.Draw(sheet)
    d.text((pad, 20), 'Vyzorix mark — design directions', font=f_head, fill=FG)
    d.text((pad, 42), 'rendered at the sizes a browser tab and top bar use', font=f_small, fill=MUTED)

    for i, s in enumerate(SIZES):
        x = pad + name_w + cell * i + cell // 2
        d.text((x - 12, 52), f'{s}px', font=f_small, fill=MUTED)

    y = 70
    for name, body in VARIANTS.items():
        d.text((pad, y + row_h // 2 - 8), name, font=f_label, fill=FG)
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
