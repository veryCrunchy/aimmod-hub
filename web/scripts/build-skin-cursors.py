from pathlib import Path
from PIL import Image,ImageDraw,ImageFilter
import zipfile,io
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
colours={'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248)}
for theme,c in colours.items():
    with zipfile.ZipFile(root/theme/'base.zip') as z:base={n:z.read(n) for n in z.namelist()}
    for style in ['ring','dot','crosshair','diamond','yellow','yellow-glow','white']:
        files={n:base[n] for n in ['cursortrail.png','cursortrail@2x.png','cursormiddle.png','cursormiddle@2x.png']}
        if style=='ring':
            for n in ['cursor.png','cursor@2x.png']:files[n]=base[n]
        else:
            im=Image.new('RGBA',(512,512));d=ImageDraw.Draw(im)
            if style in ['yellow','yellow-glow']:
                # Original artwork based on the white-core/rim and soft-core reading styles.
                glow=Image.new('RGBA',im.size);gd=ImageDraw.Draw(glow)
                radius=150 if style=='yellow' else 126
                gd.ellipse((256-radius,256-radius,256+radius,256+radius),fill=(250,255,0,165))
                im.alpha_composite(glow.filter(ImageFilter.GaussianBlur(27 if style=='yellow' else 42)))
                d=ImageDraw.Draw(im)
                if style=='yellow':
                    d.ellipse((106,106,406,406),fill=(241,255,0,255))
                    d.ellipse((145,145,367,367),fill=(255,255,255,255))
                else:
                    core=Image.new('RGBA',im.size);cd=ImageDraw.Draw(core)
                    cd.ellipse((135,135,377,377),fill=(255,255,15,255))
                    im.alpha_composite(core.filter(ImageFilter.GaussianBlur(8)))
            elif style=='white':
                d.ellipse((75,75,437,437),fill=(8,12,9,255))
                d.ellipse((102,102,410,410),fill=(248,255,252,255))
                d.ellipse((150,135,245,230),fill=(255,255,255,255))
            elif style=='dot':
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
        if style in ['yellow','yellow-glow','white']:
            for n in ['cursortrail.png','cursortrail@2x.png','cursormiddle.png','cursormiddle@2x.png']:
                original=Image.open(io.BytesIO(files[n])).convert('RGBA');tint=Image.new('RGBA',original.size,(241,255,0) if style in ['yellow','yellow-glow'] else (248,255,252));tint.putalpha(original.getchannel('A'));buf=io.BytesIO();tint.save(buf,format='PNG');files[n]=buf.getvalue()
        (root/theme/('cursor-'+style+'@2x.png')).write_bytes(files['cursor@2x.png'])
        for scale in [.75,1.25,1.5]:
            with zipfile.ZipFile(root/theme/f'cursor-{style}-{scale}.zip','w',zipfile.ZIP_DEFLATED) as z:
                for n,data in files.items():
                    original=Image.open(io.BytesIO(data));resized=original.resize(tuple(max(1,round(v*scale)) for v in original.size),Image.Resampling.LANCZOS);buf=io.BytesIO();resized.save(buf,format='PNG');z.writestr(n,buf.getvalue())
        with zipfile.ZipFile(root/theme/('cursor-'+style+'.zip'),'w',zipfile.ZIP_DEFLATED) as z:
            for n,data in files.items():z.writestr(n,data)
print('Prepared seven cursor choices with four sizes in each theme.')
