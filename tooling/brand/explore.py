#!/usr/bin/env python3
"""Explore brand-mark directions that are not a letterform and not rose.

Two sheets:
  concepts.png — ten marks, no letter V, drawn free-standing in one accent, at
                 the sizes the product actually uses.
  palettes.png — the strongest few concepts on tile backgrounds in eight
                 non-rose palettes, so colour and form can be judged together.

Run from the repo root:  python3 tooling/brand/explore.py
"""
import io
import os

import cairosvg
from PIL import Image, ImageDraw, ImageFont

OUT_CONCEPTS = '/tmp/brand-preview/concepts.png'
OUT_PALETTES = '/tmp/brand-preview/palettes.png'

DARK = (13, 13, 13)
LIGHT = (250, 250, 250)
FG = (229, 229, 229)
MUTED = (150, 150, 150)
ACCENT = '#818cf8'  # indigo-300: bright on dark, deliberately not rose


def font(size):
    p = '/workspace/mega-webclient-ref/fonts/Inter-Regular.ttf'
    return ImageFont.truetype(p, size) if os.path.exists(p) else ImageFont.load_default()


# ---------------------------------------------------------------------------
# The concepts. Each draws on a 24x24 grid and takes its colour as an argument,
# so the same shape can be shown on a dark surface or knocked out of a tile.
# ---------------------------------------------------------------------------

def beacon(c):
    """A transmitter: a node emitting two waves. Reads as telemetry."""
    return f'''
  <circle cx="12" cy="17.6" r="2.3" fill="{c}"/>
  <path d="M7.4 13.4 A6.5 6.5 0 0 1 16.6 13.4" fill="none" stroke="{c}" stroke-width="2" stroke-linecap="round"/>
  <path d="M4.2 10.4 A10.8 10.8 0 0 1 19.8 10.4" fill="none" stroke="{c}" stroke-width="2" stroke-linecap="round" opacity="0.65"/>'''


def aperture(c):
    """A scope reticle. Vyzorix Scope is a product name, so the sight fits."""
    return f'''
  <circle cx="12" cy="12" r="6.2" fill="none" stroke="{c}" stroke-width="2"/>
  <path d="M12 2.4 V5.6 M12 18.4 V21.6 M2.4 12 H5.6 M18.4 12 H21.6" stroke="{c}" stroke-width="2" stroke-linecap="round"/>'''


def hub(c):
    """A fleet: one hub, three satellites."""
    return f'''
  <path d="M12 12 L12 4.6 M12 12 L18.4 16 M12 12 L5.6 16" stroke="{c}" stroke-width="1.7" stroke-linecap="round"/>
  <circle cx="12" cy="4.6" r="2.2" fill="{c}"/>
  <circle cx="18.4" cy="16" r="2.2" fill="{c}"/>
  <circle cx="5.6" cy="16" r="2.2" fill="{c}"/>
  <circle cx="12" cy="12" r="2.6" fill="{c}"/>'''


def pulse(c):
    """A live signal: a core with two rings."""
    return f'''
  <circle cx="12" cy="12" r="2.6" fill="{c}"/>
  <circle cx="12" cy="12" r="6.2" fill="none" stroke="{c}" stroke-width="1.9"/>
  <circle cx="12" cy="12" r="9.8" fill="none" stroke="{c}" stroke-width="1.4" opacity="0.55"/>'''


def hexnode(c):
    """A device: a hex chip with a core."""
    return f'''
  <polygon points="12,2.4 20.3,7.2 20.3,16.8 12,21.6 3.7,16.8 3.7,7.2" fill="none" stroke="{c}" stroke-width="2" stroke-linejoin="round"/>
  <circle cx="12" cy="12" r="2.7" fill="{c}"/>'''


def circuit(c):
    """A data path: a trace turning through a node."""
    return f'''
  <path d="M5.2 19 V13 A3.2 3.2 0 0 1 8.4 9.8 H18.8" fill="none" stroke="{c}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  <circle cx="5.2" cy="19" r="2.2" fill="{c}"/>
  <circle cx="18.8" cy="9.8" r="2.2" fill="{c}"/>'''


def radar(c):
    """A sweep: a sector inside a ring."""
    return f'''
  <circle cx="12" cy="12" r="9.2" fill="none" stroke="{c}" stroke-width="1.5" opacity="0.5"/>
  <path d="M12 12 L12 2.8 A9.2 9.2 0 0 1 20.2 16.6 Z" fill="{c}" opacity="0.9"/>
  <circle cx="12" cy="12" r="1.9" fill="{c}"/>'''


def equalizer(c):
    """Audio levels — the closest tie to the Scope product."""
    return f'''
  <rect x="3.3" y="13" width="3" height="8.4" rx="1.5" fill="{c}"/>
  <rect x="8.1" y="7.6" width="3" height="13.8" rx="1.5" fill="{c}"/>
  <rect x="12.9" y="10.4" width="3" height="11" rx="1.5" fill="{c}"/>
  <rect x="17.7" y="5.4" width="3" height="16" rx="1.5" fill="{c}"/>'''


def orbit(c):
    """A satellite on an orbit around a hub."""
    return f'''
  <ellipse cx="12" cy="12" rx="9.4" ry="4.2" fill="none" stroke="{c}" stroke-width="1.7" transform="rotate(-28 12 12)"/>
  <circle cx="12" cy="12" r="2.8" fill="{c}"/>
  <circle cx="19.4" cy="8.2" r="2" fill="{c}"/>'''


