import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Helmet } from '../lib/helmet';
import { SkinBuilderPreview } from '../components/SkinBuilderPreview';
import { assembleSkin, encodeSkin, isSilentWav, parseSkinChoice, selectComboBreak, selectHitSamples, skinCursors, skinCursorSizes, skinGuides, skinSounds, skinSpinners, skinThemes, SKIN_ASSETS, type SkinChoice, type SkinFiles } from '../lib/skinBuilder';
import { loadBuilderArchive, soundArchiveURL } from '../lib/skinBuilderLoader';
import './skinBuilder.css';

export function OsuSkinBuilderPage() {
  const [params, setParams] = useSearchParams();
  const choice = parseSkinChoice(params);
  const theme = skinThemes.find(t => t.id === choice.theme)!;
  const sound = skinSounds.find(s => s.id === choice.sound)!;
  const [playing, setPlaying] = useState(false);
  const [pattern, setPattern] = useState<'jumps' | 'sliders' | 'spinner'>('jumps');
  const [scene, setScene] = useState('standard');
  const [busy, setBusy] = useState(false);
  const [audioBusy, setAudioBusy] = useState(false);
  const [listening, setListening] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const requests = useRef(new Set<AbortController>());
  const audioContext = useRef<AudioContext | null>(null);
  const audioNodes = useRef<AudioBufferSourceNode[]>([]);
  const audioRequest = useRef<AbortController | null>(null);
  const soundCache = useRef(new Map<string, SkinFiles>());
  const mounted = useRef(true);
  const listenTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stopAudio = () => {
    audioRequest.current?.abort(); audioRequest.current = null;
    for (const node of audioNodes.current) { try { node.stop(); } catch { /* Already ended. */ } }
    audioNodes.current = [];
    if (listenTimer.current) clearTimeout(listenTimer.current);
    setListening(false); setAudioBusy(false);
  };
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; for (const controller of requests.current) controller.abort(); audioRequest.current?.abort(); for (const node of audioNodes.current) { try { node.stop(); } catch { /* Ended. */ } } if (listenTimer.current) clearTimeout(listenTimer.current); void audioContext.current?.close(); audioContext.current = null; };
  }, []);
  const update = (patch: Partial<SkinChoice>) => {
    if (patch.sound) stopAudio();
    setError(''); setMessage('');
    const next = { ...choice, ...patch }; setParams(new URLSearchParams(next), { replace: true });
    if (patch.client === 'lazer') setScene('standard');
  };
  async function sounds(signal: AbortSignal, selected: SkinChoice): Promise<SkinFiles> {
    const cached = soundCache.current.get(selected.sound); if (cached) return cached;
    const files = await loadBuilderArchive(soundArchiveURL(selected.sound), signal, selected.sound !== 'clicky' && selected.sound !== 'soft');
    signal.throwIfAborted(); selectHitSamples(files); soundCache.current.set(selected.sound, files); return files;
  }
  async function listen(breakCue = false) {
    stopAudio(); setError(''); setAudioBusy(true);
    const controller = new AbortController(); audioRequest.current = controller;
    try {
      const context = audioContext.current ??= new AudioContext(); await context.resume();
      let files = await sounds(controller.signal, choice);
      if (breakCue) {
        files = selectComboBreak(files);
        if (!Object.keys(files).length) files = selectComboBreak(await loadBuilderArchive(`${SKIN_ASSETS}/soft.zip`, controller.signal));
      }
      const names = breakCue ? [Object.keys(files)[0]] : ['hitnormal', 'hitclap', 'hitfinish', 'hitwhistle'].map(kind => Object.keys(files).find(n => n.startsWith(`normal-${kind}.`)) ?? Object.keys(files).find(n => n.includes(`-${kind}.`)));
      const buffers = await Promise.all(names.map(async n => n && !isSilentWav(files[n]) ? context.decodeAudioData(files[n].slice().buffer as ArrayBuffer) : null));
      controller.signal.throwIfAborted();
      const times = breakCue ? [0] : [0, .36, .72, .84, .96, 1.44, 1.8, 1.92, 2.04];
      const start = context.currentTime + .05;
      const play = (buffer: AudioBuffer | null, at: number) => {
        if (!buffer) return;
        const node = context.createBufferSource(); node.buffer = buffer;
        const volume = context.createGain(); volume.gain.value = .55; node.connect(volume); volume.connect(context.destination);
        node.onended = () => { node.disconnect(); volume.disconnect(); };
        node.start(start + at); audioNodes.current.push(node);
      };
      times.forEach((at, i) => { play(buffers[0], at); if (!breakCue && i % 3 === 1) play(buffers[1 + Math.floor(i / 3) % 3] ?? null, at); });
      setAudioBusy(false); setListening(true);
      listenTimer.current = setTimeout(() => { if (mounted.current) { setListening(false); audioNodes.current = []; } }, breakCue ? Math.max(1200, (buffers[0]?.duration ?? 0) * 1000 + 100) : 3000);
    } catch (e) {
      if (!controller.signal.aborted && mounted.current) { setError(e instanceof Error ? e.message : 'This sound could not be played. Try another set.'); setAudioBusy(false); }
    }
  }
  async function download() {
    const selected = { ...choice }; const controller = new AbortController(); requests.current.add(controller);
    setBusy(true); setError(''); setMessage('');
    try {
      const [base, guides, audio, stable, cursor, spinner] = await Promise.all([
        loadBuilderArchive(`${SKIN_ASSETS}/${selected.theme}/base.zip`, controller.signal),
        loadBuilderArchive(`${SKIN_ASSETS}/${selected.theme}/${selected.guide}.zip`, controller.signal),
        sounds(controller.signal, selected),
        selected.client === 'stable' ? loadBuilderArchive(`${SKIN_ASSETS}/${selected.theme}/stable.zip`, controller.signal) : Promise.resolve({}),
        loadBuilderArchive(`${SKIN_ASSETS}/${selected.theme}/cursor-${selected.cursor}${selected.cursorSize === '1' ? '' : '-' + selected.cursorSize}.zip`, controller.signal),
        loadBuilderArchive(`${SKIN_ASSETS}/${selected.theme}/spinner-${selected.spinner}.zip`, controller.signal),
      ]);
      const packed = await encodeSkin(assembleSkin(base, guides, audio, selected, crypto.randomUUID(), stable, cursor, spinner));
      controller.signal.throwIfAborted();
      const url = URL.createObjectURL(new Blob([packed.slice().buffer as ArrayBuffer], { type: 'application/octet-stream' }));
      const link = document.createElement('a'); link.href = url; link.download = `AimMod-${selected.theme}-${selected.guide}-${selected.sound}-${selected.cursor}-${selected.cursorSize}x-${selected.spinner}-${selected.client}.osk`; document.body.appendChild(link); link.click(); link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 30000); setMessage('Your skin is ready. Open the .osk file to import it.');
    } catch (e) { if (!controller.signal.aborted && mounted.current) setError(e instanceof Error ? e.message : 'Your skin could not be downloaded. Try again.'); }
    finally { controller.abort(); requests.current.delete(controller); if (mounted.current) setBusy(false); }
  }
  return <section className="skin-builder" style={{ '--skin-accent': theme.accent } as CSSProperties}>
    <Helmet><title>osu! Skin Builder · AimMod Hub</title><meta name="description" content="Build your AimMod osu! skin. Preview colour themes, followpoints and hitsounds, then download for lazer or stable." /></Helmet>
    <div className="skin-builder-heading"><div><Link to="/osu/skins" className="skin-back">← Skin library</Link><p className="skin-eyebrow">AIMMOD SKIN STUDIO</p><h1>Your aim. Your look.</h1><p>Choose your colours, find your sound, make it yours.</p></div><span className="skin-format">.osk</span></div>
    <div className="skin-builder-layout">
      <div className="skin-controls">
        <fieldset><legend><span>01</span> Client</legend><div className="skin-segments">{(['lazer', 'stable'] as const).map(client => <button key={client} type="button" aria-pressed={choice.client === client} onClick={() => update({ client })}>osu!{client}</button>)}</div><p className="skin-help">{choice.client === 'lazer' ? 'A compact gameplay pack with the native lazer PP counter.' : 'Four modes, menus, pause screens and results. No PP overlay.'}</p></fieldset>
        <fieldset><legend><span>02</span> Colour</legend><div className="skin-theme-grid">{skinThemes.map(t => <button type="button" key={t.id} aria-pressed={choice.theme === t.id} onClick={() => update({ theme: t.id })}><span className="skin-swatch" style={{ borderColor: t.edge, boxShadow: `inset 0 0 14px ${t.edge}30` }} /><span>{t.name}</span>{choice.theme === t.id && <span aria-hidden="true" className="skin-check">✓</span>}</button>)}</div><p className="skin-help">{theme.description}</p></fieldset>
        <fieldset><legend><span>03</span> Followpoints</legend><div className="skin-guide-grid">{skinGuides.map(g => <button type="button" key={g.id} aria-pressed={choice.guide === g.id} onClick={() => update({ guide: g.id })}><span aria-hidden="true" className={`skin-guide-icon ${g.id}`}>{g.id === 'arrows' ? '→ →' : g.id === 'subtle' ? '– –' : '○   ○'}</span>{g.name}</button>)}</div><p className="skin-help">{skinGuides.find(g => g.id === choice.guide)!.description}</p></fieldset>
        <fieldset><legend><span>04</span> Cursor</legend><div className="skin-guide-grid skin-cursor-grid">{skinCursors.map(c => <button type="button" key={c.id} aria-pressed={choice.cursor === c.id} onClick={() => update({ cursor: c.id })}><img alt="" className="skin-cursor-option" src={`${SKIN_ASSETS}/${choice.theme}/cursor-${c.id}@2x.png`} />{c.name}</button>)}</div><p className="skin-help">{skinCursors.find(c => c.id === choice.cursor)!.description}</p><label className="skin-size-label" htmlFor="cursor-size">Cursor size</label><select id="cursor-size" value={choice.cursorSize} onChange={event => update({ cursorSize: event.target.value as SkinChoice['cursorSize'] })}>{skinCursorSizes.map(size => <option key={size.id} value={size.id}>{size.name} · {size.id}×</option>)}</select><p className="skin-help">Applied to the cursor and trail in your download. Your in-game cursor scale also affects the final size.</p></fieldset>
        <fieldset><legend><span>05</span> Spinner</legend><div className="skin-guide-grid">{skinSpinners.map(spinner => <button type="button" key={spinner.id} aria-pressed={choice.spinner === spinner.id} onClick={() => { update({ spinner: spinner.id }); setScene('standard'); setPattern('spinner'); }}><img className="skin-spinner-option" alt="" src={`${SKIN_ASSETS}/${choice.theme}/spinner-${spinner.id}/preview.png`} />{spinner.name}</button>)}</div><p className="skin-help">{skinSpinners.find(s => s.id === choice.spinner)!.description}</p></fieldset>
        <fieldset><legend><span>06</span> Hitsounds</legend><div className="skin-sounds">{skinSounds.map(s => <button type="button" key={s.id} aria-pressed={choice.sound === s.id} onClick={() => update({ sound: s.id })}><span className="skin-radio" aria-hidden="true" /><span><strong>{s.name}</strong><small>{s.description}</small></span></button>)}</div><div className="skin-audio-actions"><button type="button" onClick={() => listening || audioBusy ? stopAudio() : void listen()}>{audioBusy ? 'Cancel loading' : listening ? '■ Stop' : '▶ Listen'}</button><button type="button" onClick={() => void listen(true)}>Combo break</button></div>{sound.source && <a className="skin-credit" href={sound.source} target="_blank" rel="noreferrer">{sound.creator} · Original skin ↗</a>}</fieldset>
      </div>
      <div className="skin-preview-column"><div className="skin-preview-sticky">
        <div className="skin-preview-toolbar"><strong>{theme.name}<span> / {choice.client}</span></strong><div className="skin-preview-options">{scene === 'standard' && <><button type="button" aria-pressed={pattern === 'jumps'} onClick={() => setPattern('jumps')}>Jumps</button><button type="button" aria-pressed={pattern === 'sliders'} onClick={() => setPattern('sliders')}>Sliders</button><button type="button" aria-pressed={pattern === 'spinner'} onClick={() => setPattern('spinner')}>Spinner</button><button type="button" onClick={() => setPlaying(!playing)} aria-label={playing ? 'Pause preview' : 'Animate preview'}>{playing ? 'Ⅱ' : '▶'}</button></>}</div></div>
        {choice.client === 'stable' && <div className="skin-mode-tabs" aria-label="Preview scene">{['standard', 'taiko', 'catch', 'mania', 'menu', 'pause', 'results'].map(mode => <button key={mode} type="button" aria-pressed={scene === mode} onClick={() => setScene(mode)}>{mode === 'standard' ? 'osu!' : mode}</button>)}</div>}
        {scene === 'standard' ? <SkinBuilderPreview choice={choice} playing={playing} pattern={pattern} /> : <img className="skin-stable-preview" src={`${SKIN_ASSETS}/${choice.theme}/preview-${scene}.png`} alt={`${theme.name} ${scene} skin artwork`} />}
        <div className="skin-preview-caption"><span>{scene === 'standard' ? 'Gameplay preview' : 'Artwork preview'}</span><span>Custom glyphs · Quiet judgements</span></div>
        <div className="skin-download-panel"><div><p className="skin-eyebrow">YOUR BUILD</p><h2>{theme.name} <span>· {skinGuides.find(g => g.id === choice.guide)!.name}</span></h2><p>{sound.name} / osu!{choice.client}{choice.client === 'stable' ? ' / All four modes' : ''}</p></div><button type="button" className="skin-download" disabled={busy} onClick={() => void download()}>{busy ? 'Preparing skin…' : 'Download skin ↓'}</button></div>
        <div aria-live="polite">{message && <p className="skin-success">{message}</p>}{error && <p className="skin-error" role="alert">{error}</p>}</div>
        <details className="skin-install"><summary>How to install</summary><ol><li>Open the downloaded .osk file with osu!{choice.client}.</li><li>Select your AimMod skin in the game’s skin settings.</li><li>Disable beatmap skin, colour and hitsound overrides to use your selection.</li></ol></details>
      </div></div>
    </div>
  </section>;
}
