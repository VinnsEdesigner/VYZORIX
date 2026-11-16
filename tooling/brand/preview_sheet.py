#!/usr/bin/env python3
"""Render a contact sheet of the brand assets, for visual review.

Rasterises each SVG at the sizes the app actually uses, on both a dark and a
light surface, and tiles them into one image. This is the "look at what you
made" step: a mark that reads at 128px can be mush at 16px, and the only way to
know is to render it.

Run from the repo root:  python3 tooling/brand/preview_sheet.py
Writes /tmp/brand-preview/contact-sheet.png
"""
import io
import os

import cairosvg
from PIL import Image, ImageDraw, ImageFont

BRAND = os.path.join('apps', 'VyzoriX_web', 'public', 'brand')
OUT = '/tmp/brand-preview/contact-sheet.png'

DARK_BG = (13, 13, 13)
LIGHT_BG = (250, 250, 250)
DARK_FG = (229, 229, 229)
LIGHT_FG = (23, 23, 23)

MARK_SIZES = [16, 24, 32, 48, 64, 128]
WORD_HEIGHTS = [20, 28, 40]


def load_font(size: int):
    for path in ('/workspace/mega-webclient-ref/fonts/Inter-Regular.ttf',):
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def raster(name: str, width: int) -> Image.Image:
    png = cairosvg.svg2png(url=os.path.join(BRAND, name), output_width=width)
    return Image.open(io.BytesIO(png)).convert('RGBA')


def paste_center(canvas: Image.Image, img: Image.Image, cx: int, cy: int) -> None:
    canvas.paste(img, (cx - img.width // 2, cy - img.height // 2), img)


def main() -> None:
    label = load_font(13)
    title = load_font(15)

    col_w = 150
    row_h = 150
    pad = 24
    header = 56

    cols = len(MARK_SIZES)
    width = pad * 2 + col_w * cols
    height = header + pad + row_h * 4 + pad

    sheet = Image.new('RGB', (width, height), DARK_BG)
    d = ImageDraw.Draw(sheet)
    d.text((pad, 18), 'Vyzorix brand assets — contact sheet', font=title, fill=DARK_FG)
    d.text((pad, 36), 'icon at app sizes · mono · wordmark · mask — on dark and light', font=label, fill=(150, 150, 150))

    y = header + pad

    def row(label_text: str, bg, fg, draw_fn) -> None:
        nonlocal y
        d.text((pad, y - 4), label_text, font=label, fill=(150, 150, 150))
        d.rectangle([pad, y + 20, width - pad, y + row_h - 10], fill=bg)
        for i, _ in enumerate(MARK_SIZES):
            cx = pad + col_w * i + col_w // 2
            cy = y + 20 + (row_h - 30) // 2
            draw_fn(i, cx, cy)
        y += row_h

    # Row 1: icon on dark
    row('mark · dark surface', DARK_BG, DARK_FG,
        lambda i, cx, cy: paste_center(sheet, raster('vyzorix_icon.svg', MARK_SIZES[i]), cx, cy))

    # Row 2: icon on light
    row('mark · light surface', LIGHT_BG, LIGHT_FG,
        lambda i, cx, cy: paste_center(sheet, raster('vyzorix_icon.svg', MARK_SIZES[i]), cx, cy))

    # Row 3: mono, tinted (the mono tile is currentColor; simulate tints here)
    def mono_row(i, cx, cy):
        if i >= 3:
            return
        size = [40, 64, 96][i]
        img = raster('vyzorix_icon_mono.svg', size)
        # Tint: multiply the RGB by a colour while keeping alpha.
        tint = [(225, 29, 72), (56, 189, 248), (250, 250, 250)][i]
        px = img.load()
        for yy in range(img.height):
            for xx in range(img.width):
                r, g, b, a = px[xx, yy]
                px[xx, yy] = (tint[0], tint[1], tint[2], a)
        paste_center(sheet, img, cx, cy)
    row('mono mark · tinted (currentColor)', LIGHT_BG, LIGHT_FG, mono_row)

    # Row 4: wordmark, both surfaces, and the mask
    d.text((pad, y - 4), 'wordmark (dark + light) · mask icon', font=label, fill=(150, 150, 150))
    half = (width - pad * 2) // 2
    d.rectangle([pad, y + 20, pad + half - 8, y + row_h - 10], fill=DARK_BG)
    d.rectangle([pad + half + 8, y + 20, width - pad, y + row_h - 10], fill=LIGHT_BG)
    wm = raster('vyzorix_wordmark_dark.svg', 300)
    paste_center(sheet, wm, pad + half // 2, y + 20 + (row_h - 30) // 2)
    wm2 = raster('vyzorix_wordmark_light.svg', 300)
    paste_center(sheet, wm2, pad + half + 8 + half // 2, y + 20 + (row_h - 30) // 2)

    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    sheet.save(OUT)
    print('wrote', OUT, sheet.size)


if __name__ == '__main__':
    main()
