import { useEffect, useRef, useState } from 'react';
import { spinnerPreviewState, sliderPreviewPosition } from '../lib/skinPreview';
import { SKIN_ASSETS, skinThemes, trailColour, type SkinChoice } from '../lib/skinBuilder';

const previewNames = ['hitcircle', 'hitcircleoverlay', 'approachcircle', 'reversearrow', 'sliderb0', 'cursor', 'scorebar-bg', 'scorebar-colour',
  'spinner-bottom', 'spinner-top', 'spinner-middle', 'spinner-middle2', 'spinner-glow', 'spinner-approachcircle', 'spinner-spin', 'spinner-clear', 'spinner-rpm', 'aimmod-timing-track', 'aimmod-duration-face', ...[1, 2, 3, 4].map(n => `default-${n}`),
  ...Array.from({ length: 10 }, (_, n) => `aimmod-combo-${n}`), ...Array.from({ length: 10 }, (_, n) => `aimmod-score-${n}`), 'aimmod-score-dot', 'aimmod-score-percent', 'aimmod-score-pp'];

export function SkinBuilderPreview({ choice, playing, pattern }: { choice: SkinChoice; playing: boolean; pattern: 'jumps' | 'sliders' | 'spinner' }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const elapsedRef = useRef(2.8);
  const [error, setError] = useState(false);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    let disposed = false, frame = 0, animationStarted: number | null = null;
    const images = new Map<string, HTMLImageElement>();
    setReady(false); setError(false);
    const names = [...previewNames, `guide-${choice.guide}`, `cursor-${choice.cursor}`, 'cursor-trail'];
    Promise.all(names.map(name => new Promise<void>((resolve, reject) => {
      const img = new Image(); img.onload = () => { void img.decode().then(() => { images.set(name, img); resolve(); }, reject); }; img.onerror = reject;
      img.src = name === 'cursor-trail' ? `${SKIN_ASSETS}/trails/${trailColour(choice)}-${choice.trail}@2x.png` : `${SKIN_ASSETS}/${choice.theme}/${name.startsWith('spinner-') ? 'spinner-' + choice.spinner + '/' : ''}${name}@2x.png`;
    }))).then(() => {
      if (disposed) return;
      const el = canvas.current; const ctx = el?.getContext('2d'); if (!el || !ctx) return;
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      el.width = 960 * dpr; el.height = 600 * dpr;
      const theme = skinThemes.find(t => t.id === choice.theme)!;
      const start = performance.now(); animationStarted = start;
      const initialElapsed = elapsedRef.current;
      const tinted = new Map<string, HTMLCanvasElement>();
      const tint = (name: string, colour: string) => {
        const source = images.get(name)!;
        const surface = document.createElement('canvas'); surface.width = source.width; surface.height = source.height;
        const paint = surface.getContext('2d')!; paint.drawImage(source, 0, 0);
        paint.globalCompositeOperation = 'multiply'; paint.fillStyle = colour; paint.fillRect(0, 0, surface.width, surface.height);
        paint.globalCompositeOperation = 'destination-in'; paint.drawImage(source, 0, 0);
        return surface;
      };
      tinted.set('hitcircle', tint('hitcircle', theme.edge));
      tinted.set('approachcircle', tint('approachcircle', theme.edge));
      tinted.set('spinner-glow-blue', tint('spinner-glow', '#0397ff'));

      const draw = (now: number) => {
        if (disposed) return;
        const elapsed = initialElapsed + (playing ? (now - start) / 1000 : 0);
        const t = (elapsed % 2.4) / 2.4;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0); ctx.clearRect(0, 0, 960, 600); ctx.fillStyle = '#030605'; ctx.fillRect(0, 0, 960, 600);
        const image = (name: string, x: number, y: number, w: number, h?: number) => {
          const im = tinted.get(name) ?? images.get(name); if (im) ctx.drawImage(im, x, y, w, h ?? w * im.height / im.width);
        };
        const drawTrail = (position: (time: number) => [number, number]) => {
          if (choice.trail === 'off') return;
          const life = choice.trail === 'glow' ? .24 : choice.trail === 'dots' ? .13 : .18;
          const step = choice.trail === 'dots' ? .022 : .006;
          const width = 24 * Number(choice.cursorSize);
          ctx.save();
          for (let age = life; age > 0; age -= step) {
            const [x, y] = position(Math.max(0, elapsed - age));
            ctx.globalAlpha = (1 - age / life) * .7;
            image('cursor-trail', x - width / 2, y - width / 2, width);
          }
          ctx.restore();
        };
        const digits = (value: string, right: number, y: number, height: number, prefix = 'aimmod-score-') => {
          const parts = value.match(/pp|%|\.|\d/g) ?? [];
          const sprites = parts.map(p => images.get(prefix + ({ '.': 'dot', '%': 'percent' }[p] ?? p))).filter(Boolean) as HTMLImageElement[];
          let x = right - sprites.reduce((sum, im) => sum + height * im.width / im.height - 1, 0);
          for (const im of sprites) { const width = height * im.width / im.height; ctx.drawImage(im, x, y, width, height); x += width - 1; }
        };
        image('scorebar-bg', 22, 12, 260); image('scorebar-colour', 29, 19, 242);
        if (choice.client === 'lazer') digits('214pp', 495, 24, 25); digits('98.72%', 697, 24, 25); digits('00428160', 929, 24, 25);
        if (pattern === 'spinner') {
          const state = spinnerPreviewState(elapsed);
          const rotation = state.rotation;
          const spinnerLayer = (name: string, angle: number, width = 350 * state.scale) => { ctx.save(); ctx.translate(480, 295); ctx.rotate(angle); image(name, -width / 2, -width / 2, width); ctx.restore(); };
          ctx.save(); ctx.globalCompositeOperation = 'lighter'; ctx.globalAlpha = state.progress;
          spinnerLayer('spinner-glow-blue', 0);
          ctx.globalAlpha *= state.flash; spinnerLayer('spinner-glow', 0); ctx.restore();
          spinnerLayer('spinner-bottom', rotation / 6); spinnerLayer('spinner-top', rotation / 2);
          spinnerLayer('spinner-middle2', rotation); spinnerLayer('spinner-middle', 0);
          spinnerLayer('spinner-approachcircle', 0, 350 * state.approachScale);
          ctx.save(); ctx.globalAlpha = state.cleared ? 1 : state.promptAlpha;
          image(state.cleared ? 'spinner-clear' : 'spinner-spin', 430, state.cleared ? 115 : 458, 100); ctx.restore();
          image('spinner-rpm', 402, 501, 156); digits(String(state.rpm), 552, 500, 24);
          drawTrail(time => { const angle = spinnerPreviewState(time).rotation; return [480 + Math.cos(angle) * 110, 295 + Math.sin(angle) * 110]; });
          const width = 24 * Number(choice.cursorSize);
          image(`cursor-${choice.cursor}`, 480 + Math.cos(rotation) * 110 - width/2, 295 + Math.sin(rotation) * 110 - width/2, width);
        } else {
        const points = pattern === 'jumps' ? [[282, 230], [670, 390], [305, 402], [665, 218]] : [[285, 280], [560, 235], [670, 395], [430, 400]];
        if (choice.guide !== 'jumps') for (let i = 0; i < 3; i++) {
          if (pattern === 'sliders' && i === 0) continue;
          const [x1, y1] = points[i], [x2, y2] = points[i + 1];
          const length = Math.hypot(x2 - x1, y2 - y1), angle = Math.atan2(y2 - y1, x2 - x1);
          for (let distance = 78; distance < length - 66; distance += 37) {
            ctx.save(); ctx.translate(x1 + (x2 - x1) * distance / length, y1 + (y2 - y1) * distance / length); ctx.rotate(angle); ctx.globalAlpha = .68;
            const width = choice.guide === 'line' ? 37 : choice.guide === 'arrows' ? 23 : 18; image(`guide-${choice.guide}`, -width / 2, -4, width, 8); ctx.restore();
          }
        }
        if (pattern === 'sliders') {
          ctx.beginPath(); ctx.moveTo(...points[0] as [number, number]); ctx.bezierCurveTo(370, 225, 465, 320, ...points[1] as [number, number]);
          ctx.strokeStyle = theme.edge; ctx.lineWidth = 100; ctx.lineCap = 'round'; ctx.stroke();
          ctx.strokeStyle = '#040d09'; ctx.lineWidth = 88; ctx.stroke();
        }
        for (let i = 3; i >= 0; i--) {
          if (pattern === 'sliders' && i === 1) continue;
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
        const [cursorX, cursorY] = pattern === 'sliders'
          ? sliderPreviewPosition(t < .5 ? t * 2 : 2 - t * 2)
          : [points[0][0] + (points[1][0] - points[0][0]) * t, points[0][1] + (points[1][1] - points[0][1]) * t];
        if (pattern === 'sliders') image('sliderb0', cursorX - 59, cursorY - 59, 118);
        drawTrail(time => {
          const phase = (time % 2.4) / 2.4;
          return pattern === 'sliders' ? sliderPreviewPosition(phase < .5 ? phase * 2 : 2 - phase * 2)
            : [points[0][0] + (points[1][0] - points[0][0]) * phase, points[0][1] + (points[1][1] - points[0][1]) * phase];
        });
        const cursorWidth = 24 * Number(choice.cursorSize);
        image(`cursor-${choice.cursor}`, cursorX - cursorWidth / 2, cursorY - cursorWidth / 2, cursorWidth);
        }
        digits('128', 92, 539, 31, 'aimmod-combo-');
        if (choice.client === 'lazer') { image('aimmod-timing-track', 330, 548, 300); image('aimmod-duration-face', 853, 501, 68); }
        ctx.strokeStyle = theme.accent; ctx.lineWidth = 2; ctx.beginPath(); ctx.arc(887, 535, 20, -Math.PI / 2, Math.PI * (.3 + t)); if (choice.client === 'lazer') ctx.stroke();
        for (let i = 0; i < 3; i++) { digits(String([126, 130, 0][i]), 918, 205 + i * 48, 19); ctx.fillStyle = theme.accent; ctx.fillRect(896, 231 + i * 48, 22, 2); }
        if (playing) frame = requestAnimationFrame(draw);
      };
      draw(performance.now()); setReady(true);
    }).catch(() => { if (!disposed) setError(true); });
    return () => { if (playing && animationStarted !== null) elapsedRef.current += (performance.now() - animationStarted) / 1000; disposed = true; cancelAnimationFrame(frame); };
  }, [choice.theme, choice.guide, choice.client, choice.cursor, choice.cursorSize, choice.spinner, choice.trail, playing, pattern]);
  return <div className="skin-scene">
    <canvas ref={canvas} role="img" aria-label={`${choice.theme} skin with ${choice.guide} followpoints, ${pattern} pattern`} />
    {!ready && <div className="skin-scene-status" role={error ? 'alert' : 'status'}>{error ? 'Preview unavailable. Choose a colour to retry.' : 'Loading preview…'}</div>}
  </div>;
}
