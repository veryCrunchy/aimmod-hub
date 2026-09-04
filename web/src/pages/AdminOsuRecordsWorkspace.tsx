import { useEffect, useRef, useState, type ReactNode } from "react";
import { adminBytes, adminScoreParams, fetchOsuAdmin, type OsuAdminBeatmap, type OsuAdminOverview, type OsuAdminPlayer, type OsuAdminProvider, type OsuAdminRecords, type OsuAdminScope, type OsuAdminShare } from "./adminOsu";

type View = "players" | "beatmaps" | "scores" | "services";
const views: { id: View; label: string; detail: string }[] = [
  { id: "players", label: "Players & accounts", detail: "Identity and connections" },
  { id: "beatmaps", label: "Beatmaps", detail: "Stored difficulties" },
  { id: "scores", label: "Scores & replays", detail: "Visibility and uploads" },
  { id: "services", label: "Services", detail: "Provider availability" },
];
const count = (n: number) => n.toLocaleString();

function useRecords<T>(path: string, revision: number) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(""); setData(null);
    void fetchOsuAdmin<T>(path, controller.signal)
      .then(next => { if (!controller.signal.aborted) setData(next); })
      .catch(err => { if (!controller.signal.aborted) setError(err instanceof Error ? err.message : "Could not load records."); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [path, revision]);
  return { data, error, loading };
}

function DateValue({ value }: { value: string | null }) {
  return value ? <time dateTime={value} title={new Date(value).toLocaleString()}>{new Date(value).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" })}</time> : <span className="admin-note">Not recorded</span>;
}
function Fact({ label, children }: { label: string; children: ReactNode }) { return <div><dt>{label}</dt><dd>{children}</dd></div>; }
function Status({ value }: { value: string }) { return <span className={`admin-status is-${value}`}>{value === "none" ? "No replay" : value === "pending" ? "Metadata only" : value}</span>; }
function Pager({ offset, total, length, loading, change }: { offset: number; total: number; length: number; loading: boolean; change: (n: number) => void }) {
  return <nav className="admin-pagination" aria-label="Record pages"><span aria-live="polite">{loading ? "Loading records..." : total === 0 ? "0 records" : length ? `${offset + 1}\u2013${offset + length} of ${count(total)}` : "No records on this page"}</span><button type="button" aria-label="Previous page" disabled={loading || offset === 0} onClick={() => change(Math.max(0, offset - 25))}>Previous</button><button type="button" aria-label="Next page" disabled={loading || offset + 25 >= total} onClick={() => change(offset + 25)}>Next</button></nav>;
}
function RecordTable({ label, headers, loading, empty, children }: { label: string; headers: string[]; loading: boolean; empty: boolean; children: ReactNode }) {
  return <div className="admin-table-scroll" role="region" aria-label={label} aria-busy={loading} tabIndex={0}><table className="admin-table"><thead><tr>{headers.map(h => <th key={h} scope="col">{h}</th>)}</tr></thead><tbody>{children}{loading || empty ? <tr><td colSpan={headers.length} className="admin-table-empty">{loading ? "Loading records..." : "No records match these filters."}</td></tr> : null}</tbody></table></div>;
}
function Inspector({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (subtitle && window.matchMedia("(max-width: 1250px)").matches) heading.current?.focus();
  }, [title, subtitle]);
  return <aside className="admin-inspector" aria-label="Record detail"><header><span className="admin-eyebrow">Record detail</span><h3 ref={heading} tabIndex={-1}>{title}</h3>{subtitle ? <p>{subtitle}</p> : null}</header>{children}</aside>;
}
function Sharing({ item }: { item: { public: number; unlisted: number; private: number; replays: number; replayBytes: number } }) {
  return <><h4>osu! sharing</h4><dl className="admin-detail-facts"><Fact label="Public scores">{count(item.public)}</Fact><Fact label="Unlisted scores">{count(item.unlisted)}</Fact><Fact label="Private scores">{count(item.private)}</Fact><Fact label="Uploaded replays">{count(item.replays)}</Fact><Fact label="Replay storage">{adminBytes(item.replayBytes)}</Fact></dl></>;
}

