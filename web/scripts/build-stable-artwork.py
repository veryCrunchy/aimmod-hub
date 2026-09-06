"""Generate original full stable artwork for four modes and four AimMod themes.

The inventory follows ppy/osu-wiki Skinning documentation. No default skin artwork
is imported. Existing approved standard gameplay textures remain intact.
"""
from pathlib import Path
from PIL import Image, ImageDraw
import json, zipfile, io, math, re

HERE=Path(__file__).resolve().parent
OUT=HERE.parent/'public/skin-builder/v1'
CATALOG=json.loads((HERE/'stable-skin-assets.json').read_text())
GLYPHS=json.loads((HERE/'skin-glyphs.json').read_text())
THEMES={'flow':(119,189,152),'hddt':(235,242,239),'midnight':(99,210,163),'glacier':(152,219,248)}
CUSTOM_SIZES={'pause-overlay':(1366,768),'fail-background':(1366,768),'menu-background':(1366,768),
'ranking-panel':(640,666),'ranking-graph':(280,110),'ranking-title':(320,68),'ranking-accuracy':(160,34),'ranking-maxcombo':(160,34),
'ranking-perfect':(230,50),'ranking-replay':(180,48),'ranking-retry':(180,48),'ranking-winner':(230,58),
'songselect-top':(1366,112),'songselect-bottom':(1366,100),'menu-back':(180,72),'menu-button-background':(700,100),
'button-left':(16,50),'button-middle':(4,50),'button-right':(16,50), 'mania-stage-left':(20,480),'mania-stage-right':(20,480),
'mania-stage-bottom':(400,80),'mania-stage-light':(50,480),'mania-stage-hint':(400,12),'lightingL':(50,160),'lightingN':(50,160),
'fruit-catcher-idle':(306,320),'fruit-catcher-fail':(306,320),'fruit-catcher-kiai':(306,320),'fruit-ryuuta':(306,320),
'play-skip':(160,64),'section-pass':(320,80),'section-fail':(320,80),'ready':(280,90),'go':(180,90),
'spinner-background':(512,512),'spinner-circle':(512,512),'spinner-top':(512,512),'spinner-bottom':(512,512),
'spinner-middle':(160,160),'spinner-middle2':(160,160),'spinner-glow':(512,512),'spinner-metre':(64,256),
'spinner-rpm':(96,32),'spinner-clear':(240,72),'spinner-spin':(240,72),'spinner-osu':(180,180),'spinner-approachcircle':(512,512)}

