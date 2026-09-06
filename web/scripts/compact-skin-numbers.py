"""Reduce combo and leaderboard glyphs without changing score, PP or accuracy fonts."""
from pathlib import Path
from PIL import Image
import io,zipfile,json,hashlib
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
for theme in ['flow','hddt','midnight','glacier']:
 for pack in ['base','stable']:
  p=root/theme/(pack+'.zip')
  with zipfile.ZipFile(p) as z:files={n:z.read(n) for n in z.namelist()}
  for name,data in list(files.items()):
   if not name.endswith('.png') or not name.startswith(('aimmod-combo-','scoreentry-')):continue
   im=Image.open(io.BytesIO(data));target=(90 if name.startswith('aimmod-combo-') else 38)//(1 if '@2x' in name else 2)
   if im.height>target:
    im=im.resize((max(1,round(im.width*target/im.height)),target),Image.Resampling.LANCZOS);buf=io.BytesIO();im.save(buf,format='PNG');files[name]=buf.getvalue()
  if pack=='base':
   for n in range(10): (root/theme/f'aimmod-combo-{n}@2x.png').write_bytes(files[f'aimmod-combo-{n}@2x.png'])
  with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:
   for name,data in sorted(files.items()):z.writestr(zipfile.ZipInfo(name,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
 print(theme,'compact combo and leaderboard glyphs')
p=root/'inventory.json';data=json.loads(p.read_text());data['sha256']={str(f.relative_to(root)).replace('\\','/'):hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(root.rglob('*.zip'))};p.write_text(json.dumps(data,indent=2)+'\n')
