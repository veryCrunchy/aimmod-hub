import { strFromU8, strToU8, unzipSync, zip, type Zippable } from 'fflate';

export const SKIN_ASSETS = '/skin-builder/v1';
export const skinThemes = [
  { id: 'flow', name: 'Flow', accent: '#77bd98', edge: '#77bd98', description: 'The original AimMod green.' },
  { id: 'hddt', name: 'HDDT', accent: '#eff7f3', edge: '#ebf2ef', description: 'Bright edges. Deep black interiors.' },
  { id: 'midnight', name: 'Midnight', accent: '#27e4a1', edge: '#63d2a3', description: 'Deep green with a jade glow.' },
  { id: 'glacier', name: 'Glacier', accent: '#6acdff', edge: '#98dbf8', description: 'Cool blue with icy edges.' },
] as const;
export const skinGuides = [
  { id: 'subtle', name: 'Subtle', description: 'Short, faint strokes. A little guidance.' },
  { id: 'arrows', name: 'Arrows', description: 'Clear direction between notes.' },
  { id: 'jumps', name: 'Jumps', description: 'No followpoints. Keep the playfield clear.' },
] as const;
export const skinCursors = [
  { id: 'ring', name: 'Ring', icon: '⊙', description: 'An open ring with a bright centre.' },
  { id: 'dot', name: 'Dot', icon: '●', description: 'A solid point with a dark edge.' },
  { id: 'crosshair', name: 'Crosshair', icon: '+', description: 'Four short arms and a centre dot.' },
  { id: 'diamond', name: 'Diamond', icon: '◇', description: 'An angular outline with an open centre.' },
] as const;
export const skinSounds = [
  { id: 'clicky', name: 'AimMod Clicky', description: 'A crisp attack with a rounded body.', creator: 'AimMod', source: '' },
  { id: 'soft', name: 'AimMod Soft', description: 'Low, rounded and restrained.', creator: 'AimMod', source: '' },
  { id: 'rafis', name: 'Rafis HDDT', description: 'The classic 2018 HDDT set.', creator: 'DDK RPK / Rafis', source: 'https://gist.github.com/thomazgg/5fbaf92bed0eac290a7123f5b308dcb0' },
  { id: 'whitecat', name: 'WhiteCat', description: 'Hitsounds from WhiteCat 1.0 NM.', creator: 'cyperdark', source: 'https://osu.ppy.sh/community/forums/topics/986201' },
  { id: 'yugen', name: 'YUGEN', description: 'The familiar Garin sound set.', creator: 'Garin', source: 'https://osu.ppy.sh/community/forums/topics/365036' },
] as const;
export type SkinTheme = typeof skinThemes[number]['id'];
export type SkinGuide = typeof skinGuides[number]['id'];
export type SkinSound = typeof skinSounds[number]['id'];
export type SkinChoice = { theme: SkinTheme; guide: SkinGuide; sound: SkinSound; client: 'lazer' | 'stable'; cursor: typeof skinCursors[number]['id'] };
export type SkinFiles = Record<string, Uint8Array>;
export const defaultSkinChoice: SkinChoice = { theme: 'flow', guide: 'subtle', sound: 'clicky', client: 'lazer', cursor: 'ring' };

export function parseSkinChoice(params: URLSearchParams): SkinChoice {
  return {
    theme: skinThemes.find(t => t.id === params.get('theme'))?.id ?? defaultSkinChoice.theme,
    guide: skinGuides.find(t => t.id === params.get('guide'))?.id ?? defaultSkinChoice.guide,
    sound: skinSounds.find(t => t.id === params.get('sound'))?.id ?? defaultSkinChoice.sound,
    client: params.get('client') === 'stable' ? 'stable' : 'lazer',
    cursor: skinCursors.find(c => c.id === params.get('cursor'))?.id ?? 'ring',
  };
}

const hitSample = /^(normal|soft|drum)-(hit(normal|clap|finish|whistle)|slidertick)\.(wav|ogg|mp3)$/;
const safeAsset = /^[a-z0-9][a-z0-9@_.-]*$/i;

/** Some classic skins intentionally use header-only WAV files to mute a layer. */
export function isSilentWav(bytes: Uint8Array): boolean {
  if (bytes.length < 12 || strFromU8(bytes.subarray(0, 4)) !== 'RIFF' || strFromU8(bytes.subarray(8, 12)) !== 'WAVE') return false;
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  for (let at = 12; at + 8 <= bytes.length;) {
    const size = view.getUint32(at + 4, true);
    if (strFromU8(bytes.subarray(at, at + 4)) === 'data') return size === 0;
    at += 8 + size + (size % 2);
  }
  return false;
}

/** Reject oversized entries before inflating. Only server-pinned/public product archives are loaded. */
export function unpackSkin(bytes: Uint8Array, soundsOnly = false): SkinFiles {
  if (bytes.length > 32 * 1024 * 1024) throw new Error('This sound pack is too large. Choose another set.');
  let total = 0, count = 0;
  const result = unzipSync(bytes, { filter(entry) {
    if (++count > 4096) throw new Error('This skin has too many files.');
    if (!safeAsset.test(entry.name)) return false;
    if (soundsOnly && !hitSample.test(entry.name.toLowerCase())) return false;
    if (entry.originalSize > 8 * 1024 * 1024 || (total += entry.originalSize) > 40 * 1024 * 1024) throw new Error('This skin exceeds the download limit.');
    return true;
  } });
  return result;
}

