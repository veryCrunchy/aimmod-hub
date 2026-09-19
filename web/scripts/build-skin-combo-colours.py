"""Render neutral AimMod circle geometry for osu!'s native combo-colour tint.

The bevel belongs in hitcircle, not the untinted overlay. Keep the established
480/512 radius and 56px bevel at 4x supersampling, with a translucent interior.
Run with Python, Pillow and numpy. No source skin or private assets are needed.
"""
from pathlib import Path
from io import BytesIO
import zipfile
import numpy as np
from PIL import Image

root = Path(__file__).resolve().parents[1] / 'public/skin-builder/v1'
output = root / 'combo-colours'
output.mkdir(exist_ok=True)
y, x = np.mgrid[0:1024, 0:1024]
dx, dy = x - 512, y - 512
r = np.hypot(dx, dy)
bevel = np.sin(np.clip((480 - r) / 56, 0, 1) * np.pi)
light = np.maximum(0, (-dx - dy) / (np.maximum(r, 1) * np.sqrt(2)))
glow = np.exp(-(((dx + 90) / 310) ** 2 + ((dy + 150) / 310) ** 2) * 1.5)
rgba = np.zeros((1024, 1024, 4), dtype=np.uint8)
shade = np.where(r >= 424, 255 * (.72 + .22 * bevel + .06 * light * bevel), 12 + 18 * glow)
rgba[:, :, :3] = np.clip(shade[:, :, None], 0, 255).astype('uint8')
rgba[:, :, 3] = (np.clip(480 - r, 0, 1) * np.where(r >= 424, 255, 195)).astype('uint8')
circle = Image.fromarray(rgba)
files = {}
for suffix, size in [('.png', 128), ('@2x.png', 256)]:
    for stem in ['hitcircle', 'sliderstartcircle']:
        im = circle.resize((size, size), Image.Resampling.LANCZOS)
        buffer = BytesIO()
        im.save(buffer, format='PNG')
        files[stem + suffix] = buffer.getvalue()
        buffer = BytesIO()
        Image.new('RGBA', (1, 1)).save(buffer, format='PNG')
        files[stem + 'overlay' + suffix] = buffer.getvalue()
for name, data in files.items():
    (output / name).write_bytes(data)
with zipfile.ZipFile(root / 'combo-colours.zip', 'w', zipfile.ZIP_DEFLATED) as archive:
    for name, data in sorted(files.items()):
        entry = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
        entry.compress_type = zipfile.ZIP_DEFLATED
        archive.writestr(entry, data)
print('Generated tintable circles at 1x and 2x.')
