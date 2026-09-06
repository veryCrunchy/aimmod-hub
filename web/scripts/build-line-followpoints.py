"""Create understated straight followpoints for each AimMod theme."""
from pathlib import Path
from PIL import Image,ImageDraw
import io,zipfile,json,hashlib
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
for theme,c in {'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248)}.items():
 files={};im=Image.new('RGBA',(256,64));ImageDraw.Draw(im).rectangle((0,27,255,36),fill=(*c,135))
 for suffix,size in [('.png',(32,8)),('@2x.png',(64,16))]:
  buf=io.BytesIO();im.resize(size,Image.Resampling.LANCZOS).save(buf,format='PNG');files['followpoint'+suffix]=buf.getvalue()
 with zipfile.ZipFile(root/theme/'line.zip','w',zipfile.ZIP_DEFLATED) as z:
  for name,data in files.items():z.writestr(zipfile.ZipInfo(name,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
 (root/theme/'guide-line@2x.png').write_bytes(files['followpoint@2x.png'])
p=root/'inventory.json';data=json.loads(p.read_text());data['sha256']={str(f.relative_to(root)).replace('\\','/'):hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(root.rglob('*.zip'))};p.write_text(json.dumps(data,indent=2)+'\n')
print('Created Line followpoints for all four themes.')