export function AdminOsuWorkspace() {
  const [view, setView] = useState<View>("players");
  const [scope, setScope] = useState<OsuAdminScope>();
  const [revision, setRevision] = useState(0);
  const { data, loading, error } = useRecords<OsuAdminOverview>("overview?offset=0", revision);
  function openScores(next: OsuAdminScope) { setScope(next); setView("scores"); }
  return <div className="admin-osu admin-record-workspace">
    <div className="admin-health-strip" aria-label="osu totals"><span><strong>{data ? count(data.summary.profiles) : "\u2014"}</strong> synced profiles</span><span><strong>{data ? count(data.summary.scores) : "\u2014"}</strong> scores</span><span><strong>{data ? count(data.summary.uploaded) : "\u2014"}</strong> replay files</span><span><strong>{data ? adminBytes(data.summary.replayBytes) : "\u2014"}</strong> replay storage</span><span className="admin-note">All time</span><button type="button" onClick={() => setRevision(v => v + 1)} disabled={loading}>Refresh workspace</button></div>
    {error ? <p className="admin-error" role="alert">{error}</p> : null}
    <div className="admin-record-layout">
      <nav className="admin-record-nav" aria-label="osu admin views">{views.map(v => <button type="button" key={v.id} aria-current={view === v.id ? "page" : undefined} onClick={() => setView(v.id)}><strong>{v.label}</strong><small>{v.detail}</small></button>)}<p>Admin access<br />Read-only records</p></nav>
      <div className="admin-record-main">
        {view === "players" ? <Players key="players" revision={revision} openScores={openScores} /> : null}
        {view === "beatmaps" ? <Beatmaps key="beatmaps" revision={revision} openScores={openScores} /> : null}
        {view === "scores" ? <Scores key={JSON.stringify(scope)} revision={revision} scope={scope} clearScope={() => setScope(undefined)} /> : null}
        {view === "services" ? <Services revision={revision} summary={data?.summary} /> : null}
      </div>
    </div>
  </div>;
}

function RecordFilters({ query, setQuery, submit, kind, setKind, choices, placeholder, reset }: { query: string; setQuery: (s: string) => void; submit: () => void; kind: string; setKind: (s: string) => void; choices: [string, string][]; placeholder: string; reset: () => void }) {
  return <form className="admin-filters" onSubmit={e => { e.preventDefault(); submit(); }}><label className="admin-search">Search records<input type="search" aria-label="Search records" maxLength={200} placeholder={placeholder} value={query} onChange={e => setQuery(e.target.value)} /></label><label>Record type<select aria-label="Record type" value={kind} onChange={e => setKind(e.target.value)}>{choices.map(([v, label]) => <option key={v} value={v}>{label}</option>)}</select></label><button type="submit">Search</button><button type="button" onClick={reset}>Reset</button></form>;
}

