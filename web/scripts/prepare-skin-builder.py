"""Import reviewed AimMod product assets, excluding local reports and captures.

Usage: python web/scripts/prepare-skin-builder.py --source <skin-output-directory>
The source directory is deliberately external to this repository.
"""
from pathlib import Path
import argparse, hashlib, json, zipfile

parser = argparse.ArgumentParser()
parser.add_argument('--source', required=True, type=Path)
args = parser.parse_args()
output = Path(__file__).resolve().parents[1] / 'public/skin-builder/v1'
output.mkdir(parents=True, exist_ok=True)
themes = {'flow': 'Flow', 'hddt': 'HDDT', 'midnight': 'Midnight', 'glacier': 'Glacier'}
inventory = {}

def archive(path, files):
    path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as z:
        for name, data in sorted(files.items()):
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(info, data)
    inventory[str(path.relative_to(output)).replace('\\', '/')] = hashlib.sha256(path.read_bytes()).hexdigest()

preview = {'sliderb0', 'hitcircle', 'hitcircleoverlay', 'approachcircle', 'reversearrow', 'cursor',
           'scorebar-bg', 'scorebar-colour', 'aimmod-timing-track', 'aimmod-duration-face', 'inputoverlay-key'}
preview |= {'default-' + str(i) for i in range(1, 5)}
preview |= {'aimmod-score-' + str(i) for i in range(10)}
preview |= {'aimmod-score-' + c for c in ['percent', 'dot', 'pp']}

for theme, name in themes.items():
    source = args.source / f'AimMod {name} v25 Soft - lazer'
    files = {p.name: p.read_bytes() for p in source.iterdir() if p.suffix in ['.png', '.wav']}
    files['skin.ini'] = (source / 'skin.ini').read_bytes()
    files['MainHUDComponents.json'] = (source / 'MainHUDComponents.json').read_bytes()
    # Explicit blank textures prevent the game's hitcircle fallback at slider ends.
    for stem in ['sliderendcircle', 'sliderendcircleoverlay']:
        for suffix in ['.png', '@2x.png']: files[stem + suffix] = (output / 'transparent.png').read_bytes()
    archive(output / theme / 'base.zip', files)
    for stem in preview:
        (output / theme / (stem + '@2x.png')).write_bytes(files[stem + '@2x.png'])
    for guide, folder in [('arrows', source), ('subtle', args.source / f'AimMod {name} Subtle v27 Soft - lazer'),
                          ('jumps', args.source / f'AimMod {name} Jumps v26 Soft - lazer')]:
        parts = {p.name: p.read_bytes() for p in folder.glob('followpoint*.png')}
        archive(output / theme / (guide + '.zip'), parts)
        (output / theme / ('guide-' + guide + '@2x.png')).write_bytes(parts['followpoint@2x.png'])

for sound in ['Soft', 'Clicky']:
    source = args.source / f'AimMod Flow v25 {sound} - lazer'
    archive(output / (sound.lower() + '.zip'), {p.name: p.read_bytes() for p in source.glob('*.wav')})

(output / 'inventory.json').write_text(json.dumps({'revision': 1, 'sha256': inventory}, indent=2) + '\n')
print(f'Prepared {len(inventory)} product archives.')
