"""Build the AimMod modern spinner layers for both skin clients."""
from pathlib import Path
from PIL import Image,ImageDraw,ImageFilter
import io,json,math,zipfile,hashlib
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
glyphs=json.loads((Path(__file__).parent/'skin-glyphs.json').read_text())
logo=Image.open(root.parents[1]/'brand/aimmod-kit/mark-white.png').convert('RGBA');logo=logo.crop(logo.getbbox())
for theme,c in {'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248)}.items():
 for style in ['orbit','split','halo']:
  folder=root/theme/('spinner-'+style);folder.mkdir(exist_ok=True)
  assets={}
  def emit(name,im):
   for suffix,img in [('@2x.png',im),('.png',im.resize((im.width//2,im.height//2),Image.Resampling.LANCZOS))]:
    b=io.BytesIO();img.save(b,format='PNG');assets[name+suffix]=b.getvalue()
   (folder/(name+'@2x.png')).write_bytes(assets[name+'@2x.png'])
  def layer():return Image.new('RGBA',(768,768))
  def arc(im,r,start,end,width,colour):
   d=ImageDraw.Draw(im);d.arc((384-r,384-r,384+r,384+r),start,end,fill=colour,width=width)
   rr=r-width/2
   for angle in [start,end]:
    x=384+math.cos(math.radians(angle))*rr;y=384+math.sin(math.radians(angle))*rr
    d.ellipse((x-width/2,y-width/2,x+width/2,y+width/2),fill=colour)
  im=layer();d=ImageDraw.Draw(im)
  if style=='orbit':d.ellipse((130,130,638,638),fill=(3,12,8,175))
  elif style=='split':d.ellipse((129,129,639,639),outline=(*c,185),width=3)
  else:d.ellipse((75,75,693,693),outline=(*c,170),width=3)
  emit('spinner-bottom',im)
  im=layer()
  if style=='orbit':
   arc(im,310,140,488,18,(*c,235));arc(im,310,294,329,18,(221,255,233,255))
  elif style=='split':
   for a in [0,120,240]:arc(im,310,a+10,a+110,23,(*c,180 if a else 245))
   arc(im,310,15,45,23,(221,255,233,255))
  else:
   d=ImageDraw.Draw(im);d.ellipse((118,118,650,650),outline=(*c,235),width=17)
   arc(im,266,285,333,17,(221,255,233,255))
  emit('spinner-top',im);emit('spinner-circle',im)
  # The native fixed middle is hard-tinted red; keep it transparent.
  emit('spinner-middle',layer())
  im=layer();mark=logo.copy();mark.thumbnail((102,78),Image.Resampling.LANCZOS);tint=Image.new('RGBA',mark.size,(201,235,216));tint.putalpha(mark.getchannel('A'));im.alpha_composite(tint,((768-mark.width)//2,(768-mark.height)//2));emit('spinner-middle2',im)
  im=layer()
  if style=='halo':
   d=ImageDraw.Draw(im);d.ellipse((115,115,653,653),outline=(150,150,150,65),width=20);im=im.filter(ImageFilter.GaussianBlur(18))
  emit('spinner-glow',im)
  im=layer();d=ImageDraw.Draw(im);d.ellipse((8,8,760,760),outline=(*c,160),width=5);emit('spinner-approachcircle',im)
  for name,label in [('spinner-spin','SPIN'),('spinner-clear','CLEAR'),('spinner-warning','SPIN'),('spinner-rpm','RPM')]:
   im=Image.new('RGBA',(560,112) if name=='spinner-rpm' else (320,80));d=ImageDraw.Draw(im);scale=.8
   x=12 if name=='spinner-rpm' else (im.width-len(label)*50*scale)/2;y=10 if name=='spinner-rpm' else (im.height-64*scale)/2
   for ch in label:
    for stroke in glyphs[ch]:d.line([(x+px*scale,y+py*scale) for px,py in stroke],fill=(*c,210),width=2,joint='curve')
    x+=50*scale
   emit(name,im)
  with zipfile.ZipFile(root/theme/('spinner-'+style+'.zip'),'w',zipfile.ZIP_DEFLATED) as z:
   for n,data in sorted(assets.items()):z.writestr(zipfile.ZipInfo(n,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
  preview=Image.new('RGBA',(768,768),(3,8,5,255))
  for name in ['spinner-glow','spinner-bottom','spinner-top','spinner-middle2','spinner-middle']:preview.alpha_composite(Image.open(io.BytesIO(assets[name+'@2x.png'])))
  preview.resize((256,256),Image.Resampling.LANCZOS).save(folder/'preview.png')
  if style=='orbit':
   for pack in ['base','stable']:
    p=root/theme/(pack+'.zip')
    with zipfile.ZipFile(p) as z:files={n:z.read(n) for n in z.namelist() if not (n.startswith('spinner-') and n.endswith('.png'))}
    files.update(assets)
    with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:
     for n,data in sorted(files.items()):z.writestr(zipfile.ZipInfo(n,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
  print(theme,style,'spinner exported')
p=root/'inventory.json';data=json.loads(p.read_text());data['sha256']={str(f.relative_to(root)).replace('\\','/'):hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(root.rglob('*.zip'))};p.write_text(json.dumps(data,indent=2)+'\n')
