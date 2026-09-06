import test from 'node:test';
import assert from 'node:assert/strict';
import { defaultSkinChoice } from '../src/lib/skinBuilder';
import { readSkinChoice, resolveSkinChoice, saveSkinChoice } from '../src/lib/skinChoiceStorage';
test('all skin choices round trip and restore on a bare builder URL', () => {
 let raw: string | null = null;
 const storage = { getItem: () => raw, setItem: (_key: string, value: string) => { raw = value; } };
 const selected = { ...defaultSkinChoice, theme: 'glacier', cursor: 'yellow-glow', cursorSize: '1.5', trail: 'dots', guide: 'line', spinner: 'halo', sound: 'yugen', client: 'stable' } as const;
 saveSkinChoice(selected, storage);
 assert.deepEqual(resolveSkinChoice(new URLSearchParams('utm_source=test'), readSkinChoice(storage)), selected);
 assert.deepEqual(resolveSkinChoice(new URLSearchParams('theme=hddt'), selected), { ...defaultSkinChoice, theme: 'hddt' });
});
test('malformed, stale and inaccessible storage never prevents using the builder', () => {
 for (const raw of ['{', 'null', '[]', '"bad"']) assert.deepEqual(readSkinChoice({ getItem: () => raw, setItem: () => {} }), defaultSkinChoice);
 const stale = readSkinChoice({ getItem: () => '{"theme":"glacier","cursor":"unknown","trail":42}', setItem: () => {} });
 assert.deepEqual(stale, { ...defaultSkinChoice, theme: 'glacier' });
 const blocked = { getItem: (): string => { throw Error('blocked'); }, setItem: () => { throw Error('blocked'); } };
 assert.deepEqual(readSkinChoice(blocked), defaultSkinChoice);
 assert.doesNotThrow(() => saveSkinChoice(defaultSkinChoice, blocked));
});
