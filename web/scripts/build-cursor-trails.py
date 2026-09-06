"""Build original cursor-trail patches with matching cursor colours and sizes."""
from pathlib import Path
from PIL import Image,ImageDraw,ImageFilter
import zipfile,io,json,hashlib
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1';out=root/'trails';out.mkdir(exist_ok=True)
for colour,rgb in {'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248),'yellow':(241,255,0),'white':(248,255,252)}.items():
 for style in ['off','soft','dots','glow']:
  im=Image.new('RGBA',(256,256));d=ImageDraw.Draw(im)
  if style=='soft':d.ellipse((106,106,150,150),fill=(*rgb,145));im=im.filter(ImageFilter.GaussianBlur(5))
  elif style=='dots':d.ellipse((98,98,158,158),fill=(*rgb,220))
  elif style=='glow':
   d.ellipse((83,83,173,173),fill=(*rgb,155));im=im.filter(ImageFilter.GaussianBlur(15));d=ImageDraw.Draw(im);d.ellipse((114,114,142,142),fill=(*rgb,230))
  im.resize((64,64),Image.Resampling.LANCZOS).save(out/f'{colour}-{style}@2x.png')
  for size in [.75,1,1.25,1.5]:
   files={}
   for suffix,dimension in [('.png',32),('@2x.png',64)]:
    b=io.BytesIO();im.resize((round(dimension*size),)*2,Image.Resampling.LANCZOS).save(b,format='PNG');files['cursortrail'+suffix]=b.getvalue()
    if style!='dots':files['cursormiddle'+suffix]=(root/'transparent.png').read_bytes()
   with zipfile.ZipFile(out/f'{colour}-{style}-{size}.zip','w',zipfile.ZIP_DEFLATED) as z:
    for name,data in files.items():z.writestr(zipfile.ZipInfo(name,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
p=root/'inventory.json';data=json.loads(p.read_text());data['sha256']={str(f.relative_to(root)).replace('\\','/'):hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(root.rglob('*.zip'))};p.write_text(json.dumps(data,indent=2)+'\n')
print('Prepared Off, Soft, Dots and Glow trails in six colours and four sizes.')
