import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchProfile, formatDurationMs, slugifyScenarioName } from "../lib/api";

export function ProfilePage() {
  const { handle = "" } = useParams();
  const [profile, setProfile] = useState<GetProfileResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setError(null);
    void fetchProfile(handle)
      .then((next) => {
        if (!cancelled) setProfile(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this profile.");
      });
    return () => {
      cancelled = true;
    };
  }, [handle]);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Profile" title="Could not load this profile" />
          <EmptyState title="Profile not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!profile) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Profile" title="Loading profile" />
        </PageSection>
      </PageStack>
    );
  }

  const primaryFocus =
    profile.primaryScenarioType && profile.primaryScenarioType !== "Unknown"
      ? profile.primaryScenarioType
      : "Mixed practice";
  const accuracyDetail =
    profile.primaryScenarioType && profile.primaryScenarioType !== "Unknown"
      ? profile.primaryScenarioType
      : "Across all scenarios";

  return (
    <PageStack>
      <PageSection className="grid grid-cols-[1.6fr_minmax(280px,0.9fr)] gap-[18px] max-[1100px]:grid-cols-1">
        <div>
          <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Profile</div>
          <h1>@{profile.userHandle}</h1>
          <p className="text-sm leading-7 text-muted">
            {profile.userDisplayName || profile.userHandle} has {profile.runCount.toLocaleString()} runs across {profile.scenarioCount.toLocaleString()} scenarios.
          </p>
        </div>
        <div className="grid content-start gap-2 rounded-[18px] border border-cyan/18 bg-[linear-gradient(180deg,rgba(57,208,255,0.08),rgba(182,151,255,0.06))] p-[18px]">
          <span className="text-[12px] uppercase tracking-[0.08em] text-cyan">Main focus</span>
          <strong className="text-[28px]">{primaryFocus}</strong>
          <em className="text-sm not-italic leading-7 text-muted">
            {primaryFocus === "Mixed practice"
              ? "This player spreads their time across multiple scenario styles."
              : "The scenario family this player spends the most time in."}
          </em>
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
        <StatCard label="Runs" value={profile.runCount.toLocaleString()} detail="Saved practice history" />
        <StatCard label="Scenario spread" value={profile.scenarioCount.toLocaleString()} detail="Different scenarios played" accent="cyan" />
        <StatCard label="Average score" value={Math.round(profile.averageScore).toLocaleString()} detail="Average result across all runs" accent="gold" />
        <StatCard label="Average accuracy" value={`${profile.averageAccuracy.toFixed(1)}%`} detail={accuracyDetail} accent="violet" />
      </Grid>

      <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Top scenarios"
            title="Where this player spends time"
            body="The scenarios this player returns to most often."
          />
          {profile.topScenarios.length > 0 ? (
            <ScrollArea className="max-h-[min(64vh,820px)] pr-2">
              <div className="grid gap-3">
              {profile.topScenarios.map((scenario) => (
                <Link
                  key={scenario.scenarioSlug}
                  to={`/scenarios/${scenario.scenarioSlug}`}
                  className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                >
                  <strong className="block text-text">{scenario.scenarioName}</strong>
                  <p className="mt-1 text-sm text-muted">{scenario.scenarioType || "Unknown"}</p>
                  <p className="mt-3 text-sm text-mint">{scenario.runCount.toLocaleString()} runs</p>
                </Link>
              ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No scenario history" body="This profile does not have any uploaded scenario history yet." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Recent runs"
            title="Latest runs"
            body="The most recent runs on this profile."
          />
          {profile.recentRuns.length > 0 ? (
            <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="border-b border-line text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Scenario</th>
                    <th className="px-4 py-3">Score</th>
                    <th className="px-4 py-3">Acc</th>
                    <th className="px-4 py-3">Duration</th>
                    <th className="px-4 py-3">Run</th>
                  </tr>
                </thead>
                <tbody>
                  {profile.recentRuns.map((run) => (
                    <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0">
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}>
                          {run.scenarioName}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-text">{formatDurationMs(run.durationMs)}</td>
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                          Open
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
          ) : (
            <EmptyState title="No recent runs" body="This profile does not have any uploaded runs yet." />
          )}
        </PageSection>
      </Grid>
    </PageStack>
  );
}