for theme,accent in THEMES.items():
    with zipfile.ZipFile(OUT/theme/'base.zip') as z:base={n:z.read(n) for n in z.namelist()}
    files={};rendered={}
    def emit(stem,im):
        # im is rendered at 2x; both density versions are generated together.
        rendered[stem]=im
        for suffix,img in [('@2x.png',im),('.png',im.resize((max(1,im.width//2),max(1,im.height//2)),Image.Resampling.LANCZOS))]:
            buf=io.BytesIO();img.save(buf,format='PNG');files[stem+suffix]=buf.getvalue()
    def make(w,h):return Image.new('RGBA',(max(2,w*2),max(2,h*2)))
    def text(im,label,size=24,colour=None,position=None):
        label=label.upper();units=sum(26 if c==' ' else 50 for c in label)
        scale=min(size*2/64,im.width*.88/max(1,units))
        x,y=position or (im.width/2,im.height/2);x-=units*scale/2;y-=32*scale
        d=ImageDraw.Draw(im)
        for char in label:
            for line in GLYPHS.get(char,[]):
                d.line([(x+(px+4)*scale,y+py*scale) for px,py in line],fill=colour or accent,width=max(1,round(5*scale)),joint='curve')
            x+=(26 if char==' ' else 50)*scale
    def plate(im,active=False):
        d=ImageDraw.Draw(im);w,h=im.size;m=max(2,min(w,h)//14)
        d.rounded_rectangle((m,m,w-m-1,h-m-1),radius=max(2,min(18,h//5)),fill=(*accent,55) if active else (6,16,11,240),outline=(*accent,180),width=2)
        d.line((m*2,h-m-4,w-m*2,h-m-4),fill=(*accent,100),width=2)
    def ring(im,filled=True):
        d=ImageDraw.Draw(im);w,h=im.size;m=max(3,int(min(w,h)*.06));stroke=max(3,int(min(w,h)*.055))
        if filled:d.ellipse((m,m,w-m,h-m),fill=(255,255,255,215))
        else:d.ellipse((m,m,w-m,h-m),outline=(*accent,240),width=stroke)
    def mark(im):
        # Original compact M geometry, supplied AimMod silhouette expressed as polygons.
        w,h=im.size;d=ImageDraw.Draw(im)
        pts=[(.12,.77),(.31,.22),(.45,.22),(.61,.77),(.46,.77),(.38,.46),(.27,.77)]
        d.polygon([(x*w,y*h) for x,y in pts],fill=accent)
        pts=[(.49,.22),(.68,.22),(.86,.77),(.71,.77),(.63,.46),(.58,.58)]
        d.polygon([(x*w,y*h) for x,y in pts],fill=accent)
    for filename,meta in CATALOG.items():
        stem=Path(filename).stem
        # Reuse the already approved gameplay, fonts, silent particles and judgement assets.
        if filename in base and not stem.startswith(('spinner','selection','pause','ranking','menu','songselect','mode-','button','count','ready','go','section','score-','lightingL','lightingN','play-','arrow-','star')):
            continue
        if stem.startswith('score-') and 'aimmod-'+filename in base:
            files[filename]=base['aimmod-'+filename]
            hd='aimmod-'+stem+'@2x.png'
            if hd in base:files[stem+'@2x.png']=base[hd]
            continue
        size=CUSTOM_SIZES.get(stem) or meta['size'] or (128,64)
        if stem.startswith('ranking-') and stem.split('-')[1] in ['XH','X','SH','S','A','B','C','D']:size=(34,40) if stem.endswith('small') else (200,214)
        if stem.startswith('selection-mod-'):size=(68,64)
        if stem.startswith('pause-') and stem!='pause-overlay':size=(280,68)
        if stem.startswith('mania-note'):size=(50,64 if stem.endswith('L') else 20)
        if stem.startswith('mania-hit') or stem.startswith('taiko-hit'):size=(140,56)
        if stem.startswith('pippidon'):size=(128,128)
        if stem.startswith('comboburst'):size=(180,200)
        if stem.startswith('mode-'):size=(32,32) if stem.endswith('small') else (64,64) if stem.endswith('med') else (96,96)
        w,h=size;im=make(w,h);d=ImageDraw.Draw(im);W,H=im.size
        if stem in ['menu-background','pause-overlay','fail-background']:
            d.rectangle((0,0,W,H),fill=(3,10,7,245) if stem!='menu-background' else (3,10,7,255))
            for y in range(H):
                glow=max(0,1-abs(y-H*.3)/H);d.line((0,y,W,y),fill=(round(3+accent[0]*.035*glow),round(8+accent[1]*.06*glow),round(5+accent[2]*.04*glow),245 if stem!='menu-background' else 255))
            logo=Image.open(HERE.parent/'public/brand/aimmod-kit/wordmark-white.png').convert('RGBA');logo.thumbnail((520,130));im.alpha_composite(logo,(90,70))
            if stem!='menu-background':text(im,'PAUSED' if stem=='pause-overlay' else 'KEEP GOING',48,position=(W/2,H*.26))
            if stem=='menu-background':
                buf=io.BytesIO();im.convert('RGB').resize((w,h)).save(buf,format='JPEG',quality=92);files[filename]=buf.getvalue();rendered[stem]=im;continue
        elif stem.startswith('mania-note'):
            if stem.endswith('L'):
                d.rectangle((8,0,W-8,H),fill=(*accent,100));d.rectangle((8,0,12,H),fill=accent);d.rectangle((W-12,0,W-8,H),fill=accent)
            else:
                d.rounded_rectangle((4,4,W-4,H-4),radius=5,fill=accent if '2' not in stem else (230,249,241,255));d.line((10,7,W-10,7),fill='white',width=2)
        elif stem.startswith('mania-key'):
            plate(im,stem.endswith('D'));d.rectangle((8,12,W-8,20),fill=accent if stem.endswith('D') else (*accent,130))
        elif stem.startswith('mania-hit') or stem.startswith('taiko-hit'):
            value=re.search(r'hit(\d+)',stem)[1];label={'0':'MISS','50':'50','100':'100','200':'200','300':'MAX' if stem.endswith('g') else '300'}[value]
            text(im,label,28,(255,99,127) if value=='0' else accent)
        elif stem.startswith('mania-stage') or stem in ['lightingL','lightingN']:
            if stem.endswith(('left','right')):
                d.rectangle((W//2-2,0,W//2+2,H),fill=(*accent,160))
            elif stem.endswith(('light','lightingL','lightingN')) or stem in ['lightingL','lightingN']:
                for y in range(H):d.line((0,y,W,y),fill=(*accent,round(110*y/H)))
            elif stem.endswith('hint'):d.rectangle((0,H//2-2,W,H//2+2),fill=accent)
            else:plate(im)
        elif stem.startswith('fruit-catcher') or stem=='fruit-ryuuta':
            d.polygon([(W*.13,H*.69),(W*.23,H*.9),(W*.77,H*.9),(W*.87,H*.69)],fill=(10,29,20,255),outline=accent,width=6)
            d.rounded_rectangle((W*.1,H*.63,W*.9,H*.72),radius=15,fill=(255,99,127) if 'fail' in stem else accent)
            small=make(100,100);mark(small);im.alpha_composite(small,(int(W/2-100),int(H*.72-45)))
        elif stem.startswith('fruit-'):
            fruit=stem.replace('-overlay','');overlay=stem.endswith('overlay')
            if overlay:
                d.arc((W*.18,H*.16,W*.82,H*.83),205,280,fill=(245,255,250,220),width=7)
            elif 'bananas' in fruit:
                d.arc((W*.12,H*.03,W*.84,H*.86),25,150,fill=(255,220,102),width=int(W*.23))
            elif 'grapes' in fruit:
                for cx,cy in [(.35,.35),(.58,.32),(.46,.54),(.58,.6),(.4,.74)]:d.ellipse((W*(cx-.15),H*(cy-.15),W*(cx+.15),H*(cy+.15)),fill=accent,outline=(220,250,238),width=3)
            elif 'drop' in fruit:
                d.polygon([(W*.5,H*.1),(W*.24,H*.64),(W*.3,H*.84),(W*.7,H*.84),(W*.76,H*.64)],fill=accent)
            else:
                d.ellipse((W*.2,H*.24,W*.8,H*.85),fill=accent,outline=(230,251,241),width=4)
                d.polygon([(W*.49,H*.29),(W*.51,H*.1),(W*.67,H*.14),(W*.6,H*.25)],fill=(230,249,241))
        elif stem.startswith('taiko'):
            if 'hitcircle' in stem or 'bigcircle' in stem:ring(im,not stem.endswith('overlay'))
            elif 'roll-middle' in stem:d.rectangle((0,H*.25,W,H*.75),fill=(250,221,132,230))
            elif 'roll-end' in stem:ring(im)
            elif stem=='taiko-barline':d.rectangle((0,0,W,H),fill=(*accent,100))
            elif 'drum' in stem:
                d.ellipse((W*.05,H*.18,W*.95,H*.82),fill=(8,20,14,255),outline=accent,width=8)
                if 'inner' in stem:d.ellipse((W*.2,H*.29,W*.8,H*.71),fill=(*accent,180))
            elif 'glow' in stem:ring(im,False)
            elif 'flower' in stem:mark(im)
            elif stem=='taiko-bar-left':plate(im);mark(im)
            else:
                d.rectangle((0,0,W,H),fill=(5,14,10,255));d.line((0,H*.25,W,H*.25),fill=(*accent,110),width=3);d.line((0,H*.75,W,H*.75),fill=(*accent,110),width=3)
        elif stem.startswith('pippidon') or stem.startswith('comboburst'):mark(im)
        elif stem.startswith('spinner'):
            if any(stem.endswith(s) for s in ['clear','spin','rpm','warning']):text(im,{'spinner-clear':'CLEAR','spinner-spin':'SPIN','spinner-rpm':'RPM','spinner-warning':'SPIN'}.get(stem,'SPIN'),26)
            elif stem=='spinner-metre':
                for i in range(12):d.rounded_rectangle((8,int(H*(1-(i+1)/12))+3,W-8,int(H*(1-i/12))-3),radius=4,fill=(*accent,230))
            elif stem in ['spinner-osu','spinner-middle','spinner-middle2']:mark(im)
            else:
                ring(im,False)
                for i in range(12):
                    a=i*math.pi/6;cx=W/2;cy=H/2;d.line((cx+math.cos(a)*W*.39,cy+math.sin(a)*H*.39,cx+math.cos(a)*W*.43,cy+math.sin(a)*H*.43),fill=accent,width=5)
        elif stem=='options-offset-tick':
            d.rounded_rectangle((W//2-2,3,W//2+2,H-3),radius=2,fill=accent)
        elif stem=='hitcircleselect':ring(im,False)
        elif stem in ['sliderb','sliderb-nd','sliderb-spec']:
            ring(im,False)
        elif stem in ['sliderpoint10','sliderpoint30']:text(im,stem[-2:],22)
        elif stem in ['sliderendmiss','slidertickmiss']:text(im,'X',20,(255,99,127))
        elif stem.startswith('ranking-'):
            label=stem.split('-')[1]
            if label in ['XH','X','SH','S','A','B','C','D']:
                ring(im,False);text(im,{'XH':'SS','X':'SS','SH':'S'}.get(label,label),max(15,int(w*.45)))
            elif label=='panel':
                d.rounded_rectangle((12,12,W-12,H-12),radius=24,fill=(4,13,9,220),outline=(*accent,80),width=2)
            elif label=='graph':plate(im)
            else:text(im,{'maxcombo':'MAX COMBO','accuracy':'ACCURACY','title':'RESULTS','perfect':'PERFECT','replay':'WATCH REPLAY','retry':'RETRY','winner':'WINNER'}.get(label,label.upper()),22)
        elif stem.startswith('selection-mod-'):
            code={'doubletime':'DT','hardrock':'HR','hidden':'HD','nightcore':'NC','halftime':'HT','flashlight':'FL','nofail':'NF','easy':'EZ','relax':'RX','relax2':'AP','autoplay':'AT','cinema':'CM','spunout':'SO','suddendeath':'SD','perfect':'PF','scorev2':'V2','keycoop':'CO','freemodallowed':'FM','touchdevice':'TD'}
            label=stem.replace('selection-mod-','');plate(im);text(im,code.get(label,label.replace('key','K').upper()[:3]),24)
        elif stem.startswith('mode-'):
            ring(im,False);text(im,{'osu':'O','taiko':'T','fruits':'C','mania':'M'}.get(stem.split('-')[1],'O'),max(12,int(w*.4)))
        elif stem.startswith('songselect'):
            d.rectangle((0,0,W,H),fill=(4,13,9,235));d.line((0,H-3,W,H-3),fill=accent,width=3)
        elif stem.startswith('count'):text(im,stem[-1],40)
        elif stem in ['star','star2','menu-snow','cursor-ripple','cursor-smoke']:
            if stem=='star2':pass  # Explicit quiet gameplay sparkle suppression.
            else:d.polygon([(W*.5,H*.1),(W*.6,H*.4),(W*.9,H*.5),(W*.6,H*.6),(W*.5,H*.9),(W*.4,H*.6),(W*.1,H*.5),(W*.4,H*.4)],fill=accent)
        elif stem in ['masking-border','button-middle','button-left','button-right','menu-button-background','selection-tab']:plate(im)
        elif stem.startswith(('arrow-','play-warning','mania-warning')):
            d.polygon([(W*.2,H*.2),(W*.8,H*.5),(W*.2,H*.8),(W*.36,H*.5)],fill=accent)
        else:
            label={'pause-back':'BACK','pause-continue':'CONTINUE','pause-retry':'RETRY','pause-replay':'REPLAY','menu-back':'BACK','play-skip':'SKIP','section-pass':'NICE WORK','section-fail':'KEEP GOING','play-unranked':'UNRANKED','rank-forum':'DISCUSS','welcome_text':'AIMMOD'}.get(stem,stem.replace('selection-','').replace('-over','').replace('-',' ').upper())
            if stem.startswith(('pause-','menu-','selection-')):plate(im,stem.endswith('over'))
            text(im,label,24)
        emit(stem,im)
    # Explicit centre-lane hold-head assets missing from older wiki inventories.
    for suffix in ['H','L','T']:
        for density in ['.png','@2x.png']:
            files['mania-noteS'+suffix+density]=files['mania-note1'+suffix+density]
    ini=[]
    for keys in range(1,19):
        width=min(50,600//keys);left=max(60,(640-keys*width)//2)
        ini+=['[Mania]',f'Keys: {keys}',f'ColumnStart: {left}','ColumnWidth: '+','.join([str(width)]*keys),'ColumnLineWidth: '+','.join(['0']*(keys+1)),'HitPosition: 402','LightPosition: 402','ScorePosition: 90','ComboPosition: 180','JudgementLine: 0','StageLeft: mania-stage-left','StageRight: mania-stage-right','StageBottom: mania-stage-bottom','StageHint: mania-stage-hint','StageLight: mania-stage-light','LightingN: lightingN','LightingL: lightingL']
        for col in range(keys):
            kind='S' if keys%2 and col==keys//2 else '1' if col%2==0 else '2'
            ini += [f'KeyImage{col}: mania-key{kind}',f'KeyImage{col}D: mania-key{kind}D',f'NoteImage{col}: mania-note{kind}']
            for suffix in ['H','L','T']:ini.append(f'NoteImage{col}{suffix}: mania-note{kind}{suffix}')
    files['stable.ini']=('\n'.join(ini)+'\n').encode()
    # The stable pack covers every documented static image (or the reviewed base).
    for name in CATALOG:assert name in files or name in base,name
    with zipfile.ZipFile(OUT/theme/'stable.zip','w',zipfile.ZIP_DEFLATED) as z:
        for name,data in sorted(files.items()):z.writestr(name,data)
    def sprite(name):
        data=files.get(name+'@2x.png') or base.get(name+'@2x.png');return Image.open(io.BytesIO(data)).convert('RGBA')
    for scene in ['menu','pause','results','mania','catch','taiko']:
        canvas=Image.new('RGBA',(960,600),(3,9,6,255));d=ImageDraw.Draw(canvas)
        def put(name,xy,size):canvas.alpha_composite(sprite(name).resize(size,Image.Resampling.LANCZOS),xy)
        if scene in ['menu','pause']:
            bg=rendered['menu-background' if scene=='menu' else 'pause-overlay'].resize((960,600));canvas.alpha_composite(bg)
            if scene=='pause':
                for i,name in enumerate(['pause-continue','pause-retry','pause-back']):put(name,(330,250+i*80),(300,70))
            else:
                text(canvas,'SONG SELECT',25,position=(730,130))
                for i,label in enumerate(['FIRST LIGHT','MIDNIGHT RUN','CLEAR SKIES','AFTERGLOW']):
                    tile=sprite('menu-button-background').resize((460,76));text(tile,label,18);canvas.alpha_composite(tile,(460,185+i*86))
                for i,mode in enumerate(['osu','taiko','fruits','mania']):put('mode-'+mode,(54+i*86,470),(66,66))
        elif scene=='results':
            put('ranking-panel',(30,30),(570,520));put('ranking-title',(70,56),(300,65));put('ranking-S',(650,140),(220,230))
            for i,label in enumerate(['SCORE  00428160','ACCURACY  98.72%','MAX COMBO  428']):text(canvas,label,19,position=(310,205+i*92))
            put('ranking-replay',(635,455),(230,65))
        elif scene=='mania':
            for col in range(4):
                x=320+col*80;d.rectangle((x,0,x+78,540),fill=(7,18,12,255));put('mania-key'+('1' if col%2==0 else '2'),(x,490),(78,100))
                for y in [80+col*35,240+col*25,390-col*22]:put('mania-note'+('1' if col%2==0 else '2'),(x,y),(78,25))
            put('mania-note1L',(320,130),(78,165));put('mania-note1H',(320,130),(78,25));put('mania-stage-hint',(320,481),(320,10))
        elif scene=='catch':
            for i,fruit in enumerate(['apple','grapes','pear','bananas','orange','drop']):
                put('fruit-'+fruit,(100+i*132,70+(i%3)*100),(70,70));put('fruit-'+fruit+'-overlay',(100+i*132,70+(i%3)*100),(70,70))
            put('fruit-catcher-idle',(360,370),(210,220))
        else:
            put('taiko-bar-right',(140,230),(820,165));put('taiko-bar-left',(0,230),(150,165));put('pippidonidle',(70,77),(145,145))
            for i in range(5):
                note=sprite('taikohitcircle').resize((88,88));col=(232,120,120) if i%2==0 else accent
                tint=Image.new('RGBA',note.size,col);tint.putalpha(note.getchannel('A'));canvas.alpha_composite(tint,(230+i*146,267));put('taikohitcircleoverlay',(230+i*146,267),(88,88))
        canvas.convert('RGB').save(OUT/theme/f'preview-{scene}.png')
    print(theme+': '+str(len(files))+' stable assets; all documented static assets covered.',flush=True)

# Apply the shared modern spinner after the general artwork pass.
import runpy
runpy.run_path(str(HERE/'build-spinner.py'))
runpy.run_path(str(HERE/'compact-skin-numbers.py'))
