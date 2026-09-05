import type { LearnEntryResponse, LearnIndexResponse, LearnTopicResponse } from "./api";
import { GetLearningEntryResponse, GetLearningIndexResponse, GetLearningTopicResponse } from "../gen/aimmod/hub/v1/hub_pb";

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
  const value = getGlobalPayload()?.learningIndex;
  return value ? GetLearningIndexResponse.fromJsonString(JSON.stringify(value), { ignoreUnknownFields: true }) : null;
}

export function getPrerenderLearningEntry(entryId: string): LearnEntryResponse | null {
  const value = getGlobalPayload()?.learningEntries?.[entryId];
  // Restore the same nested defaults supplied by live protobuf responses.
  return value ? GetLearningEntryResponse.fromJsonString(JSON.stringify(value), { ignoreUnknownFields: true }) : null;
}

export function getPrerenderLearningTopic(topic: string): LearnTopicResponse | null {
  const value = getGlobalPayload()?.learningTopics?.[topic];
  return value ? GetLearningTopicResponse.fromJsonString(JSON.stringify(value), { ignoreUnknownFields: true }) : null;
}

export function serializePrerenderPayload(payload: PrerenderPayload): string {
  return JSON.stringify(payload).replace(/</g, "\\u003c");
}