function Players({ revision, openScores }: { revision: number; openScores: (scope: OsuAdminScope) => void }) {
  const [query, setQuery] = useState(""); const [search, setSearch] = useState(""); const [kind, setKind] = useState(""); const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<number>();
  const { data, loading, error } = useRecords<OsuAdminRecords<OsuAdminPlayer>>(`players?${new URLSearchParams({ q: search, kind, offset: String(offset) })}`, revision);
  const item = data?.items.find(p => p.userId === selected);
  return <><div className="admin-section-heading"><h2>Players & accounts</h2><span>{data ? `${count(data.total)} hub users` : ""}</span></div>
    <RecordFilters query={query} setQuery={setQuery} submit={() => { setSearch(query.trim()); setOffset(0); }} kind={kind} setKind={v => { setKind(v); setOffset(0); }} choices={[["", "All hub users"], ["synced", "osu! profile synced"], ["unsynced", "No osu! profile"]]} placeholder="Handle, name or osu! user ID" reset={() => { setQuery(""); setSearch(""); setKind(""); setOffset(0); }} />
    {error ? <p role="alert" className="admin-error">{error}</p> : null}
    <div className="admin-record-split"><div className="admin-record-list"><RecordTable label="Player records" headers={["Player", "osu! profile", "Scores", "Connections", "Last score"]} loading={loading} empty={!error && data?.items.length === 0}>
      {data?.items.map(p => <tr key={p.userId} className={selected === p.userId ? "is-selected" : ""}><td><button className="admin-record-link" onClick={() => setSelected(p.userId)} aria-label={`Inspect player ${p.handle}`}>{p.displayName || p.handle}</button><small>@{p.handle}</small></td><td>{p.username || "Not synced"}<small>{p.osuUserId ? `osu! #${p.osuUserId}` : ""}</small></td><td>{count(p.scores)}<small>{count(p.public)} public</small></td><td>{p.accounts.map(a => a.provider).join(", ") || "None"}<small>{count(p.activeCredentials)} active credentials</small></td><td><DateValue value={p.lastScoreAt} /></td></tr>)}
    </RecordTable><Pager offset={offset} total={data?.total ?? 0} length={data?.items.length ?? 0} loading={loading} change={setOffset} /></div>
    {item ? <Inspector title={item.username || item.displayName || item.handle} subtitle={`@${item.handle} / Hub user #${item.userId}`}>
      <button type="button" className="admin-primary-command" onClick={() => openScores({ userId: item.userId, label: item.username || item.handle })}>Inspect player scores</button>
      <dl className="admin-detail-facts"><Fact label="Hub account created"><DateValue value={item.createdAt} /></Fact><Fact label="osu! user ID">{item.osuUserId || "Not synced"}</Fact><Fact label="Country">{item.country || "Not recorded"}</Fact><Fact label="Profile last synced"><DateValue value={item.profileUpdatedAt} /></Fact></dl>
      <Sharing item={item} /><h4>Linked accounts</h4><ul className="admin-account-list">{item.accounts.map(a => <li key={a.provider}><strong>{a.provider}</strong><span>{a.username || "No username"}</span><small>{a.verified ? "Verified" : "Unverified"} / linked <DateValue value={a.createdAt} /></small></li>)}</ul>{item.accounts.length === 0 ? <p className="admin-note">No linked accounts.</p> : null}
      <h4>Hub upload access</h4><dl className="admin-detail-facts"><Fact label="Active credentials">{count(item.activeCredentials)}</Fact><Fact label="Last credential use"><DateValue value={item.lastCredentialUse} /></Fact></dl><p className="admin-note">Credential activity is hub-wide. osu! profile details are supplied by desktop clients.</p>
    </Inspector> : <Inspector title="Select a player"><p className="admin-note">Account connections, profile sync and sharing activity.</p></Inspector>}
    </div></>;
}

