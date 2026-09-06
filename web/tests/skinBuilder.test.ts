import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { strFromU8, strToU8, zipSync, unzipSync } from 'fflate';
import { assembleSkin, defaultSkinChoice, encodeSkin, isSilentWav, parseSkinChoice, selectHitSamples, skinCursors, skinGuides, skinThemes, unpackSkin } from '../src/lib/skinBuilder';

const asset = (path: string) => unpackSkin(new Uint8Array(readFileSync(new URL('../public/skin-builder/v1/' + path, import.meta.url))));
const sounds = { soft: asset('soft.zip'), clicky: asset('clicky.zip') };
const id = '00000000-0000-4000-8000-000000000001';

test('URL choices are allowlisted and default to the subtle lazer pack', () => {
  assert.deepEqual(parseSkinChoice(new URLSearchParams('theme=../../other&sound=unknown&client=unknown&cursor=unknown')), defaultSkinChoice);
  assert.deepEqual(parseSkinChoice(new URLSearchParams('theme=hddt&sound=rafis&guide=jumps&client=stable&cursor=dot')), { theme: 'hddt', sound: 'rafis', guide: 'jumps', client: 'stable', cursor: 'dot' });
});

test('every built-in combination retains its chosen cursor, guides, audio and correct client layout', () => {
  for (const theme of skinThemes) {
    const base = asset(`${theme.id}/base.zip`), stable = asset(`${theme.id}/stable.zip`);
    for (const guide of skinGuides) {
      const guides = asset(`${theme.id}/${guide.id}.zip`);
      for (const cursor of skinCursors) {
        const cursors = asset(`${theme.id}/cursor-${cursor.id}.zip`);
        for (const sound of ['soft', 'clicky'] as const) for (const client of ['lazer', 'stable'] as const) {
          const result = assembleSkin(base, guides, sounds[sound], { theme: theme.id, guide: guide.id, cursor: cursor.id, sound, client }, id, stable, cursors);
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
  for (const name of Object.keys(catalog)) assert.ok(result[name], name);
});

test('community samples replace competing encodings but never import community artwork', () => {
  const base = asset('flow/base.zip');
  const foreign = { 'normal-hitnormal.ogg': strToU8('audio-fixture'), 'hitcircle.png': strToU8('not-our-art'), 'skin.ini': strToU8('Name: foreign'), 'combobreak.wav': strToU8('foreign-break') };
  const result = assembleSkin(base, asset('flow/arrows.zip'), foreign, { ...defaultSkinChoice, sound: 'rafis' }, id, {}, asset('flow/cursor-dot.zip'));
  assert.ok(!result['normal-hitnormal.wav']);
  assert.deepEqual(result['normal-hitnormal.ogg'], foreign['normal-hitnormal.ogg']);
  assert.deepEqual(result['hitcircle.png'], base['hitcircle.png']);
  assert.deepEqual(result['combobreak.wav'], base['combobreak.wav']);
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