def waveform(c):
    """A telemetry trace: one horizontal signal line."""
    return f'''
  <path d="M2.6 14.6 H6.2 L8.4 8.2 L11.8 18.8 L14.4 11.4 L16.2 14.6 H21.4"
        fill="none" stroke="{c}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>'''


CONCEPTS = {
    'beacon': beacon,
    'aperture': aperture,
    'hub': hub,
    'pulse': pulse,
    'hex node': hexnode,
    'circuit': circuit,
    'radar': radar,
    'equalizer': equalizer,
    'orbit': orbit,
    'waveform': waveform,
}

PALETTES = {
    'indigo': ('#818cf8', '#4f46e5'),
    'violet': ('#c084fc', '#7e22ce'),
    'blue': ('#60a5fa', '#1d4ed8'),
    'cyan': ('#67e8f9', '#0891b2'),
    'teal': ('#2dd4bf', '#0f766e'),
    'emerald': ('#6ee7b7', '#047857'),
    'amber': ('#fcd34d', '#d97706'),
    'graphite': ('#94a3b8', '#334155'),
}


def svg_plain(body):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">
{body}
</svg>'''


def svg_tiled(body, light, dark):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">
  <defs><linearGradient id="t" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="{light}"/><stop offset="1" stop-color="{dark}"/>
  </linearGradient></defs>
  <rect x="1" y="1" width="22" height="22" rx="6" fill="url(#t)"/>
  <g transform="translate(3.6 3.6) scale(0.7)">{body}</g>
</svg>'''


def raster(svg, size):
    png = cairosvg.svg2png(bytestring=svg.encode(), output_width=size)
    return Image.open(io.BytesIO(png)).convert('RGBA')


def ink(svg, size):
    img = raster(svg, size)
    return sum(1 for y in range(size) for x in range(size) if img.getpixel((x, y))[3] > 60)


def sheet_concepts():
    sizes = [16, 24, 32, 48, 64]
    f_l, f_h, f_s = font(14), font(16), font(12)
    pad, name_w, cell, row_h = 28, 200, 110, 120
    width = pad * 2 + name_w + cell * len(sizes)
    height = 70 + row_h * len(CONCEPTS) + pad

    sheet = Image.new('RGB', (width, height), DARK)
    d = ImageDraw.Draw(sheet)
    d.text((pad, 20), 'Vyzorix — concepts without a letterform', font=f_h, fill=FG)
    d.text((pad, 42), 'free-standing marks, one accent, shown at product sizes', font=f_s, fill=MUTED)
    for i, s in enumerate(sizes):
        d.text((pad + name_w + cell * i + cell // 2 - 12, 52), f'{s}px', font=f_s, fill=MUTED)

    y = 70
    for name, fn in CONCEPTS.items():
        body = fn(ACCENT)
        d.text((pad, y + row_h // 2 - 8), name, font=f_l, fill=FG)
        d.text((pad, y + row_h // 2 + 10), f'ink@16={ink(svg_plain(body), 16)}', font=f_s, fill=MUTED)
        for i, s in enumerate(sizes):
            img = raster(svg_plain(body), s)
            cx = pad + name_w + cell * i + cell // 2
            cy = y + row_h // 2
            sheet.paste(img, (cx - img.width // 2, cy - img.height // 2), img)
        d.line([(pad, y + row_h), (width - pad, y + row_h)], fill=(38, 38, 38))
        y += row_h

    os.makedirs(os.path.dirname(OUT_CONCEPTS), exist_ok=True)
    sheet.save(OUT_CONCEPTS)
    print('wrote', OUT_CONCEPTS, sheet.size)


def sheet_palettes():
    picks = ['beacon', 'aperture', 'hex node', 'equalizer', 'orbit', 'hub']
    f_l, f_h, f_s = font(13), font(16), font(12)
    pad, name_w, cell, row_h = 28, 150, 110, 116
    width = pad * 2 + name_w + cell * len(PALETTES)
    height = 70 + row_h * len(picks) + pad

    sheet = Image.new('RGB', (width, height), DARK)
    d = ImageDraw.Draw(sheet)
    d.text((pad, 20), 'Concepts on tile backgrounds — eight non-rose palettes', font=f_h, fill=FG)
    d.text((pad, 42), 'the tile treatment, so colour and form are judged together', font=f_s, fill=MUTED)
    for i, name in enumerate(PALETTES):
        d.text((pad + name_w + cell * i + cell // 2 - len(name) * 3, 52), name, font=f_s, fill=MUTED)

    y = 70
    for cname in picks:
        fn = CONCEPTS[cname]
        d.text((pad, y + row_h // 2 - 8), cname, font=f_l, fill=FG)
        for i, (pname, (light, dark)) in enumerate(PALETTES.items()):
            img = raster(svg_tiled(fn('#ffffff'), light, dark), 48)
            cx = pad + name_w + cell * i + cell // 2
            cy = y + row_h // 2
            sheet.paste(img, (cx - img.width // 2, cy - img.height // 2), img)
        d.line([(pad, y + row_h), (width - pad, y + row_h)], fill=(38, 38, 38))
        y += row_h

    sheet.save(OUT_PALETTES)
    print('wrote', OUT_PALETTES, sheet.size)


if __name__ == '__main__':
    sheet_concepts()
    sheet_palettes()
