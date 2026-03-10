import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useAuth } from "../lib/AuthContext";
import {
  clearAdminFailures,
  displayScenarioType,
  fetchAdminOverview,
  fetchAdminUserDetail,
  formatDurationMs,
  formatRelativeTime,
  runAdminReclassify,
  type AdminOverviewResponse,
  type AdminUserDetailResponse,
} from "../lib/api";

function formatCount(value: number) {
  return value.toLocaleString();
}

export function AdminPage() {
  const auth = useAuth();
  const [overview, setOverview] = useState<AdminOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionState, setActionState] = useState<string | null>(null);
  const [runningAction, setRunningAction] = useState(false);
  const [filter, setFilter] = useState("");
  const [selectedHandle, setSelectedHandle] = useState<string | null>(null);
  const [selectedUser, setSelectedUser] = useState<AdminUserDetailResponse | null>(null);
  const [selectedUserError, setSelectedUserError] = useState<string | null>(null);

  const normalizedFilter = filter.trim().toLowerCase();

  const load = useCallback(() => {
    if (!auth.isAdmin) {
      return;
    }
    void fetchAdminOverview()
      .then((next) => {
        setOverview(next);
        setError(null);
        if (!selectedHandle && next.userSyncHealth.length > 0) {
          setSelectedHandle(next.userSyncHealth[0]?.userHandle ?? null);
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Could not load admin overview.");
      });
  }, [auth.isAdmin]);

  useEffect(() => {
    load();
  }, [load]);

  useAutoRefresh(load, 30_000);

  useEffect(() => {
    if (!auth.isAdmin || !selectedHandle) {
      setSelectedUser(null);
      setSelectedUserError(null);
      return;
    }
    let cancelled = false;
    void fetchAdminUserDetail(selectedHandle)
      .then((detail) => {
        if (!cancelled) {
          setSelectedUser(detail);
          setSelectedUserError(null);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setSelectedUser(null);
          setSelectedUserError(err instanceof Error ? err.message : "Could not load user detail.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [auth.isAdmin, selectedHandle]);

  async function handleReclassify() {
    setRunningAction(true);
    try {
      const result = await runAdminReclassify();
      setActionState(`Scenario type repair finished. Updated ${formatCount(result.updated)} runs.`);
      load();
    } catch (err) {
      setActionState(err instanceof Error ? err.message : "Could not run scenario type repair.");
    } finally {
      setRunningAction(false);
    }
  }

  async function handleClearFailures() {
    setRunningAction(true);
    try {
      const result = await clearAdminFailures();
      setActionState(`Cleared ${formatCount(result.cleared)} stored ingest failures.`);
      load();
      if (selectedHandle) {
        const detail = await fetchAdminUserDetail(selectedHandle);
        setSelectedUser(detail);
        setSelectedUserError(null);
      }
    } catch (err) {
      setActionState(err instanceof Error ? err.message : "Could not clear ingest failures.");
    } finally {
      setRunningAction(false);
    }
  }

  const filteredUnknownScenarios = overview?.topUnknownScenarios.filter((scenario) =>
    normalizedFilter === "" || scenario.scenarioName.toLowerCase().includes(normalizedFilter)
  ) ?? [];

  const filteredUserSyncHealth = overview?.userSyncHealth.filter((user) =>
    normalizedFilter === "" ||
    user.userHandle.toLowerCase().includes(normalizedFilter) ||
    user.userDisplayName.toLowerCase().includes(normalizedFilter)
  ) ?? [];

  const filteredRecentFailures = overview?.recentFailures.filter((failure) =>
    normalizedFilter === "" ||
    failure.scenarioName.toLowerCase().includes(normalizedFilter) ||
    failure.userExternalId.toLowerCase().includes(normalizedFilter) ||
    failure.sessionId.toLowerCase().includes(normalizedFilter) ||
    failure.errorMessage.toLowerCase().includes(normalizedFilter)
  ) ?? [];

  const filteredRecentIngests = overview?.recentIngests.filter((run) =>
    normalizedFilter === "" ||
    run.scenarioName.toLowerCase().includes(normalizedFilter) ||
    run.userHandle.toLowerCase().includes(normalizedFilter) ||
    run.userDisplayName.toLowerCase().includes(normalizedFilter) ||
    run.publicRunId.toLowerCase().includes(normalizedFilter)
  ) ?? [];

  if (auth.loading) {
    return (
      <PageSection>
        <SectionHeader eyebrow="Admin" title="Loading admin access" body="Checking your linked account." />
      </PageSection>
    );
  }

  if (!auth.authenticated) {
    return (
      <PageSection>
        <EmptyState
          title="Sign in to open the admin dashboard"
          body="This page is only available to the configured AimMod admin account."
        />
      </PageSection>
    );
  }

  if (!auth.isAdmin) {
    return (
      <PageSection>
        <EmptyState
          title="This page is locked"
          body="Your account is linked, but it is not the configured admin account for this hub."
        />
      </PageSection>
    );
  }

  return (
    <PageStack>
      <PageSection className="border-mint/18 bg-[radial-gradient(circle_at_top_left,rgba(121,201,151,0.18),transparent_22%),linear-gradient(135deg,rgba(9,25,18,0.98),rgba(4,12,9,0.98))]">
        <SectionHeader
          eyebrow="Admin"
          title="AimMod hub health"
          body="A compact view of ingest quality, missing data, and the latest runs coming in."
          aside={
            <div className="flex flex-wrap items-center gap-2">
              {actionState ? <span>{actionState}</span> : null}
              <input
                value={filter}
                onChange={(event) => setFilter(event.target.value)}
                placeholder="Filter admin data"
                className="min-w-[220px] rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-4 py-2 text-sm text-text outline-none placeholder:text-muted focus:border-mint/70"
              />
              <Button onClick={() => void handleReclassify()} disabled={runningAction}>
                {runningAction ? "Repairing..." : "Repair scenario types"}
              </Button>
              <Button onClick={() => void handleClearFailures()} disabled={runningAction}>
                {runningAction ? "Working..." : "Clear failure log"}
              </Button>
            </div>
          }
        />
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(190px,1fr))]">
        <StatCard label="Runs" value={overview ? formatCount(overview.totalRuns) : "—"} detail="Stored on the hub" />
        <StatCard label="Players" value={overview ? formatCount(overview.totalPlayers) : "—"} detail="Linked contributors" accent="cyan" />
        <StatCard label="Scenarios" value={overview ? formatCount(overview.totalScenarios) : "—"} detail="Distinct scenario pages" accent="gold" />
        <StatCard label="Unknown type" value={overview ? formatCount(overview.unknownTypeRuns) : "—"} detail="Runs still missing a family" accent="violet" />
        <StatCard label="Missing timeline" value={overview ? formatCount(overview.missingTimelineRuns) : "—"} detail="Runs without per-second detail" />
        <StatCard label="Missing context" value={overview ? formatCount(overview.missingContextRuns) : "—"} detail="Runs without saved focus windows" />
        <StatCard label="Missing features" value={overview ? formatCount(overview.missingFeatureRuns) : "—"} detail="Runs without derived feature sets" />
        <StatCard label="Zero score" value={overview ? formatCount(overview.zeroScoreRuns) : "—"} detail="Runs saved with no score" />
      </Grid>

      {error ? (
        <PageSection>
          <EmptyState title="Could not load the admin dashboard" body={error} />
        </PageSection>
      ) : null}

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Unknown scenario family"
            title="What still needs classification"
            body="The scenarios creating the most unknown-family runs right now."
          />
          {filteredUnknownScenarios.length ? (
            <ScrollArea className="max-h-[420px] pr-2">
              <div className="grid gap-3">
                {filteredUnknownScenarios.map((scenario) => (
                  <Link
                    key={scenario.scenarioSlug}
                    to={`/scenarios/${scenario.scenarioSlug}`}
                    className="rounded-[18px] border border-line bg-white/2 px-4 py-3 transition-colors hover:border-mint/30 hover:bg-white/3"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <strong className="text-text">{scenario.scenarioName}</strong>
                      <span className="text-sm text-mint">{formatCount(scenario.runCount)}</span>
                    </div>
                  </Link>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No unknown families right now" body="Scenario family coverage looks healthy." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Source ids"
            title="Runs that may be hard to repair later"
            body="These counts show where source ids or summary layers are still missing."
          />
          <div className="grid gap-3">
            <div className="rounded-[18px] border border-line bg-white/2 px-4 py-3">
              <div className="text-[11px] uppercase tracking-[0.08em] text-muted">Missing source session id</div>
              <div className="mt-2 text-2xl text-text">{overview ? formatCount(overview.missingSourceSessionRuns) : "—"}</div>
            </div>
            <div className="rounded-[18px] border border-line bg-white/2 px-4 py-3">
              <div className="text-[11px] uppercase tracking-[0.08em] text-muted">Missing summary layer</div>
              <div className="mt-2 text-2xl text-text">{overview ? formatCount(overview.missingSummaryRuns) : "—"}</div>
            </div>
          </div>
        </PageSection>
      </Grid>

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader eyebrow="App versions" title="What versions are still sending runs" body="Useful when a bug only affects a specific app build." />
          {overview?.appVersions.length ? (
            <ScrollArea className="max-h-[360px] pr-2">
              <div className="grid gap-2">
                {overview.appVersions.map((version) => (
                  <div key={version.label} className="flex items-center justify-between rounded-[16px] border border-line bg-white/2 px-4 py-3">
                    <span className="text-text">{version.label || "Unknown"}</span>
                    <span className="text-sm text-mint">{formatCount(version.runCount)}</span>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No version data yet" body="App version counts will appear here once runs are stored." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader eyebrow="Schema versions" title="What upload formats are still active" body="Useful when a bug is tied to an older upload schema." />
          {overview?.schemaVersions.length ? (
            <ScrollArea className="max-h-[360px] pr-2">
              <div className="grid gap-2">
                {overview.schemaVersions.map((version) => (
                  <div key={version.label} className="flex items-center justify-between rounded-[16px] border border-line bg-white/2 px-4 py-3">
                    <span className="text-text">v{version.label || "Unknown"}</span>
                    <span className="text-sm text-cyan">{formatCount(version.runCount)}</span>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No schema data yet" body="Schema version counts will appear here once runs are stored." />
          )}
        </PageSection>
      </Grid>

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Player sync health"
            title="Whose data still needs attention"
            body="The players with the most unknown families, missing timelines, or other repair signals."
          />
          {filteredUserSyncHealth.length ? (
            <ScrollArea className="max-h-[480px] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Player</th>
                    <th className="px-4 py-3">Runs</th>
                    <th className="px-4 py-3">Unknown</th>
                    <th className="px-4 py-3">No timeline</th>
                    <th className="px-4 py-3">No context</th>
                    <th className="px-4 py-3">Zero score</th>
                    <th className="px-4 py-3">Played</th>
                    <th className="px-4 py-3">Stored</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredUserSyncHealth.map((user) => (
                    <tr
                      key={user.userHandle}
                      className="border-b border-white/6 last:border-b-0 cursor-pointer hover:bg-white/[0.02]"
                      onClick={() => setSelectedHandle(user.userHandle)}
                    >
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/profiles/${user.userHandle}`}>
                          {user.userDisplayName || user.userHandle}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text">{formatCount(user.runCount)}</td>
                      <td className="px-4 py-3 text-text">{formatCount(user.unknownTypeRuns)}</td>
                      <td className="px-4 py-3 text-text">{formatCount(user.missingTimelineRuns)}</td>
                      <td className="px-4 py-3 text-text">{formatCount(user.missingContextRuns)}</td>
                      <td className="px-4 py-3 text-text">{formatCount(user.zeroScoreRuns)}</td>
                      <td className="px-4 py-3 text-muted">{formatRelativeTime(user.lastPlayedAt)}</td>
                      <td className="px-4 py-3 text-muted">{formatRelativeTime(user.lastIngestedAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
          ) : (
            <EmptyState title="No player sync issues yet" body="Player-level sync health will appear here once runs are stored." />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Selected player"
            title={selectedUser ? (selectedUser.userDisplayName || selectedUser.userHandle) : "Pick a player"}
            body={selectedUser ? "Use this to inspect one player's sync quality, failures, and latest runs." : "Click a player in sync health to inspect their recent runs and ingest problems."}
          />
          {selectedUserError ? (
            <EmptyState title="Could not load player detail" body={selectedUserError} />
          ) : selectedUser ? (
            <div className="grid gap-4">
              <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
                <StatCard label="Runs" value={formatCount(selectedUser.runCount)} detail="Stored for this player" />
                <StatCard label="Scenarios" value={formatCount(selectedUser.scenarioCount)} detail="Distinct scenario pages" accent="cyan" />
                <StatCard label="Unknown" value={formatCount(selectedUser.unknownTypeRuns)} detail="Still missing a family" accent="violet" />
                <StatCard label="Zero score" value={formatCount(selectedUser.zeroScoreRuns)} detail="Runs saved with 0 score" accent="gold" />
              </Grid>

              <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
                <div className="rounded-[18px] border border-line bg-white/2 p-4">
                  <div className="mb-3 text-[12px] uppercase tracking-[0.1em] text-cyan">Unknown scenarios</div>
                  {selectedUser.topUnknownScenarios.length ? (
                    <div className="grid gap-2">
                      {selectedUser.topUnknownScenarios.map((scenario) => (
                        <Link key={scenario.scenarioSlug} to={`/scenarios/${scenario.scenarioSlug}`} className="flex items-center justify-between rounded-[14px] border border-line bg-white/2 px-3 py-2 hover:border-mint/30">
                          <span className="text-text">{scenario.scenarioName}</span>
                          <span className="text-sm text-mint">{formatCount(scenario.runCount)}</span>
                        </Link>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-muted">No unknown-family scenarios for this player.</div>
                  )}
                </div>

                <div className="rounded-[18px] border border-line bg-white/2 p-4">
                  <div className="mb-3 text-[12px] uppercase tracking-[0.1em] text-cyan">Recent failures</div>
                  {selectedUser.recentFailures.length ? (
                    <ScrollArea className="max-h-[260px] pr-2">
                      <div className="grid gap-2">
                        {selectedUser.recentFailures.map((failure) => (
                          <div key={failure.id} className="rounded-[14px] border border-line bg-white/2 px-3 py-2">
                            <div className="text-sm text-text">{failure.scenarioName || "Unknown scenario"}</div>
                            <div className="mt-1 text-xs text-[#f1b7b7]">{failure.errorMessage}</div>
                          </div>
                        ))}
                      </div>
                    </ScrollArea>
                  ) : (
                    <div className="text-sm text-muted">No recent failures stored for this player.</div>
                  )}
                </div>
              </Grid>

              <div className="rounded-[18px] border border-line bg-white/2 p-4">
                <div className="mb-3 text-[12px] uppercase tracking-[0.1em] text-cyan">Recent runs</div>
                {selectedUser.recentRuns.length ? (
                  <ScrollArea className="max-h-[320px] overflow-auto rounded-[14px] border border-line bg-[rgba(4,12,9,0.5)]">
                    <table className="min-w-full text-left text-sm">
                      <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                        <tr>
                          <th className="px-4 py-3">Scenario</th>
                          <th className="px-4 py-3">Family</th>
                          <th className="px-4 py-3">Score</th>
                          <th className="px-4 py-3">Acc</th>
                          <th className="px-4 py-3">Duration</th>
                          <th className="px-4 py-3">When</th>
                        </tr>
                      </thead>
                      <tbody>
                        {selectedUser.recentRuns.map((run) => (
                          <tr key={run.publicRunId} className="border-b border-white/6 last:border-b-0">
                            <td className="px-4 py-3 text-text">
                              <Link className="hover:text-cyan transition-colors" to={`/runs/${run.publicRunId}`}>{run.scenarioName}</Link>
                            </td>
                            <td className="px-4 py-3 text-muted">{displayScenarioType(run.scenarioType) ?? "Unknown"}</td>
                            <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                            <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                            <td className="px-4 py-3 text-text">{formatDurationMs(run.durationMs)}</td>
                            <td className="px-4 py-3 text-muted">{formatRelativeTime(run.playedAt)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </ScrollArea>
                ) : (
                  <div className="text-sm text-muted">No recent runs stored for this player yet.</div>
                )}
              </div>
            </div>
          ) : (
            <EmptyState title="No player selected" body="Pick a player from the sync health table to inspect them here." />
          )}
        </PageSection>
      </Grid>

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Recent ingest failures"
            title="What the hub recently rejected"
            body="Useful for spotting broken uploads, schema drift, or bad local data before users report it."
          />
          {filteredRecentFailures.length ? (
            <ScrollArea className="max-h-[480px] pr-2">
              <div className="grid gap-3">
                {filteredRecentFailures.map((failure) => (
                  <div key={failure.id} className="rounded-[18px] border border-line bg-white/2 px-4 py-3">
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0">
                        <div className="text-text">{failure.scenarioName || "Unknown scenario"}</div>
                        <div className="mt-1 text-xs text-muted">
                          {failure.userExternalId || "unknown user"} · {failure.sessionId || "missing session id"}
                        </div>
                      </div>
                      <div className="shrink-0 text-xs text-muted">{formatRelativeTime(failure.createdAt)}</div>
                    </div>
                    <div className="mt-3 text-sm leading-6 text-[#f1b7b7]">{failure.errorMessage}</div>
                  </div>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No recent failures" body="The hub has not rejected any uploads recently." />
          )}
        </PageSection>

        <PageSection>
        <SectionHeader
          eyebrow="Latest ingests"
          title="What just landed on the hub"
          body="Recent ingests ordered by when they were stored, with played time shown beside them."
        />
        {filteredRecentIngests.length ? (
          <ScrollArea className="max-h-[620px] overflow-auto rounded-[18px] border border-line bg-white/2">
            <table className="min-w-full text-left text-sm">
              <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                <tr>
                  <th className="px-4 py-3">Scenario</th>
                  <th className="px-4 py-3">Family</th>
                  <th className="px-4 py-3">Player</th>
                  <th className="px-4 py-3">Score</th>
                  <th className="px-4 py-3">Played</th>
                  <th className="px-4 py-3">Stored</th>
                  <th className="px-4 py-3">Run</th>
                </tr>
              </thead>
              <tbody>
                {filteredRecentIngests.map((run) => (
                  <tr key={run.publicRunId} className="border-b border-white/6 last:border-b-0">
                    <td className="px-4 py-3 text-text">
                      <Link className="hover:text-cyan transition-colors" to={`/scenarios/${run.scenarioSlug}`}>{run.scenarioName}</Link>
                    </td>
                    <td className="px-4 py-3 text-muted">{displayScenarioType(run.scenarioType) ?? "Unknown"}</td>
                    <td className="px-4 py-3 text-text">
                      <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle}`}>{run.userDisplayName || run.userHandle}</Link>
                    </td>
                    <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                    <td className="px-4 py-3 text-muted">{formatRelativeTime(run.playedAt)}</td>
                    <td className="px-4 py-3 text-muted">{formatRelativeTime(run.ingestedAt)}</td>
                    <td className="px-4 py-3 text-text">
                      <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.publicRunId}`}>Open</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </ScrollArea>
        ) : (
          <EmptyState title="No ingests yet" body="Recent ingests will appear here once the hub receives runs." />
        )}
        </PageSection>
      </Grid>
    </PageStack>
  );
}
