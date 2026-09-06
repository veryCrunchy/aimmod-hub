from pathlib import Path
from PIL import Image,ImageDraw
import zipfile,io
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
colours={'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248)}
for theme,c in colours.items():
    with zipfile.ZipFile(root/theme/'base.zip') as z:base={n:z.read(n) for n in z.namelist()}
    for style in ['ring','dot','crosshair','diamond']:
        files={n:base[n] for n in ['cursortrail.png','cursortrail@2x.png','cursormiddle.png','cursormiddle@2x.png']}
        if style=='ring':
            for n in ['cursor.png','cursor@2x.png']:files[n]=base[n]
        else:
            im=Image.new('RGBA',(512,512));d=ImageDraw.Draw(im)
            if style=='dot':
                d.ellipse((122,122,390,390),fill=(3,10,7,255));d.ellipse((148,148,364,364),fill=c);d.ellipse((189,173,244,228),fill=(248,255,251,245))
            elif style=='crosshair':
                for a,b in [((256,72),(256,192)),((256,320),(256,440)),((72,256),(192,256)),((320,256),(440,256))]:
                    d.line([a,b],fill=(3,10,7,255),width=48);d.line([a,b],fill=c,width=22)
                d.ellipse((232,232,280,280),fill=(248,255,251))
            else:
                pts=[(256,78),(434,256),(256,434),(78,256),(256,78)]
                d.line(pts,fill=(3,10,7,255),width=55,joint='curve');d.line(pts,fill=c,width=27,joint='curve');d.ellipse((237,237,275,275),fill='white')
            for suffix,size in [('@2x.png',(64,64)),('.png',(32,32))]:
                buf=io.BytesIO();im.resize(size,Image.Resampling.LANCZOS).save(buf,format='PNG');files['cursor'+suffix]=buf.getvalue()
        (root/theme/('cursor-'+style+'@2x.png')).write_bytes(files['cursor@2x.png'])
        with zipfile.ZipFile(root/theme/('cursor-'+style+'.zip'),'w',zipfile.ZIP_DEFLATED) as z:
            for n,data in files.items():z.writestr(n,data)
print('Prepared four cursor choices in each theme.')
