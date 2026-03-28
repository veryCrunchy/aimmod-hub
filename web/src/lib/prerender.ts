import type { LearnEntryResponse, LearnIndexResponse, LearnTopicResponse } from "./api";

export type PrerenderPayload = {
  learningIndex?: LearnIndexResponse | null;
  learningEntries?: Record<string, LearnEntryResponse>;
  learningTopics?: Record<string, LearnTopicResponse>;
};

type RuntimeWindow = Window & {
  __AIMMOD_HUB_PRERENDER__?: PrerenderPayload;
};

type RuntimeGlobal = typeof globalThis & {
  __AIMMOD_HUB_PRERENDER__?: PrerenderPayload;
};

function getGlobalPayload(): PrerenderPayload | null {
  return (globalThis as RuntimeGlobal).__AIMMOD_HUB_PRERENDER__ ?? null;
}

export function getPrerenderPayload(): PrerenderPayload | null {
  return getGlobalPayload();
}

export function getPrerenderLearningIndex(): LearnIndexResponse | null {
  return getGlobalPayload()?.learningIndex ?? null;
}

export function getPrerenderLearningEntry(entryId: string): LearnEntryResponse | null {
  return getGlobalPayload()?.learningEntries?.[entryId] ?? null;
}

export function getPrerenderLearningTopic(topic: string): LearnTopicResponse | null {
  return getGlobalPayload()?.learningTopics?.[topic] ?? null;
}

export function serializePrerenderPayload(payload: PrerenderPayload): string {
  return JSON.stringify(payload).replace(/</g, "\\u003c");
}
