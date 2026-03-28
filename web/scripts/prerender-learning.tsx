import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToString } from "react-dom/server";
import { HelmetProvider, type FilledContext } from "react-helmet-async";
import { StaticRouter } from "react-router-dom/server";
import { App } from "../src/App";
import { serializePrerenderPayload, type PrerenderPayload } from "../src/lib/prerender";

type Drill = {
  label?: string;
  query?: string;
  reason?: string;
};

type FlawRef = {
  id?: string;
  title?: string;
  summary?: string;
  signalKeys?: string[];
  contextTags?: string[];
  telltales?: string[];
  contraindications?: string[];
  avoid?: string[];
};

type MechanicRef = {
  id?: string;
  title?: string;
  summary?: string;
  cues?: string[];
  failureModes?: string[];
  relatedSignalKeys?: string[];
};

type ScenarioRef = {
  id?: string;
  name?: string;
  aliases?: string[];
  scenarioTypes?: string[];
  summary?: string;
  whatItTrains?: string[];
  goodForFlaws?: string[];
  cautions?: string[];
};

type EvidenceRef = {
  sourceId?: string;
  claim?: string;
  excerpt?: string;
  startSec?: number;
  endSec?: number;
  confidence?: string;
  reviewStatus?: string;
};

type SourceRef = {
  id?: string;
  kind?: string;
  title?: string;
  author?: string;
  url?: string;
  publishedAtIso?: string;
};

type Entry = {
  id: string;
  title: string;
  summary: string;
  scenarioTypes?: string[];
  scenarioNames?: string[];
  signalKeys?: string[];
  contextTags?: string[];
  focusAreas?: string[];
  challengePreferences?: string[];
  timePreferences?: string[];
  why?: string[];
  actions?: string[];
  drills?: Drill[];
  avoid?: string[];
  priority?: string;
  flaw?: FlawRef | null;
  mechanics?: MechanicRef[];
  scenarios?: ScenarioRef[];
  evidence?: EvidenceRef[];
  sources?: SourceRef[];
};

type PublishedKnowledge = {
  version: string;
  updatedAtIso: string;
  entries: Entry[];
};

type LearnEntryPreview = {
  id: string;
  title: string;
  summary: string;
  priority: string;
  scenarioTypes: string[];
  contextTags: string[];
  signalKeys: string[];
  focusAreas: string[];
  sourceCount: number;
  drillCount: number;
};

type LearnIndexResponse = {
  version: string;
  updatedAtIso: string;
  entryCount: number;
  sourceCount: number;
  signalKeyCount: number;
  scenarioTypeCount: number;
  contextTagCount: number;
  featuredEntries: LearnEntryPreview[];
  entries: LearnEntryPreview[];
  topContextTags: string[];
  topScenarioTypes: string[];
};

type LearnEntryResponse = {
  version: string;
  updatedAtIso: string;
  entry: Entry;
  relatedEntries: LearnEntryPreview[];
};

type LearnTopicResponse = {
  version: string;
  updatedAtIso: string;
  topic: string;
  title: string;
  description: string;
  entryCount: number;
  featuredEntries: LearnEntryPreview[];
  entries: LearnEntryPreview[];
  relatedTopics: string[];
};

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const webDir = path.resolve(__dirname, "..");
const repoDir = path.resolve(webDir, "..");
const distDir = path.join(webDir, "dist");
const publishedKnowledgePath = path.join(repoDir, "api/internal/coaching/published/knowledge.v1.json");

