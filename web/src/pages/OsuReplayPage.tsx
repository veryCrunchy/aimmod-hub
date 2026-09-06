import { formatScorePp } from "../lib/scorePp";
import { useEffect, useMemo, useState } from "react";
import { PageSeo } from "../components/PageSeo";
import { useScorePp } from "../hooks/useScorePp";
import { ScorePpStatus } from "../components/ScorePpStatus";
import { OsuReplayPlayer } from "../components/OsuReplayPlayer";
import { replayHeaderScorePp } from "../lib/replayHeaderPp";
import { Link, useParams } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { formatOsuMissReason } from "../lib/osuReplayAnalysis";
import {
  fetchOsuReplay,
  fetchOsuOfficialScore,
  formatOsuAccuracy,
  formatOsuDuration,
  formatOsuMods,
  osuReplayDownloadUrl,
  osuOfficialReplayDownloadUrl,
  type OsuReplayJudgement,
  type OsuReplayAnalysis,
  type OsuSharedReplay,
} from "../lib/osuCommunity";

export function OsuReplayPage() {
  const { shareId = "", officialScoreId = "" } = useParams();
  const [rawReplay, setReplay] = useState<OsuSharedReplay | null>(null);
  const replayItems = useMemo(() => rawReplay ? [rawReplay] : [], [rawReplay]);
  const calculated = useScorePp(replayItems);
  const replay = calculated.items[0];
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [browserAnalysis, setBrowserAnalysis] = useState<OsuReplayAnalysis | null>(null);
  const [playbackError, setPlaybackError] = useState("");
  const [seekToMs, setSeekToMs] = useState<number>();

  useEffect(() => {
    let cancelled = false;
    setReplay(null);
    setError(null);
    setBrowserAnalysis(null); setPlaybackError(""); setSeekToMs(undefined);
    void (officialScoreId ? fetchOsuOfficialScore(officialScoreId) : fetchOsuReplay(shareId))
      .then((value) => { if (!cancelled) setReplay(value); })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : "Could not load this replay."); });
    return () => { cancelled = true; };
  }, [shareId, officialScoreId, attempt]);

  const analysis = replay?.analysis?.judgements?.length ? replay.analysis : browserAnalysis;
  const misses = useMemo(
    () => analysis?.judgements?.filter((item) => ["miss", "sliderbreak"].includes(item.result?.toLowerCase() ?? "")) ?? [],
    [analysis],
  );
  const missReasons = useMemo(() => {
    const counts = new Map<string, number>();
    for (const judgement of misses) {
      if (!judgement.missAnalysis?.reason) continue;
      const reason = formatOsuMissReason(judgement.missAnalysis?.reason);
      counts.set(reason, (counts.get(reason) ?? 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1]);
  }, [misses]);

  if (error) {
    return <PageStack><PageSeo title="Replay unavailable · AimMod Hub" description="This replay is unavailable." noindex /><PageSection><EmptyState title="Replay unavailable" body="The replay may no longer be shared, or the service may be temporarily unavailable."><Button onClick={() => setAttempt(value => value + 1)}>Try again</Button><Button to="/osu/replays">Browse replays</Button></EmptyState></PageSection></PageStack>;
  }
  if (!replay) {
    return <PageStack><PageSeo title="osu! replay · AimMod Hub" description="Loading shared replay." noindex /><PageSection role="status" aria-label="Loading replay"><p className="mb-3 text-muted">Loading replay...</p><Skeleton className="h-44" /></PageSection><PageSection><Skeleton className="h-80" /></PageSection></PageStack>;
  }

  const pp = replay.performancePoints == null ? (calculated.pending ? "Calculating..." : "Unavailable") : formatScorePp(replay.performancePoints);
  const sourceScoreId = officialScoreId || replay.officialScoreId;
  const playbackSource = replay.hasReplayFile && replay.shareId ? osuReplayDownloadUrl(replay.shareId)
    : sourceScoreId && replay.officialReplayExists ? osuOfficialReplayDownloadUrl(sourceScoreId) : undefined;
  const notable = misses
    .slice()
    .sort((a, b) => (b.missAnalysis?.confidence ?? 0) - (a.missAnalysis?.confidence ?? 0))
    .slice(0, 8);

  return (
    <PageStack>
      <PageSeo title={replay.visibility === "public" ? `${replay.artist} - ${replay.title} · osu! replay · AimMod Hub` : "osu! shared replay · AimMod Hub"}
        description={replay.visibility === "public" ? `${formatOsuAccuracy(replay.accuracy)} on ${replay.difficulty}, shared by ${replay.osuUsername}.` : "View an osu! replay shared by link."}
        noindex={replay.visibility !== "public"} />
      <PageSection className="relative overflow-hidden p-0">
        {replay.coverUrl ? <img src={replay.coverUrl} alt="" className="absolute inset-0 h-full w-full object-cover opacity-20" /> : null}
        <div className="relative grid gap-5 bg-black/60 px-5 py-6 md:grid-cols-[minmax(0,1fr)_auto] md:px-7 md:py-7">
          <div className="min-w-0">
            <span className="text-[10px] uppercase text-[#ff9bc7]">osu! replay analysis</span>
            <h1 className="mt-2 break-words text-2xl md:text-3xl leading-tight">{replay.artist} - {replay.title}</h1>
            <p className="mt-2 text-[13px] text-muted">[{replay.difficulty}] by {replay.creator}</p>
            <p className="mt-3 text-[12px] text-muted">
              Played by <Link className="text-cyan hover:text-text" to={`/osu/profiles/${replay.hubHandle || replay.osuUserId}`}>{replay.osuUsername}</Link>
              {replay.visibility === "unlisted" ? " · Unlisted" : " · Public"}
            </p>
          </div>
          <div className="flex items-end gap-2 md:items-start">
            {officialScoreId && <Button href={`https://osu.ppy.sh/scores/${officialScoreId}`} target="_blank" rel="noreferrer">View score on osu!</Button>}
            {playbackSource ? <Button href={playbackSource} variant="primary">Download replay</Button> : null}
            {replay.beatmapSetId > 0 ? <Button href={`https://osu.ppy.sh/beatmapsets/${replay.beatmapSetId}#osu/${replay.beatmapId}`} target="_blank" rel="noreferrer">Open beatmap</Button> : null}
          </div>
        </div>
      </PageSection>

      {playbackSource && replay.ruleset === "osu" && <OsuReplayPlayer replayUrl={playbackSource} beatmapId={replay.beatmapId} beatmapsetId={replay.beatmapSetId} backgroundUrl={replay.coverUrl || undefined} title={replay.title} seekToMs={seekToMs} onAnalysis={value => { setBrowserAnalysis(value); setPlaybackError(""); }} onPlaybackError={setPlaybackError}
        onVerifiedReplay={parsed => {
          if (replay.performancePoints != null || replay.ppCalculation) return;
          const result = replayHeaderScorePp(parsed.replay, { beatmapId: replay.beatmapId, objectCount: parsed.beatmap.hitObjects.length, passed: replay.passed });
          setReplay(current => {
            if (!current || current.shareId !== replay.shareId || current.beatmapId !== replay.beatmapId || current.performancePoints != null || current.ppCalculation) return current;
            return result.input ? { ...current, ppCalculation: result.input } : { ...current, ppCalculationError: result.reason };
          });
        }} />}
      {officialScoreId && !replay.officialReplayExists && <p className="text-sm text-muted">osu! does not list a replay file for this score.</p>}

      <PageSection className="grid grid-cols-[repeat(6,minmax(0,1fr))] gap-px overflow-hidden p-0 bg-line max-[840px]:grid-cols-3 max-[480px]:grid-cols-2">
        <Metric label="Accuracy" value={!replay.passed && replay.count300 + replay.count100 + replay.count50 + replay.countMiss === 0 ? "—" : formatOsuAccuracy(replay.accuracy)} accent="cyan" />
        <Metric label={replay.ppSource === "calculated" ? replay.passed ? "Calculated PP" : "Partial play PP" : "Performance"} value={pp} accent="pink" />
        <Metric label="Score" value={replay.totalScore.toLocaleString()} />
        <Metric label="Combo" value={`${replay.maxCombo}x`} />
        <Metric label={replay.calculatedStarRating == null && replay.source === "official" ? "Base difficulty" : "Difficulty"} value={`${(replay.calculatedStarRating ?? replay.starRating).toFixed(2)}★`} accent="gold" />
        <Metric label="Mods" value={formatOsuMods(replay.mods)} />
      </PageSection>

      {!replay.passed && <p className="text-sm text-muted">Failed or stopped early · {replay.count300 + replay.count100 + replay.count50 + replay.countMiss} objects judged{replay.mapObjectCount ? ` / ${replay.mapObjectCount}` : ""}. {replay.ppSource === "calculated" ? "PP is calculated for the portion played." : ""}</p>}
      <div className="grid grid-cols-[minmax(0,1.4fr)_minmax(280px,0.6fr)] gap-4 max-[900px]:grid-cols-1">
        <PageSection>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <span className="text-[10px] uppercase text-cyan">Notable moments</span>
              <h2 className="mt-1 text-[20px]">What happened during this play</h2>
            </div>
            <span className="text-[11px] text-muted">{replay.analysis?.judgements?.length ? replay.analysisEngine : browserAnalysis ? "Replay reconstruction" : ""}</span>
          </div>
          {notable.length === 0 ? (
            analysis ? <EmptyState title="No misses or slider breaks" body="No misses or slider breaks were found in this replay analysis." />
              : playbackError ? <EmptyState title="Replay analysis unavailable" body={playbackError} />
              : playbackSource && replay.ruleset === "osu" ? <div role="status" className="flex min-h-36 flex-col items-center justify-center gap-3"><strong>Preparing replay analysis</strong><progress aria-label="Preparing replay analysis" className="h-1 w-48" /></div>
              : <EmptyState title="Replay data unavailable" body="Judgement analysis requires the replay file. The score summary alone cannot identify when or why a miss happened." />
          ) : (
            <div className="mt-5 border-y border-line">
              {notable.map((judgement, index) => <JudgementRow key={`${judgement.objectIndex ?? index}-${judgement.startTimeMs ?? index}`} judgement={judgement} onSeek={playbackSource ? () => setSeekToMs(Math.max(0, (judgement.startTimeMs ?? 0) - 1500)) : undefined} />)}
            </div>
          )}
        </PageSection>

        <PageSection>
          <span className="text-[10px] uppercase text-cyan">Run summary</span>
          <h2 className="mt-1 text-[20px]">Judgements</h2>
          <div className="mt-5 grid grid-cols-2 gap-px overflow-hidden rounded-md bg-line">
            <JudgementMetric label="300" value={replay.count300} colour="text-[#66ccff]" />
            <JudgementMetric label="100" value={replay.count100} colour="text-[#88d498]" />
            <JudgementMetric label="50" value={replay.count50} colour="text-[#ffb347]" />
            <JudgementMetric label="Miss" value={replay.countMiss} colour="text-[#ff5d73]" />
          </div>
          <dl className="mt-5 divide-y divide-line border-y border-line text-[12px]">
            <Detail label="Length" value={formatOsuDuration(replay.lengthMs)} />
            <Detail label="BPM" value={Math.round(replay.bpm).toLocaleString()} />
            <Detail label="Played" value={new Date(replay.playedAt).toLocaleString()} />
          </dl>
          {missReasons.length > 0 ? (
            <div className="mt-5">
              <span className="text-[10px] uppercase text-muted-2">Miss pattern</span>
              <div className="mt-2 flex flex-wrap gap-2">
                {missReasons.map(([reason, count]) => <span key={reason} className="rounded-sm border border-line px-2.5 py-1 text-[11px] text-muted">{reason} <strong className="text-text">{count}</strong></span>)}
              </div>
            </div>
          ) : null}
        </PageSection>
      </div>
      <ScorePpStatus {...calculated} />
    </PageStack>
  );
}

