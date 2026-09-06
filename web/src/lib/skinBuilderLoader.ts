import { API_BASE_URL } from './config';
import { SKIN_ASSETS, unpackSkin, type SkinFiles, type SkinSound } from './skinBuilder';

export async function loadBuilderArchive(url: string, signal: AbortSignal, soundsOnly = false): Promise<SkinFiles> {
  const response = await fetch(url, { signal: AbortSignal.any([signal, AbortSignal.timeout(65000)]), credentials: 'same-origin' });
  if (!response.ok || !response.body) throw new Error('This skin option could not be loaded. Try again or choose another.');
  const reader = response.body.getReader();
  const parts: Uint8Array[] = []; let size = 0;
  try {
    for (;;) {
      const next = await reader.read(); if (next.done) break;
      size += next.value.length;
      if (size > 32 * 1024 * 1024) throw new Error('This sound pack is too large. Choose another.');
      parts.push(next.value);
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
  signal.throwIfAborted();
  const bytes = new Uint8Array(size); let offset = 0;
  for (const part of parts) { bytes.set(part, offset); offset += part.length; }
  return unpackSkin(bytes, soundsOnly);
}

export function soundArchiveURL(sound: SkinSound): string {
  return sound === 'soft' || sound === 'clicky' ? `${SKIN_ASSETS}/${sound}.zip` : `${API_BASE_URL}/api/osu/v1/playback/skins/${sound}`;
}
