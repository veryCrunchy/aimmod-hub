import { useEffect, useState } from "react";
import { adminBytes, fetchOsuAdmin, type OsuAdminOverview, type OsuAdminProvider } from "./adminOsu";

function Timestamp({ value, empty = "Not uploaded" }: { value: string | null; empty?: string }) {
  return value ? <time dateTime={value} title={value}>{new Date(value).toLocaleString()}</time> : <span>{empty}</span>;
}

export function AdminOsuWorkspace() {
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState("");
  const [visibility, setVisibility] = useState("");
  const [status, setStatus] = useState("");
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState(0);
  const [data, setData] = useState<OsuAdminOverview | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<OsuAdminProvider[] | null>(null);
  const [providerError, setProviderError] = useState("");
  const [checking, setChecking] = useState(true);
  const [check, setCheck] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(""); setData(null);
    const params = new URLSearchParams({ q: search, visibility, status, offset: String(offset) });
    void fetchOsuAdmin<OsuAdminOverview>(`overview?${params}`, controller.signal)
      .then(result => { if (!controller.signal.aborted) setData(result); })
      .catch(err => { if (!controller.signal.aborted) setError(err instanceof Error ? err.message : "Could not load scores."); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [search, visibility, status, offset, revision]);

  useEffect(() => {
    const controller = new AbortController();
    setChecking(true); setProviderError(""); setProviders(null);
    void fetchOsuAdmin<{ items: OsuAdminProvider[] }>("providers", controller.signal)
      .then(result => { if (!controller.signal.aborted) setProviders(result.items); })
      .catch(err => { if (!controller.signal.aborted) setProviderError(err instanceof Error ? err.message : "Could not check providers."); })
      .finally(() => { if (!controller.signal.aborted) setChecking(false); });
    return () => controller.abort();
  }, [check]);

  const summary = data?.summary;
  const count = (value: number | undefined) => value?.toLocaleString() ?? "\u2014";

  return <div className="admin-osu">
    <section aria-labelledby="osu-summary">
      <div className="admin-section-heading"><h2 id="osu-summary">osu! sharing</h2><span>All time</span><button type="button" disabled={loading} onClick={() => setRevision(v => v + 1)}>Refresh data</button></div>
      <dl className="admin-metrics">
        {([["Scores", summary?.scores], ["Public", summary?.public], ["Unlisted", summary?.unlisted], ["Private", summary?.private], ["Uploaded replays", summary?.uploaded], ["Replays without file", summary?.pending], ["Synced profiles", summary?.profiles], ["Profiles with public scores", summary?.publicProfiles]] as const).map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{count(value)}</dd></div>)}
        <div><dt>Replay storage</dt><dd>{summary ? adminBytes(summary.replayBytes) : "\u2014"}</dd></div>
      </dl>
      <div className="admin-connections"><strong>Hub connections</strong><span>{count(summary?.activeCredentials)} active upload credentials</span><span>{count(summary?.connectedAccounts)} linked accounts</span><span>{count(summary?.pendingDevices)} pending device requests</span></div>
      <p className="admin-note">Connections are hub-wide. Synced osu! profiles are supplied by desktop clients.</p>
    </section>

    <section aria-labelledby="osu-shares">
      <div className="admin-section-heading"><h2 id="osu-shares">Score records</h2><span>{data ? `${count(data.total)} matching` : ""}</span></div>
      <form className="admin-filters" onSubmit={event => { event.preventDefault(); setOffset(0); setSearch(query.trim()); }}>
        <label className="admin-search">Search<input value={query} maxLength={200} onChange={event => setQuery(event.target.value)} placeholder="Player, beatmap or difficulty" type="search" /></label>
        <label>Visibility<select aria-label="Visibility" value={visibility} onChange={event => { setVisibility(event.target.value); setOffset(0); }}><option value="">All visibility</option><option value="public">Public</option><option value="unlisted">Unlisted</option><option value="private">Private</option></select></label>
        <label>Replay<select aria-label="Replay state" value={status} onChange={event => { setStatus(event.target.value); setOffset(0); }}><option value="">All replay states</option><option value="uploaded">Uploaded</option><option value="pending">Metadata only</option><option value="none">No replay</option></select></label>
        <button type="submit">Search</button>
        <button type="button" onClick={() => { setQuery(""); setSearch(""); setVisibility(""); setStatus(""); setOffset(0); }}>Reset filters</button>
      </form>
      {error ? <p role="alert" className="admin-error">{error}</p> : null}
      <div className="admin-table-scroll" tabIndex={0} role="region" aria-label="osu score records" aria-busy={loading}>
        <table className="admin-table"><thead><tr><th scope="col">Player / hub handle</th><th scope="col">Beatmap / difficulty</th><th scope="col">Visibility</th><th scope="col">Replay</th><th scope="col">Size</th><th scope="col">Stored</th><th scope="col">Uploaded</th></tr></thead><tbody>
          {data?.items.map(item => <tr key={item.id}><td><strong>{item.username || item.handle}</strong><small>{item.handle}</small></td><td><strong>{item.title}</strong><small>{item.difficulty}</small></td><td><span className={`admin-status is-${item.visibility}`}>{item.visibility}</span></td><td><span className={`admin-status is-${item.status}`}>{item.status === "none" ? "No replay" : item.status === "pending" ? "Metadata only" : "Uploaded"}</span></td><td>{item.status === "uploaded" ? adminBytes(item.byteSize) : "\u2014"}</td><td><Timestamp value={item.createdAt} /></td><td><Timestamp value={item.uploadedAt} /></td></tr>)}
          {loading || (!error && data?.items.length === 0) ? <tr><td colSpan={7} className="admin-table-empty">{loading ? "Loading score records..." : "No scores match these filters."}</td></tr> : null}
        </tbody></table>
      </div>
      <nav className="admin-pagination" aria-label="Score pages"><span aria-live="polite">{data ? data.total === 0 ? "0 records" : data.items.length ? `${offset + 1}\u2013${offset + data.items.length} of ${count(data.total)}` : "No records on this page" : loading ? "Loading..." : "Records unavailable"}</span><button type="button" disabled={loading || offset === 0} aria-label="Previous score page" onClick={() => setOffset(v => Math.max(0, v - 25))}>Previous</button><button type="button" disabled={loading || !data || offset + 25 >= data.total} aria-label="Next score page" onClick={() => setOffset(v => v + 25)}>Next</button></nav>
    </section>

    <section aria-labelledby="osu-providers">
      <div className="admin-section-heading"><h2 id="osu-providers">Provider availability</h2><button type="button" disabled={checking} onClick={() => setCheck(v => v + 1)}>{checking ? "Checking..." : "Check providers"}</button></div>
      {providerError ? <p role="alert" className="admin-error">{providerError}</p> : null}
      <div className="admin-table-scroll" tabIndex={0} role="region" aria-label="Provider availability">
        <table className="admin-table"><thead><tr><th scope="col">Provider</th><th scope="col">Configuration</th><th scope="col">Availability</th><th scope="col">Checked</th></tr></thead><tbody>
          {providers?.map(provider => <tr key={provider.name}>
            <td>{provider.name}</td>
            <td>{provider.browserOnly ? "No server integration" : provider.configured ? "Configured" : "Not configured"}</td>
            <td><span className={`admin-status is-${provider.available ? "uploaded" : "pending"}`}>{provider.browserOnly ? "Browser handoff only" : provider.available ? "Available" : "Unavailable"}</span></td>
            <td><Timestamp value={provider.checkedAt || null} empty="Not checked" /></td>
          </tr>)}
          {checking ? <tr><td colSpan={4} className="admin-table-empty">Checking provider availability...</td></tr> : null}
        </tbody></table>
      </div>
    </section>
  </div>;
}