function Metric({ label, value, accent }: { label: string; value: string; accent?: "cyan" | "pink" | "gold" }) {
  const colour = accent === "cyan" ? "text-cyan" : accent === "pink" ? "text-[#ff78b4]" : accent === "gold" ? "text-gold" : "text-text";
  return <div className="grid min-h-20 content-center gap-1 bg-panel px-4 py-3"><span className="text-[9px] uppercase text-muted-2">{label}</span><strong className={`truncate text-[18px] tabular-nums ${colour}`}>{value}</strong></div>;
}

function JudgementMetric({ label, value, colour }: { label: string; value: number; colour: string }) {
  return <div className="bg-panel px-3 py-3"><span className={`text-[18px] font-semibold tabular-nums ${colour}`}>{value}</span><span className="ml-2 text-[10px] text-muted">{label}</span></div>;
}

function JudgementRow({ judgement, onSeek }: { judgement: OsuReplayJudgement; onSeek?: () => void }) {
  const reason = judgement.missAnalysis?.reason ? formatOsuMissReason(judgement.missAnalysis.reason) : judgement.result === "SliderBreak" ? "Slider break" : "Miss";
  const timestamp = formatOsuDuration(judgement.startTimeMs ?? 0);
  const timing = !judgement.missAnalysis ? "Review cursor movement and tapping at this moment" : judgement.missAnalysis.pressTimeOffsetMs == null ? "No click registered" : `${Math.round(judgement.missAnalysis.pressTimeOffsetMs)}ms tap offset`;
  return (
    <div className="grid grid-cols-[56px_minmax(0,1fr)_auto] items-center gap-3 border-b border-line py-3 last:border-b-0 max-[560px]:grid-cols-[48px_minmax(0,1fr)]">
      {onSeek ? <button className="text-[11px] tabular-nums text-cyan" onClick={onSeek} aria-label={`Seek to ${timestamp}`}>{timestamp}</button> : <span className="text-[11px] tabular-nums text-muted">{timestamp}</span>}
      <span className="min-w-0"><strong className="block truncate text-[12px] text-text">{reason}</strong><span className="mt-0.5 block truncate text-[10px] text-muted">{timing}</span></span>
      {judgement.missAnalysis?.confidence != null && <span className="text-[10px] tabular-nums text-muted max-[560px]:hidden">{Math.round(judgement.missAnalysis.confidence * 100)}% confidence</span>}
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between gap-4 py-2.5"><dt className="text-muted">{label}</dt><dd className="m-0 tabular-nums text-text">{value}</dd></div>;
}