function Beatmaps({ revision, openScores }: { revision: number; openScores: (scope: OsuAdminScope) => void }) {
  const [query, setQuery] = useState(""); const [search, setSearch] = useState(""); const [kind, setKind] = useState(""); const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<string>();
  const { data, loading, error } = useRecords<OsuAdminRecords<OsuAdminBeatmap>>(`beatmaps?${new URLSearchParams({ q: search, kind, offset: String(offset) })}`, revision);
  const item = data?.items.find(b => b.key === selected);
  return <><div className="admin-section-heading"><h2>Beatmap records</h2><span>{data ? `${count(data.total)} stored difficulties` : ""}</span></div>
    <RecordFilters query={query} setQuery={setQuery} submit={() => { setSearch(query.trim()); setOffset(0); }} kind={kind} setKind={v => { setKind(v); setOffset(0); }} choices={[["", "All difficulties"], ["online", "Online ID present"], ["local", "Local identity"]]} placeholder="Title, artist, creator or beatmap ID" reset={() => { setQuery(""); setSearch(""); setKind(""); setOffset(0); }} />
    {error ? <p role="alert" className="admin-error">{error}</p> : null}
    <div className="admin-record-split"><div className="admin-record-list"><RecordTable label="Beatmap records" headers={["Beatmap / difficulty", "Identity", "Stars", "Scores", "Players", "Updated"]} loading={loading} empty={!error && data?.items.length === 0}>
      {data?.items.map(b => <tr key={b.key} className={selected === b.key ? "is-selected" : ""}><td><button className="admin-record-link" onClick={() => setSelected(b.key)} aria-label={`Inspect beatmap ${b.title} ${b.version}`}>{b.title}</button><small>{b.artist} / {b.version}</small></td><td>{b.onlineId ? `#${b.onlineId}` : "Local"}</td><td>{b.stars.toFixed(2)}</td><td>{count(b.scores)}<small>{count(b.public)} public</small></td><td>{count(b.players)}</td><td><DateValue value={b.updatedAt} /></td></tr>)}
    </RecordTable><Pager offset={offset} total={data?.total ?? 0} length={data?.items.length ?? 0} loading={loading} change={setOffset} /></div>
    {item ? <Inspector title={item.title} subtitle={`${item.artist} / ${item.version}`}>
      <button type="button" className="admin-primary-command" onClick={() => openScores({ difficultyKey: item.key, label: `${item.title} [${item.version}]` })}>Inspect difficulty scores</button>
      <dl className="admin-detail-facts"><Fact label="Creator">{item.creator || "Not recorded"}</Fact><Fact label="Beatmap ID">{item.onlineId || "Local identity"}</Fact><Fact label="Beatmapset ID">{item.setOnlineId || "Local identity"}</Fact><Fact label="Ruleset">{item.ruleset}</Fact><Fact label="Difficulty">{item.stars.toFixed(2)} stars</Fact><Fact label="Tempo">{item.bpm.toLocaleString()} BPM</Fact><Fact label="Length">{Math.floor(item.lengthMs / 60000)}:{String(Math.floor(item.lengthMs / 1000) % 60).padStart(2, "0")}</Fact><Fact label="Last score stored"><DateValue value={item.lastScoreAt} /></Fact></dl>
      <Sharing item={item} /><p className="admin-note">Metadata from synced scores. Star ratings are stored values.</p>
    </Inspector> : <Inspector title="Select a difficulty"><p className="admin-note">Beatmap metadata, contributors and related score records.</p></Inspector>}
    </div></>;
}