function normalizeToken(value: string | undefined | null): string {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function humanizeToken(value: string): string {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function sharedCount(a: string[] = [], b: string[] = []): number {
  const set = new Set(b.map(normalizeToken).filter(Boolean));
  return a.reduce((count, value) => (set.has(normalizeToken(value)) ? count + 1 : count), 0);
}

function priorityRank(priority: string | undefined): number {
  switch ((priority ?? "").toLowerCase()) {
    case "high":
      return 3;
    case "medium":
      return 2;
    default:
      return 1;
  }
}

function previewForEntry(entry: Entry): LearnEntryPreview {
  return {
    id: entry.id,
    title: entry.title,
    summary: entry.summary,
    priority: entry.priority ?? "low",
    scenarioTypes: [...(entry.scenarioTypes ?? [])],
    contextTags: [...(entry.contextTags ?? [])],
    signalKeys: [...(entry.signalKeys ?? [])],
    focusAreas: [...(entry.focusAreas ?? [])],
    sourceCount: entry.sources?.length ?? 0,
    drillCount: entry.drills?.length ?? 0,
  };
}

function sortLearnPreviews(previews: LearnEntryPreview[]) {
  previews.sort((a, b) => {
    if (priorityRank(a.priority) !== priorityRank(b.priority)) {
      return priorityRank(b.priority) - priorityRank(a.priority);
    }
    if (a.sourceCount !== b.sourceCount) {
      return b.sourceCount - a.sourceCount;
    }
    if (a.drillCount !== b.drillCount) {
      return b.drillCount - a.drillCount;
    }
    return a.title.localeCompare(b.title);
  });
}

function topKeysByFrequency(freq: Map<string, number>, limit: number): string[] {
  return [...freq.entries()]
    .sort((a, b) => (a[1] !== b[1] ? b[1] - a[1] : a[0].localeCompare(b[0])))
    .slice(0, limit)
    .map(([key]) => key);
}

function overlapScore(a: Entry, b: Entry): number {
  return (
    sharedCount(a.signalKeys, b.signalKeys) * 3 +
    sharedCount(a.contextTags, b.contextTags) * 2 +
    sharedCount(a.scenarioTypes, b.scenarioTypes) * 2 +
    sharedCount(a.focusAreas, b.focusAreas)
  );
}

function entryMatchesTopic(entry: Entry, topic: string): boolean {
  return (
    sharedCount([topic], entry.contextTags) > 0 ||
    sharedCount([topic], entry.scenarioTypes) > 0 ||
    sharedCount([topic], entry.signalKeys) > 0 ||
    sharedCount([topic], entry.focusAreas) > 0
  );
}

function buildLearningIndex(knowledge: PublishedKnowledge): LearnIndexResponse {
  const previews = knowledge.entries.map(previewForEntry);
  const sourceIDs = new Set<string>();
  const signalKeys = new Set<string>();
  const scenarioTypes = new Set<string>();
  const contextTags = new Set<string>();
  const contextTagFreq = new Map<string, number>();
  const scenarioTypeFreq = new Map<string, number>();

  for (const entry of knowledge.entries) {
    for (const source of entry.sources ?? []) {
      if (source.id) sourceIDs.add(source.id);
    }
    for (const value of entry.signalKeys ?? []) {
      signalKeys.add(value);
    }
    for (const value of entry.scenarioTypes ?? []) {
      scenarioTypes.add(value);
      scenarioTypeFreq.set(value, (scenarioTypeFreq.get(value) ?? 0) + 1);
    }
    for (const value of entry.contextTags ?? []) {
      contextTags.add(value);
      contextTagFreq.set(value, (contextTagFreq.get(value) ?? 0) + 1);
    }
  }

  sortLearnPreviews(previews);

  return {
    version: knowledge.version,
    updatedAtIso: knowledge.updatedAtIso,
    entryCount: knowledge.entries.length,
    sourceCount: sourceIDs.size,
    signalKeyCount: signalKeys.size,
    scenarioTypeCount: scenarioTypes.size,
    contextTagCount: contextTags.size,
    featuredEntries: previews.slice(0, 8),
    entries: previews,
    topContextTags: topKeysByFrequency(contextTagFreq, 12),
    topScenarioTypes: topKeysByFrequency(scenarioTypeFreq, 8),
  };
}

function buildLearningEntry(knowledge: PublishedKnowledge, entryId: string): LearnEntryResponse {
  const current = knowledge.entries.find((entry) => normalizeToken(entry.id) === normalizeToken(entryId));
  if (!current) {
    throw new Error(`Missing learning entry: ${entryId}`);
  }

  const relatedEntries = knowledge.entries
    .filter((entry) => entry.id !== current.id)
    .map((entry) => ({ preview: previewForEntry(entry), score: overlapScore(current, entry) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => {
      if (a.score !== b.score) return b.score - a.score;
      if (priorityRank(a.preview.priority) !== priorityRank(b.preview.priority)) {
        return priorityRank(b.preview.priority) - priorityRank(a.preview.priority);
      }
      return a.preview.title.localeCompare(b.preview.title);
    })
    .slice(0, 6)
    .map((item) => item.preview);

  return {
    version: knowledge.version,
    updatedAtIso: knowledge.updatedAtIso,
    entry: current,
    relatedEntries,
  };
}

function buildLearningTopic(knowledge: PublishedKnowledge, topic: string): LearnTopicResponse {
  const normalizedTopic = normalizeToken(topic);
  const previews = knowledge.entries.filter((entry) => entryMatchesTopic(entry, normalizedTopic)).map(previewForEntry);
  if (!previews.length) {
    throw new Error(`Missing learning topic: ${topic}`);
  }

  sortLearnPreviews(previews);
  const relatedFreq = new Map<string, number>();
  for (const entry of knowledge.entries) {
    if (!entryMatchesTopic(entry, normalizedTopic)) continue;
    for (const value of [...(entry.contextTags ?? []), ...(entry.scenarioTypes ?? [])]) {
      const normalized = normalizeToken(value);
      if (!normalized || normalized === normalizedTopic) continue;
      relatedFreq.set(normalized, (relatedFreq.get(normalized) ?? 0) + 1);
    }
  }

  const title = humanizeToken(normalizedTopic);
  return {
    version: knowledge.version,
    updatedAtIso: knowledge.updatedAtIso,
    topic: normalizedTopic,
    title,
    description: `KB-backed aim training guides related to ${title.toLowerCase()}, generated from AimMod's coaching knowledge.`,
    entryCount: previews.length,
    featuredEntries: previews.slice(0, 6),
    entries: previews,
    relatedTopics: topKeysByFrequency(relatedFreq, 10),
  };
}

function routeOutputPath(routePath: string): string {
  const trimmed = routePath.replace(/^\/+/, "");
  return path.join(distDir, trimmed, "index.html");
}

function injectRenderedMarkup(baseHtml: string, appHtml: string, payload: PrerenderPayload, helmet: FilledContext["helmet"]) {
  const titleMarkup = helmet?.title?.toString() || "<title>AimMod Hub</title>";
  const metaMarkup = helmet?.meta?.toString() || "";
  const linkMarkup = helmet?.link?.toString() || "";
  const initialDataScript = `<script>window.__AIMMOD_HUB_PRERENDER__=${serializePrerenderPayload(payload)};globalThis.__AIMMOD_HUB_PRERENDER__=window.__AIMMOD_HUB_PRERENDER__;</script>`;

  let html = baseHtml.replace(/<title>[\s\S]*?<\/title>/i, titleMarkup);
  html = html.replace(/<meta name="description" content="[^"]*"\s*\/?>/i, "");
  html = html.replace("</head>", `${metaMarkup}${linkMarkup}</head>`);
  html = html.replace('<div id="root"></div>', `<div id="root">${appHtml}</div>\n    ${initialDataScript}`);
  return html;
}

async function renderRoute(baseHtml: string, routePath: string, payload: PrerenderPayload) {
  const helmetContext: { helmet?: FilledContext["helmet"] } = {};
  (globalThis as typeof globalThis & { __AIMMOD_HUB_PRERENDER__?: PrerenderPayload }).__AIMMOD_HUB_PRERENDER__ = payload;

  const appHtml = renderToString(
    <HelmetProvider context={helmetContext}>
      <App RouterComponent={StaticRouter} routerProps={{ location: routePath }} />
    </HelmetProvider>,
  );

  delete (globalThis as typeof globalThis & { __AIMMOD_HUB_PRERENDER__?: PrerenderPayload }).__AIMMOD_HUB_PRERENDER__;

  const outputPath = routeOutputPath(routePath);
  await fs.mkdir(path.dirname(outputPath), { recursive: true });
  await fs.writeFile(outputPath, injectRenderedMarkup(baseHtml, appHtml, payload, helmetContext.helmet), "utf8");
}

async function main() {
  const [baseHtml, rawKnowledge] = await Promise.all([
    fs.readFile(path.join(distDir, "index.html"), "utf8"),
    fs.readFile(publishedKnowledgePath, "utf8"),
  ]);
  const knowledge = JSON.parse(rawKnowledge) as PublishedKnowledge;
  const learningIndex = buildLearningIndex(knowledge);

  const routes: Array<{ path: string; payload: PrerenderPayload }> = [
    {
      path: "/learn",
      payload: { learningIndex },
    },
  ];

  for (const entry of knowledge.entries) {
    routes.push({
      path: `/learn/${entry.id}`,
      payload: {
        learningEntries: {
          [entry.id]: buildLearningEntry(knowledge, entry.id),
        },
      },
    });
  }

  const topics = new Set<string>([...learningIndex.topContextTags, ...learningIndex.topScenarioTypes]);
  for (const entry of knowledge.entries) {
    for (const topic of [...(entry.contextTags ?? []), ...(entry.scenarioTypes ?? [])]) {
      const normalized = normalizeToken(topic);
      if (normalized) topics.add(normalized);
    }
  }

  for (const topic of topics) {
    routes.push({
      path: `/learn/topics/${topic}`,
      payload: {
        learningTopics: {
          [topic]: buildLearningTopic(knowledge, topic),
        },
      },
    });
  }

  for (const route of routes) {
    await renderRoute(baseHtml, route.path, route.payload);
  }

  console.log(`Prerendered ${routes.length} learning routes.`);
}

void main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
