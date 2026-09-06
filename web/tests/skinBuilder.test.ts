import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { strFromU8, strToU8, zipSync, unzipSync } from 'fflate';
import { assembleSkin, defaultSkinChoice, encodeSkin, isSilentAudio, isSilentWav, parseSkinChoice, selectHitSamples, selectComboBreak, skinCursors, skinGuides, skinThemes, unpackSkin } from '../src/lib/skinBuilder';

const asset = (path: string) => unpackSkin(new Uint8Array(readFileSync(new URL('../public/skin-builder/v1/' + path, import.meta.url))));
const sounds = { soft: asset('soft.zip'), clicky: asset('clicky.zip') };
const id = '00000000-0000-4000-8000-000000000001';

test('URL choices are allowlisted and default to the subtle lazer pack', () => {
  assert.deepEqual(parseSkinChoice(new URLSearchParams('theme=../../other&sound=unknown&client=unknown&cursor=unknown')), defaultSkinChoice);
  assert.deepEqual(parseSkinChoice(new URLSearchParams('theme=hddt&sound=rafis&guide=jumps&client=stable&cursor=dot')), { theme: 'hddt', sound: 'rafis', guide: 'jumps', client: 'stable', cursor: 'dot', cursorSize: '1', spinner: 'orbit', trail: 'soft' });
});

test('every built-in combination retains its chosen cursor, guides, audio and correct client layout', () => {
  for (const theme of skinThemes) {
    const base = asset(`${theme.id}/base.zip`), stable = asset(`${theme.id}/stable.zip`);
    for (const guide of skinGuides) {
      const guides = asset(`${theme.id}/${guide.id}.zip`);
      for (const cursor of skinCursors) {
        const cursors = asset(`${theme.id}/cursor-${cursor.id}.zip`);
        for (const sound of ['soft', 'clicky'] as const) for (const client of ['lazer', 'stable'] as const) {
          const result = assembleSkin(base, guides, sounds[sound], { theme: theme.id, guide: guide.id, cursor: cursor.id, cursorSize: '1', spinner: 'orbit', trail: 'soft', sound, client }, id, stable, cursors);
          assert.deepEqual(result['cursor.png'], cursors['cursor.png']);
          assert.deepEqual(result['followpoint.png'], guides['followpoint.png']);
          assert.deepEqual(result['normal-hitnormal.wav'], sounds[sound]['normal-hitnormal.wav']);
          assert.deepEqual(result['combobreak.wav'], base['combobreak.wav']);
          assert.deepEqual(result['hit300.png'], base['hit300.png']);
          assert.ok(!Object.keys(result).some(name => name.includes('READABILITY')));
          if (client === 'lazer') {
            assert.ok(result['MainHUDComponents.json'] && result['skininfo.json']);
            assert.ok(!result['mania-note1.png'] && !result['menu-background.jpg'] && !result['ranking-panel.png']);
          } else {
            assert.ok(!result['MainHUDComponents.json'] && !result['skininfo.json']);
            assert.ok(result['mania-note1.png'] && result['fruit-catcher-idle.png'] && result['taikohitcircle.png'] && result['ranking-panel.png']);
            const ini = strFromU8(result['skin.ini']);
            assert.equal((ini.match(/\[Mania\]/g) ?? []).length, 18);
            for (const match of ini.matchAll(/^(?:KeyImage\d+D?|NoteImage\d+[HLT]?|Stage(?:Left|Right|Bottom|Light|Hint)|Lighting[NL]): (.+)$/gm)) assert.ok(result[match[1].trim() + '.png'], match[1]);
          }
        }
      }
    }
  }
});

test('all documented stable static artwork is present in the full skin', () => {
  const catalog = JSON.parse(readFileSync(new URL('../scripts/stable-skin-assets.json', import.meta.url), 'utf8'));
  const result = assembleSkin(asset('flow/base.zip'), asset('flow/subtle.zip'), sounds.soft, { ...defaultSkinChoice, client: 'stable' }, id, asset('flow/stable.zip'), asset('flow/cursor-ring.zip'));
  for (const name of Object.keys(catalog)) {
    if (/^spinner-(background|metre|osu)(@2x)?\.png$/.test(name)) continue; // Mutually exclusive old spinner style.
    assert.ok(result[name], name);
  }
});

