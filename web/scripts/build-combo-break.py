"""Create the original AimMod rounded-tail combo-break cue and update product packs."""
from pathlib import Path
import io, math, struct, wave, zipfile
root=Path(__file__).resolve().parents[1]/'public/skin-builder/v1'
rate=44100
samples=[]
for i in range(round(rate*.165)):
    t=i/rate
    value=(1-math.exp(-t/.002))*math.exp(-t/.022)*math.sin(2*math.pi*260*t)
    value+=.21*(1-math.exp(-t/.009))*math.exp(-t/.046)*math.sin(2*math.pi*130*t)
    samples.append(value*min(1,(.165-t)/.015))
peak=max(map(abs,samples));buf=io.BytesIO()
with wave.open(buf,'wb') as wav:
    wav.setnchannels(1);wav.setsampwidth(2);wav.setframerate(rate)
    wav.writeframes(b''.join(struct.pack('<h',round(v/peak*.38*32767)) for v in samples))
audio=buf.getvalue()
for p in [root/'soft.zip',root/'clicky.zip',*[root/theme/'base.zip' for theme in ['flow','hddt','midnight','glacier']]]:
    with zipfile.ZipFile(p) as z:files={n:z.read(n) for n in z.namelist() if n not in ['combobreak.wav','combobreak.ogg','combobreak.mp3']}
    files['combobreak.wav']=audio
    with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:
        for name,data in sorted(files.items()):z.writestr(zipfile.ZipInfo(name,(2026,1,1,0,0,0)),data,compress_type=zipfile.ZIP_DEFLATED)
print('Updated AimMod break cue: 165 ms, peak 0.38, rounded tap with a low tail.')
