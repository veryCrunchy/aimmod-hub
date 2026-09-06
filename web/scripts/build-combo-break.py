"""Create the original AimMod rounded two-note combo-break cue and update product packs."""
from pathlib import Path
import io, math, struct, wave, zipfile
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
rate=44100
samples=[]
for i in range(round(rate*.38)):
    t=i/rate
    value=0.0
    for onset,freq,decay,gain in [(0,330,.070,1),(.095,196,.085,.88)]:
        u=t-onset
        if u>=0:
            envelope=(1-math.exp(-u/.0018))*math.exp(-u/decay)
            value+=gain*envelope*(math.sin(2*math.pi*freq*u)+.18*math.sin(4*math.pi*freq*u))
    value*=min(1,(.38-t)/.03)
    samples.append(value)
peak=max(map(abs,samples));buf=io.BytesIO()
with wave.open(buf,'wb') as wav:
    wav.setnchannels(1);wav.setsampwidth(2);wav.setframerate(rate)
    wav.writeframes(b''.join(struct.pack('<h',round(v/peak*.94*32767)) for v in samples))
audio=buf.getvalue()
for p in [root/'soft.zip',root/'clicky.zip',*[root/theme/'base.zip' for theme in ['flow','hddt','midnight','glacier']]]:
    with zipfile.ZipFile(p) as z:files={n:z.read(n) for n in z.namelist() if n not in ['combobreak.wav','combobreak.ogg','combobreak.mp3']}
    files['combobreak.wav']=audio
    with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:
        for name,data in sorted(files.items()):z.writestr(zipfile.ZipInfo(name,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
print('Updated AimMod break cue: 380 ms, peak 0.94, rounded descending tones.')