test('community samples replace competing encodings but never import community artwork', () => {
  const base = asset('flow/base.zip');
  const foreign = { 'normal-hitnormal.ogg': strToU8('audio-fixture'), 'hitcircle.png': strToU8('not-our-art'), 'skin.ini': strToU8('Name: foreign'), 'combobreak.wav': strToU8('foreign-break') };
  const result = assembleSkin(base, asset('flow/arrows.zip'), foreign, { ...defaultSkinChoice, sound: 'rafis' }, id, {}, asset('flow/cursor-dot.zip'));
  assert.ok(!result['normal-hitnormal.wav']);
  assert.deepEqual(result['normal-hitnormal.ogg'], foreign['normal-hitnormal.ogg']);
  assert.deepEqual(result['hitcircle.png'], base['hitcircle.png']);
  assert.deepEqual(result['combobreak.wav'], foreign['combobreak.wav']);
  assert.match(strFromU8(result['CREDITS.txt']), /DDK RPK/);
  assert.throws(() => selectHitSamples({ 'hitcircle.png': new Uint8Array(1) }), /no playable hitsounds/);
});

test('archive filters omit nested paths and unrelated files before sound extraction', () => {
  const pack = zipSync({ '../normal-hitnormal.wav': new Uint8Array(1), 'normal-hitnormal.wav': new Uint8Array(2), 'capture.png': new Uint8Array(4) });
  assert.deepEqual(Object.keys(unpackSkin(pack, true)), ['normal-hitnormal.wav']);
});

test('the download is an importable ZIP with matching skin identity', async () => {
  const files = assembleSkin(asset('hddt/base.zip'), asset('hddt/jumps.zip'), sounds.clicky, { ...defaultSkinChoice, theme: 'hddt', guide: 'jumps', cursor: 'diamond' }, id, {}, asset('hddt/cursor-diamond.zip'));
  const packed = unzipSync(await encodeSkin(files));
  const info = JSON.parse(strFromU8(packed['skininfo.json']));
  assert.equal(info.ID, id);
  assert.ok(strFromU8(packed['skin.ini']).includes('Name: ' + info.Name));
});

test('header-only muted WAV samples are silent rather than decoder errors', () => {
  const data = new Uint8Array(44); data.set(strToU8('RIFF'), 0); data.set(strToU8('WAVE'), 8); data.set(strToU8('fmt '), 12); new DataView(data.buffer).setUint32(16, 16, true); data.set(strToU8('data'), 36);
  assert.equal(isSilentWav(data), true);
  new DataView(data.buffer).setUint32(40, 4, true);
  assert.equal(isSilentWav(data), false);
  assert.equal(isSilentWav(new Uint8Array(4)), false);
});

test('header-only OGG layers skip playback without discarding audible or broken files', () => {
  const page = new Uint8Array(51);
  page.set(strToU8('OggS')); page[26] = 3; page.set([7, 7, 7], 27);
  for (let i = 0; i < 3; i++) { page[30 + i * 7] = 1 + i * 2; page.set(strToU8('vorbis'), 31 + i * 7); }
  assert.equal(isSilentAudio(page), true);
  assert.deepEqual(selectComboBreak({ 'combobreak.ogg': page }), {});
  assert.equal(isSilentAudio(page.subarray(0, 50)), false);
  const audible = new Uint8Array(80); audible.set(page); audible.set(strToU8('OggS'), 51); audible[77] = 1; audible[78] = 1;
  assert.equal(isSilentAudio(audible), false);
  assert.equal(isSilentAudio(new Uint8Array()), false);
});

test('cursor sizes export real scaled textures and reject invalid URL scales', () => {
  assert.equal(parseSkinChoice(new URLSearchParams('cursorSize=100')).cursorSize, '1');
  for (const scale of ['0.75', '1', '1.25', '1.5']) {
    const patch = asset(`flow/cursor-yellow${scale === '1' ? '' : '-' + scale}.zip`);
    const bytes = patch['cursor@2x.png'];
    assert.equal(new DataView(bytes.buffer, bytes.byteOffset).getUint32(16), Math.round(64 * Number(scale)));
    assert.equal(parseSkinChoice(new URLSearchParams('cursorSize=' + scale)).cursorSize, scale);
  }
});
test('selected combo break replaces every encoding and missing cues fall back', () => {
  const base = asset('flow/base.zip');
  const selected = { 'normal-hitnormal.wav': sounds.soft['normal-hitnormal.wav'], 'combobreak.ogg': strToU8('synthetic-break') };
  const result = assembleSkin(base, asset('flow/subtle.zip'), selected, defaultSkinChoice, id, {}, asset('flow/cursor-yellow.zip'));
  assert.equal(result['combobreak.wav'], undefined);
  assert.deepEqual(result['combobreak.ogg'], selected['combobreak.ogg']);
  assert.deepEqual(selectComboBreak({}), {});
  const fallback = assembleSkin(base, asset('flow/subtle.zip'), { 'normal-hitnormal.wav': selected['normal-hitnormal.wav'] }, defaultSkinChoice, id, {}, asset('flow/cursor-yellow.zip'));
  assert.deepEqual(fallback['combobreak.wav'], base['combobreak.wav']);
});

