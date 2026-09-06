import { defaultSkinChoice, parseSkinChoice, type SkinChoice } from './skinBuilder';

export const SKIN_CHOICE_STORAGE_KEY = 'aimmod.skin-builder.choices.v1';
const keys = Object.keys(defaultSkinChoice) as (keyof SkinChoice)[];
type ChoiceStorage = Pick<Storage, 'getItem' | 'setItem'>;

export function readSkinChoice(storage?: ChoiceStorage): SkinChoice {
  try {
    const value: unknown = JSON.parse(storage?.getItem(SKIN_CHOICE_STORAGE_KEY) ?? 'null');
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { ...defaultSkinChoice };
    const params = new URLSearchParams();
    for (const key of keys) {
      const entry = (value as Record<string, unknown>)[key];
      if (typeof entry === 'string') params.set(key, entry);
    }
    return parseSkinChoice(params);
  } catch { return { ...defaultSkinChoice }; }
}

export function resolveSkinChoice(params: URLSearchParams, saved: SkinChoice): SkinChoice {
  // Shared configurations stay deterministic even when the recipient has saved preferences.
  return keys.some(key => params.has(key)) ? parseSkinChoice(params) : saved;
}

export function saveSkinChoice(choice: SkinChoice, storage?: ChoiceStorage): void {
  try { storage?.setItem(SKIN_CHOICE_STORAGE_KEY, JSON.stringify(parseSkinChoice(new URLSearchParams(choice)))); }
  catch { /* Private browsing, disabled storage or quota limits must not block the builder. */ }
}

export function skinChoiceStorage(): ChoiceStorage | undefined {
  try { return typeof window === 'undefined' ? undefined : window.localStorage; }
  catch { return undefined; }
}