function Scores({ revision, scope, clearScope }: { revision: number; scope?: OsuAdminScope; clearScope: () => void }) {
  const [query, setQuery] = useState(""); const [search, setSearch] = useState(""); const [visibility, setVisibility] = useState(""); const [status, setStatus] = useState(""); const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<number>();
  const { data, loading, error } = useRecords<OsuAdminOverview>(`overview?${adminScoreParams(search, visibility, status, offset, scope)}`, revision);
  const item = data?.items.find(s => s.id === selected);
  return <><div className="admin-section-heading"><h2>Scores & replays</h2><span>{data ? `${count(data.total)} matching records` : ""}</span></div>
    {scope ? <div className="admin-scope"><span>Showing scores for <strong>{scope.label}</strong></span><button type="button" onClick={clearScope}>Show all scores</button></div> : null}
    <form className="admin-filters" onSubmit={e => { e.preventDefault(); setSearch(query.trim()); setOffset(0); }}><label className="admin-search">Search scores<input type="search" aria-label="Search scores" maxLength={200} placeholder="Player, beatmap or difficulty" value={query} onChange={e => setQuery(e.target.value)} /></label><label>Visibility<select aria-label="Visibility" value={visibility} onChange={e => { setVisibility(e.target.value); setOffset(0); }}><option value="">All visibility</option><option value="public">Public</option><option value="unlisted">Unlisted</option><option value="private">Private</option></select></label><label>Replay state<select aria-label="Replay state" value={status} onChange={e => { setStatus(e.target.value); setOffset(0); }}><option value="">All states</option><option value="uploaded">Uploaded</option><option value="pending">Metadata only</option><option value="none">No replay</option></select></label><button type="submit">Search</button><button type="button" onClick={() => { setQuery(""); setSearch(""); setVisibility(""); setStatus(""); setOffset(0); }}>Reset</button></form>
    {error ? <p role="alert" className="admin-error">{error}</p> : null}
    <div className="admin-record-split"><div className="admin-record-list"><RecordTable label="Score records" headers={["Score / beatmap", "Player", "Visibility", "Replay", "Stored"]} loading={loading} empty={!error && data?.items.length === 0}>
      {data?.items.map(s => <tr key={s.id} className={selected === s.id ? "is-selected" : ""}><td><button className="admin-record-link" onClick={() => setSelected(s.id)} aria-label={`Inspect score ${s.id}`}>{s.title}</button><small>#{s.id} / {s.difficulty}</small></td><td>{s.username || s.handle}<small>@{s.handle}</small></td><td><Status value={s.visibility} /></td><td><Status value={s.status} /><small>{s.status === "uploaded" ? adminBytes(s.byteSize) : ""}</small></td><td><DateValue value={s.createdAt} /></td></tr>)}
    </RecordTable><Pager offset={offset} total={data?.total ?? 0} length={data?.items.length ?? 0} loading={loading} change={setOffset} /></div>
    {item ? <ScoreInspector item={item} /> : <Inspector title="Select a score"><p className="admin-note">Score result, visibility and replay upload metadata.</p></Inspector>}
    </div></>;
}

function ScoreInspector({ item }: { item: OsuAdminShare }) {
  return <Inspector title={`Score #${item.id}`} subtitle={`${item.title} [${item.difficulty}]`}><dl className="admin-detail-facts"><Fact label="Player">{item.username || item.handle}</Fact><Fact label="Visibility"><Status value={item.visibility} /></Fact><Fact label="Result">{item.passed ? "Passed" : "Failed"}</Fact><Fact label="Accuracy">{(item.accuracy * 100).toFixed(2)}%</Fact><Fact label="Score">{count(item.totalScore)}</Fact><Fact label="Performance">{item.performancePoints === null ? "Not recorded" : `${item.performancePoints.toFixed(2)} PP`}</Fact><Fact label="Combo / misses">{count(item.maxCombo)}x / {count(item.misses)}</Fact><Fact label="Mods">{item.mods?.join(", ") || "No Mod"}</Fact><Fact label="Played"><DateValue value={item.playedAt} /></Fact></dl><h4>Replay file</h4><dl className="admin-detail-facts"><Fact label="Status"><Status value={item.status} /></Fact><Fact label="Size">{item.status === "uploaded" ? adminBytes(item.byteSize) : "No file stored"}</Fact><Fact label="Uploaded"><DateValue value={item.uploadedAt} /></Fact></dl><p className="admin-note">Metadata only. Private replay downloads and share links are not exposed here.</p></Inspector>;
}

function Services({ revision, summary }: { revision: number; summary?: OsuAdminOverview["summary"] }) {
  const [check, setCheck] = useState(0);
  const { data, loading, error } = useRecords<{ items: OsuAdminProvider[] }>("providers", revision + check);
  return <><div className="admin-section-heading"><h2>Provider availability</h2><button type="button" disabled={loading} onClick={() => setCheck(v => v + 1)}>Check providers</button></div>{error ? <p className="admin-error" role="alert">{error}</p> : null}<RecordTable label="Provider availability" headers={["Provider", "Configuration", "Availability", "Checked"]} loading={loading} empty={false}>{data?.items.map(p => <tr key={p.name}><td>{p.name}</td><td>{p.browserOnly ? "No server integration" : p.configured ? "Configured" : "Not configured"}</td><td>{p.browserOnly ? "Browser handoff only" : p.available ? "Available" : "Unavailable"}</td><td>{p.checkedAt ? <time dateTime={p.checkedAt}>{new Date(p.checkedAt).toLocaleString()}</time> : "Not checked"}</td></tr>)}</RecordTable><div className="admin-connection-band"><h3>Hub connections</h3><dl className="admin-detail-facts"><Fact label="Linked accounts">{summary ? count(summary.connectedAccounts) : "\u2014"}</Fact><Fact label="Active upload credentials">{summary ? count(summary.activeCredentials) : "\u2014"}</Fact><Fact label="Pending device requests">{summary ? count(summary.pendingDevices) : "\u2014"}</Fact></dl><p className="admin-note">Connection counts cover all hub games.</p></div></>;
}