test('both clients use the complete modern spinner without forcing the old style', () => {
  for (const theme of skinThemes) for (const client of ['lazer', 'stable'] as const) {
    const result = assembleSkin(asset(`${theme.id}/base.zip`), asset(`${theme.id}/subtle.zip`), sounds.soft, { ...defaultSkinChoice, theme: theme.id, client }, id, asset(`${theme.id}/stable.zip`), asset(`${theme.id}/cursor-ring.zip`));
    for (const name of ['bottom','top','middle','middle2','glow','approachcircle','spin','clear','rpm']) for (const suffix of ['.png','@2x.png']) assert.ok(result[`spinner-${name}${suffix}`]);
    assert.equal(result['spinner-background.png'], undefined);
    assert.equal(result['spinner-background@2x.png'], undefined);
  }
});

test('all spinner choices survive export in both clients and invalid URLs use Orbit', () => {
  assert.equal(parseSkinChoice(new URLSearchParams('spinner=../unknown')).spinner, 'orbit');
  for (const theme of skinThemes) for (const spinner of ['orbit', 'split', 'halo'] as const) for (const client of ['lazer', 'stable'] as const) {
    const patch = asset(`${theme.id}/spinner-${spinner}.zip`);
    const result = assembleSkin(asset(`${theme.id}/base.zip`), asset(`${theme.id}/subtle.zip`), sounds.soft, { ...defaultSkinChoice, theme: theme.id, spinner, client }, id, asset(`${theme.id}/stable.zip`), asset(`${theme.id}/cursor-ring.zip`), patch);
    assert.deepEqual(result['spinner-top@2x.png'], patch['spinner-top@2x.png']);
    assert.deepEqual(result['spinner-glow@2x.png'], patch['spinner-glow@2x.png']);
    assert.equal(result['spinner-background.png'], undefined);
    assert.ok(strFromU8(result['skin.ini']).includes(spinner));
  }
});

test('slider ends are explicitly transparent while slider starts and reverse indicators remain', () => {
  const blank = new Uint8Array(readFileSync(new URL('../public/skin-builder/v1/transparent.png', import.meta.url)));
  for (const theme of skinThemes) for (const client of ['lazer', 'stable'] as const) {
    const base = asset(`${theme.id}/base.zip`);
    const result = assembleSkin(base, asset(`${theme.id}/subtle.zip`), sounds.soft, { ...defaultSkinChoice, theme: theme.id, client }, id, asset(`${theme.id}/stable.zip`), asset(`${theme.id}/cursor-ring.zip`));
    for (const name of ['sliderendcircle', 'sliderendcircleoverlay']) for (const suffix of ['.png', '@2x.png']) assert.deepEqual(result[name + suffix], blank);
    assert.deepEqual(result['reversearrow@2x.png'], base['reversearrow@2x.png']);
    assert.ok(result['hitcircle@2x.png'].length > blank.length);
  }
});

test('trail patches override cursor trails, with explicit Off and independent dot mode', () => {
 for (const client of ['lazer','stable'] as const) for (const trail of ['off','soft','dots','glow'] as const) {
  const patch = asset(`trails/yellow-${trail}-1.zip`);
  const result = assembleSkin(asset('flow/base.zip'), asset('flow/subtle.zip'), sounds.soft, { ...defaultSkinChoice, client, trail, cursor: 'yellow' }, id, asset('flow/stable.zip'), asset('flow/cursor-yellow.zip'), asset('flow/spinner-orbit.zip'), patch);
  assert.deepEqual(result['cursortrail.png'], patch['cursortrail.png']);
  assert.deepEqual(result['cursormiddle.png'], patch['cursormiddle.png']);
  if (trail === 'dots') assert.equal(result['cursormiddle@2x.png'], undefined);
 }
 assert.equal(parseSkinChoice(new URLSearchParams('trail=bad')).trail, 'soft');
});
test('compact numbers keep combo and leaderboard heights separate from score font', () => {
 for (const theme of skinThemes) {
  const base = asset(`${theme.id}/base.zip`);
  const height = (name: string) => new DataView(base[name].buffer, base[name].byteOffset).getUint32(20);
  assert.equal(height('aimmod-combo-0@2x.png'), 90);
  assert.equal(height('scoreentry-0@2x.png'), 38);
  assert.equal(height('aimmod-score-0@2x.png'), 80);
 }
});
