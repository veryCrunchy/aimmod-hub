import { useEffect, useRef, useState } from 'react';
import { SKIN_ASSETS, skinThemes, type SkinChoice } from '../lib/skinBuilder';

const previewNames = ['hitcircle', 'hitcircleoverlay', 'approachcircle', 'reversearrow', 'cursor', 'scorebar-bg', 'scorebar-colour',
  'aimmod-timing-track', 'aimmod-duration-face', ...[1, 2, 3, 4].map(n => `default-${n}`),
  ...Array.from({ length: 10 }, (_, n) => `aimmod-score-${n}`), 'aimmod-score-dot', 'aimmod-score-percent', 'aimmod-score-pp'];

export function SkinBuilderPreview({ choice, playing, pattern }: { choice: SkinChoice; playing: boolean; pattern: 'jumps' | 'sliders' }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState(false);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    let disposed = false, frame = 0;
    const images = new Map<string, HTMLImageElement>();
    setReady(false); setError(false);
    const names = [...previewNames, `guide-${choice.guide}`, `cursor-${choice.cursor}`];
    Promise.all(names.map(name => new Promise<void>((resolve, reject) => {
      const img = new Image(); img.onload = () => { images.set(name, img); resolve(); }; img.onerror = reject;
      img.src = `${SKIN_ASSETS}/${choice.theme}/${name}@2x.png`;
    }))).then(() => {
      if (disposed) return;
      setReady(true);
      const el = canvas.current; const ctx = el?.getContext('2d'); if (!el || !ctx) return;
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      el.width = 960 * dpr; el.height = 600 * dpr;
      const theme = skinThemes.find(t => t.id === choice.theme)!;
      const start = performance.now();
      const draw = (now: number) => {
        if (disposed) return;
        const t = playing ? ((now - start) % 2400) / 2400 : .35;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0); ctx.clearRect(0, 0, 960, 600); ctx.fillStyle = '#030605'; ctx.fillRect(0, 0, 960, 600);
        const image = (name: string, x: number, y: number, w: number, h?: number) => {
          const im = images.get(name); if (im) ctx.drawImage(im, x, y, w, h ?? w * im.height / im.width);
        };
        const digits = (value: string, right: number, y: number, height: number) => {
          const parts = value.match(/pp|%|\.|\d/g) ?? [];
          const sprites = parts.map(p => images.get('aimmod-score-' + ({ '.': 'dot', '%': 'percent' }[p] ?? p))).filter(Boolean) as HTMLImageElement[];
          let x = right - sprites.reduce((sum, im) => sum + height * im.width / im.height - 1, 0);
          for (const im of sprites) { const width = height * im.width / im.height; ctx.drawImage(im, x, y, width, height); x += width - 1; }
        };
        image('scorebar-bg', 22, 12, 260); image('scorebar-colour', 29, 19, 242);
        if (choice.client === 'lazer') digits('214pp', 495, 24, 25); digits('98.72%', 697, 24, 25); digits('00428160', 929, 24, 25);
        const points = pattern === 'jumps' ? [[282, 230], [670, 390], [305, 402], [665, 218]] : [[285, 280], [560, 235], [670, 395], [430, 400]];
        if (choice.guide !== 'jumps') for (let i = 0; i < 3; i++) {
          const [x1, y1] = points[i], [x2, y2] = points[i + 1];
          const length = Math.hypot(x2 - x1, y2 - y1), angle = Math.atan2(y2 - y1, x2 - x1);
          for (let distance = 78; distance < length - 66; distance += 37) {
            ctx.save(); ctx.translate(x1 + (x2 - x1) * distance / length, y1 + (y2 - y1) * distance / length); ctx.rotate(angle); ctx.globalAlpha = .68;
            const width = choice.guide === 'arrows' ? 23 : 18; image(`guide-${choice.guide}`, -width / 2, -4, width, 8); ctx.restore();
          }
        }
        if (pattern === 'sliders') {
          ctx.beginPath(); ctx.moveTo(...points[0] as [number, number]); ctx.bezierCurveTo(370, 225, 465, 320, ...points[1] as [number, number]);
          ctx.strokeStyle = theme.edge; ctx.lineWidth = 100; ctx.lineCap = 'round'; ctx.stroke();
          ctx.strokeStyle = '#07100b'; ctx.lineWidth = 88; ctx.stroke();
        }
        for (let i = 3; i >= 0; i--) {
          const [x, y] = points[i]; const opacity = playing ? .55 + .45 * Math.max(0, Math.sin((t + i * .18) * Math.PI)) : 1;
          ctx.globalAlpha = opacity; image('hitcircle', x - 59, y - 59, 118); image('hitcircleoverlay', x - 59, y - 59, 118);
          // A slider's reverse endpoint shows only the reverse indicator.
          if (pattern !== 'sliders' || i !== 1) {
            const number = images.get(`default-${i + 1}`)!; const h = 43, w = h * number.width / number.height; ctx.drawImage(number, x - w / 2, y - h / 2, w, h);
          }
        }
        ctx.globalAlpha = .55;
        const ring = 128 + (1 - t) * 120; image('approachcircle', points[0][0] - ring / 2, points[0][1] - ring / 2, ring); ctx.globalAlpha = 1;
        if (pattern === 'sliders') { ctx.save(); ctx.translate(points[1][0], points[1][1]); ctx.rotate(Math.atan2(points[0][1] - points[1][1], points[0][0] - points[1][0])); image('reversearrow', -64, -64, 128); ctx.restore(); }
        const cursorX = points[0][0] + (points[1][0] - points[0][0]) * t, cursorY = points[0][1] + (points[1][1] - points[0][1]) * t;
        image(`cursor-${choice.cursor}`, cursorX - 12, cursorY - 12, 24);
        digits('128', 115, 529, 42);
        if (choice.client === 'lazer') { image('aimmod-timing-track', 330, 548, 300); image('aimmod-duration-face', 853, 501, 68); }
        ctx.strokeStyle = theme.accent; ctx.lineWidth = 2; ctx.beginPath(); ctx.arc(887, 535, 20, -Math.PI / 2, Math.PI * (.3 + t)); if (choice.client === 'lazer') ctx.stroke();
        for (let i = 0; i < 3; i++) { digits(String([126, 130, 0][i]), 918, 205 + i * 48, 19); ctx.fillStyle = theme.accent; ctx.fillRect(896, 231 + i * 48, 22, 2); }
        if (playing) frame = requestAnimationFrame(draw);
      };
      draw(performance.now());
    }).catch(() => { if (!disposed) setError(true); });
    return () => { disposed = true; cancelAnimationFrame(frame); };
  }, [choice.theme, choice.guide, choice.client, choice.cursor, playing, pattern]);
  return <div className="skin-scene">
    <canvas ref={canvas} role="img" aria-label={`${choice.theme} skin with ${choice.guide} followpoints, ${pattern} pattern`} />
    {!ready && <div className="skin-scene-status" role={error ? 'alert' : 'status'}>{error ? 'Preview unavailable. Choose a colour to retry.' : 'Loading preview…'}</div>}
  </div>;
}