export function selectHitSamples(files: SkinFiles): SkinFiles {
  const selected: SkinFiles = {};
  // Prefer WAV if an author supplied multiple encodings of the same sample.
  for (const extension of ['mp3', 'ogg', 'wav']) for (const [name, data] of Object.entries(files)) {
    const key = name.toLowerCase();
    if (!hitSample.test(key) || !key.endsWith('.' + extension)) continue;
    const stem = key.slice(0, -(extension.length + 1));
    for (const ext of ['wav', 'ogg', 'mp3']) delete selected[stem + '.' + ext];
    selected[key] = data;
  }
  if (!Object.keys(selected).some(k => /-hitnormal\./.test(k))) throw new Error('This set has no playable hitsounds. Choose another set.');
  return selected;
}

export function assembleSkin(base: SkinFiles, guides: SkinFiles, sounds: SkinFiles, choice: SkinChoice, id: string, stable: SkinFiles = {}, cursor: SkinFiles = {}): SkinFiles {
  const files: SkinFiles = {};
  for (const [name, data] of Object.entries(base)) {
    if (safeAsset.test(name) && (/\.(png|wav|ogg|mp3)$/i.test(name) || ['skin.ini', 'MainHUDComponents.json'].includes(name))) files[name] = data;
  }
  if (!files['skin.ini'] || !files['MainHUDComponents.json']) throw new Error('The skin could not be prepared. Try again.');
  if (choice.client === 'stable') {
    if (!stable['stable.ini'] || !stable['mania-note1.png'] || !stable['fruit-catcher-idle.png'] || !stable['taikohitcircle.png']) throw new Error('The stable artwork could not be loaded. Try again.');
    for (const [name, data] of Object.entries(stable)) if (safeAsset.test(name) && /\.(png|jpg|wav|ogg)$/i.test(name)) files[name] = data;
    files['skin.ini'] = strToU8(strFromU8(files['skin.ini']) + '\n' + strFromU8(stable['stable.ini']));
    delete files['MainHUDComponents.json'];
  } else {
    const gameplay = /^(hit|approachcircle|slider|reversearrow|followpoint|cursor|default-|scoreentry-|aimmod-|scorebar-|inputoverlay-|spinner|star2|lighting|particle)/;
    for (const name of Object.keys(files)) if (/\.png$/i.test(name) && !gameplay.test(name)) delete files[name];
  }
  if (!cursor['cursor.png'] || !cursor['cursor@2x.png']) throw new Error('The cursor could not be loaded. Try again.');
  for (const [name, data] of Object.entries(cursor)) if (/^cursor(?:middle|trail)?(?:@2x)?\.png$/.test(name)) files[name] = data;
  for (const key of Object.keys(files)) if (/^followpoint.*\.png$/i.test(key)) delete files[key];
  if (!guides['followpoint.png'] || !guides['followpoint@2x.png']) throw new Error('The followpoints could not be loaded. Try again.');
  for (const [name, data] of Object.entries(guides)) if (/^followpoint(?:-\d+)?(?:@2x)?\.png$/.test(name)) files[name] = data;
  const selected = selectHitSamples(sounds);
  for (const [name, data] of Object.entries(selected)) {
    const stem = name.replace(/\.(wav|ogg|mp3)$/, '');
    for (const ext of ['wav', 'ogg', 'mp3']) delete files[stem + '.' + ext];
    files[name] = data;
  }
  // Preserve the user's latest AimMod break cue and silence for continuous slider layers.
  const theme = skinThemes.find(t => t.id === choice.theme)!;
  const guide = skinGuides.find(t => t.id === choice.guide)!;
  const sound = skinSounds.find(t => t.id === choice.sound)!;
  const name = `AimMod ${theme.name} · ${guide.name} · ${choice.cursor} · ${sound.name} · ${choice.client}`;
  files['skin.ini'] = strToU8(strFromU8(files['skin.ini']).replace(/^Name\s*:.*$/m, `Name: ${name}`));
  if (choice.client === 'lazer') files['skininfo.json'] = strToU8(JSON.stringify({ ID: id, Name: name, Creator: 'AimMod', InstantiationInfo: 'osu.Game.Skinning.LegacySkin, osu.Game' }, null, 2));
  files['README.txt'] = strToU8(`${name}\n\nImport this .osk into osu!${choice.client === 'lazer' ? 'lazer' : 'stable'}, then select it in skin settings. Enable the key overlay and turn off beatmap skin/colour overrides to use your chosen appearance. Turn off beatmap hitsounds to hear this set.\n\nThis skin does not enable Hidden or Double Time. Followpoints: ${guide.name}. Standard 300 judgements are hidden; 100, 50 and MISS use 80% opacity.\n${choice.client === 'stable' ? 'Includes standard, taiko, catch and mania artwork. PP and the lazer HUD layout are not available in stable.\n' : ''}`);
  files['CREDITS.txt'] = strToU8(`Artwork and layout: AimMod.\nHitsounds: ${sound.name} by ${sound.creator}.\n${sound.source ? 'Source: ' + sound.source + '\n' : ''}Combo break, menu sounds and missing-sample fallbacks: AimMod.\n`);
  return files;
}

export function encodeSkin(files: SkinFiles): Promise<Uint8Array> {
  const data: Zippable = {};
  for (const [name, bytes] of Object.entries(files)) data[name] = [bytes, { mtime: new Date('2026-01-01T00:00:00Z') }];
  return new Promise((resolve, reject) => zip(data, { level: 6 }, (error, bytes) => error ? reject(error) : resolve(bytes)));
}
