import { useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Helmet } from "../lib/helmet";
import { SectionHeader } from "../components/SectionHeader";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { Skeleton } from "../components/ui/Skeleton";
import { useCatalogRequest } from "../hooks/useCatalogRequest";
import { beatmapLinks, mediaUrl, modeName, numberRange, osuClient, rulesets, skinLinks, skinSource, sliderRangeValue } from "../lib/osuCatalog";
import { BeatmapSearchFilters, Provider, ProviderCursor, Ruleset, SearchBeatmapItemsRequest, SearchSkinsRequest, SkinDownloadHandoffKind, SkinItem, SkinProvider, SkinProviderCursor, SkinSort, SortDirection } from "../gen/aimmod/osu/v1/osu_pb";
import "./osuCatalog.css";

const ranges = [["stars", "Stars", 10, 0.1], ["bpm", "BPM", 300, 1], ["lengthSeconds", "Length", 600, 5], ["approachRate", "AR", 11, 0.1], ["circleSize", "CS", 10, 0.1], ["overallDifficulty", "OD", 11, 0.1]] as const;
const beatmapSort = [["relevance_desc", "Relevance"], ["ranked_desc", "Recently ranked"], ["updated_desc", "Recently updated"], ["plays_desc", "Most played"], ["favourites_desc", "Most favourited"], ["difficulty_asc", "Difficulty: low to high"], ["difficulty_desc", "Difficulty: high to low"], ["title_asc", "Title"]] as const;
const skinSort = [["1", "Relevance"], ["2", "Newest"], ["3", "Most viewed"], ["4", "Most downloaded"], ["5", "Name"]] as const;

function Select({ label, value, options, onChange }: { label: string; value: string; options: readonly (readonly [string, string])[]; onChange: (value: string) => void }) {
  return <label>{label}<select value={value} onChange={event => onChange(event.target.value)}>{options.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>;
}

export function RangeSlider({ name, label, limit, step, minimum, maximum, onChange }: {
  name: string; label: string; limit: number; step: number; minimum: string; maximum: string;
  onChange: (endpoint: "Min" | "Max", value: string) => void;
}) {
  const id = useId();
  const dragEndpoint = useRef<"Min" | "Max" | null>(null);
  const extent = useRef(limit);
  const parse = (value: string) => { try { return numberRange(value, "")?.minimum; } catch { return undefined; } };
  const min = parse(minimum);
  const max = parse(maximum);
  // Keep shared URLs outside the usual scale editable without shrinking the rail during a drag.
  extent.current = Math.max(extent.current, Math.ceil(min ?? 0), Math.ceil(max ?? 0));
  const ceiling = extent.current;
  const low = min ?? -step;
  const high = max ?? ceiling + step;
  const position = (value: number) => `${(value + step) / (ceiling + 2 * step) * 100}%`;
  const display = (value: string) => {
    if (!value.trim()) return "Any";
    const number = parse(value);
    if (number === undefined) return "Invalid";
    return name === "lengthSeconds" ? `${Math.floor(number / 60)}:${String(Number((number % 60).toFixed(3))).padStart(2, "0")}` : String(number);
  };
  const change = (endpoint: "Min" | "Max", value: number) => onChange(endpoint, sliderRangeValue(value, endpoint, ceiling, step, endpoint === "Min" ? maximum : minimum));
  const pointerValue = (element: HTMLDivElement, clientX: number) => {
    const rect = element.getBoundingClientRect();
    const fraction = Math.max(0, Math.min(1, (clientX - rect.left - 12) / (rect.width - 24)));
    return Math.round((fraction * (ceiling + 2 * step) - step) / step) * step;
  };
  return <fieldset className={`catalog-range-control${name === "stars" ? " catalog-star-range" : ""}`}>
    <legend>{label}</legend>
    <div className="catalog-range-values">
      <label htmlFor={`${id}-min`}>Minimum <strong>{display(minimum)}</strong></label>
      <label htmlFor={`${id}-max`}>Maximum <strong>{display(maximum)}</strong></label>
    </div>
    <div className="catalog-dual-range" style={{ "--range-low": position(low), "--range-high": position(high) } as CSSProperties}
      onPointerDown={event => {
        if (event.target instanceof HTMLInputElement || event.button !== 0) return;
        const value = pointerValue(event.currentTarget, event.clientX);
        const endpoint = Math.abs(value - low) < Math.abs(value - high) ? "Min" : "Max";
        dragEndpoint.current = endpoint;
        event.currentTarget.setPointerCapture(event.pointerId);
        event.currentTarget.querySelector<HTMLInputElement>(`#${CSS.escape(`${id}-${endpoint.toLowerCase()}`)}`)?.focus({ preventScroll: true });
        change(endpoint, value);
      }}
      onPointerMove={event => { if (dragEndpoint.current) change(dragEndpoint.current, pointerValue(event.currentTarget, event.clientX)); }}
      onPointerUp={() => { dragEndpoint.current = null; }} onPointerCancel={() => { dragEndpoint.current = null; }}>
      <div className="catalog-range-rail" aria-hidden="true" />
      {(["Min", "Max"] as const).map(endpoint => <input key={endpoint} id={`${id}-${endpoint.toLowerCase()}`} type="range" min={-step} max={ceiling + step} step={step}
        value={endpoint === "Min" ? low : high} aria-label={`${endpoint === "Min" ? "Minimum" : "Maximum"} ${label}`}
        aria-valuetext={display(endpoint === "Min" ? minimum : maximum) === "Any" ? `No ${endpoint === "Min" ? "minimum" : "maximum"}` : `${display(endpoint === "Min" ? minimum : maximum)}${name === "stars" ? " stars" : name === "bpm" ? " BPM" : ""}`}
        onChange={event => change(endpoint, Number(event.target.value))} />)}
    </div>
    <div className="catalog-range-scale" aria-hidden="true"><span>0</span><span>{display(String(ceiling / 2))}</span><span>{display(String(ceiling))}</span></div>
  </fieldset>;
}
function External({ href, children }: { href: string; children: ReactNode }) { return <Button href={href} target="_blank" rel="noopener noreferrer">{children}</Button>; }
function CatalogLoading({ skins = false }: { skins?: boolean }) {
  return <div role="status" aria-label={skins ? "Loading skins" : "Loading beatmaps"}><p className="hub-results">Loading {skins ? "skins" : "beatmaps"}...</p><div aria-hidden="true" className={skins ? "catalog-skins" : "catalog-list"}>{Array.from({ length: 6 }, (_, index) => <Skeleton key={index} className={skins ? "h-56" : "mb-3 h-24"} />)}</div></div>;
}
function Unavailable({ retry, source, name }: { retry: () => void; source: string; name: string }) {
  return <div className="catalog-notice" role="status"><p>{name} is unavailable. Please try again.</p><div className="catalog-actions"><Button onClick={retry}>Try again</Button><External href={source}>Browse {name}</External></div></div>;
}
function Cover({ src, name, className = "" }: { src: string; name: string; className?: string }) {
  const url = mediaUrl(src);
  const [failed, setFailed] = useState(false);
  return url && !failed ? <img className={className} src={url} alt={name} loading="lazy" onError={() => setFailed(true)} /> : <div className={`${className} catalog-no-image`}>No image available</div>;
}

export function OsuCatalogPage({ skins = false }: { skins?: boolean }) {
  const [params, setParams] = useSearchParams();
  const get = (key: string, fallback = "") => params.get(key) ?? fallback;
  function update(key: string, value: string) { const next = new URLSearchParams(params); next.delete("item"); next.delete("itemProvider"); if (value) next.set(key, value); else next.delete(key); setParams(next, { replace: true }); }
  const mode = Number(get("mode", skins ? "0" : "1"));
  const ruleset = [0, 1, 2, 3, 4].includes(mode) ? mode as Ruleset : Ruleset.UNSPECIFIED;
  const provider = get("provider", "1");
  let validation = "";
  const defaultSort = skins ? "1" : get("q").trim() ? "relevance_desc" : "ranked_desc";
  const filters = new BeatmapSearchFilters({ ruleset, status: get("status", "ranked") });
  if (!skins) {
    try { for (const [key] of ranges) filters[key] = numberRange(get(`${key}Min`), get(`${key}Max`)); }
    catch (error) { validation = (error as Error).message; }
  }
  const request = skins ? new SearchSkinsRequest({ query: get("q").trim().slice(0, 256), providers: provider === "all" ? [SkinProvider.OSU_SKINS, SkinProvider.OSUCK] : [provider === "2" ? SkinProvider.OSUCK : SkinProvider.OSU_SKINS], filters: { rulesets: ruleset ? [ruleset] : [], creator: get("creator"), player: get("player") }, sort: Number(get("sort", "1")) as SkinSort, direction: get("direction", "desc") === "asc" ? SortDirection.ASCENDING : SortDirection.DESCENDING })
    : new SearchBeatmapItemsRequest({ query: get("q").trim().slice(0, 256), providers: [Provider.OSU_OFFICIAL], filters, sort: get("sort", defaultSort) });
  const key = `${skins ? "skins" : "beatmaps"}:${request.toJsonString()}`;
  const activeRanges = ranges.slice(1).filter(([key]) => get(`${key}Min`) || get(`${key}Max`)).length;
  return <PageStack className="catalog-workspace"><Helmet><title>{skins ? "Skins" : "Beatmaps"} · AimMod Hub</title></Helmet><PageSection className="catalog-section">
    <SectionHeader level={1} eyebrow="osu!" title={skins ? "Skins" : "Beatmaps"} aside={<External href={skins ? "https://osuskins.net" : "https://osu.ppy.sh/beatmapsets"}>{skins ? "Browse osuskins.net" : "Browse osu!"}</External>} />
    {skins && <Link className="inline-flex my-3 text-cyan underline underline-offset-4" to="/osu/skin-builder">Build your own AimMod skin →</Link>}
    <div className="hub-filters catalog-filters">
      <label>Search {skins ? "skins" : "beatmaps"}<input type="search" value={get("q")} maxLength={256} placeholder={skins ? "Skin name" : "Title, artist, or mapper"} onChange={event => update("q", event.target.value)} /></label>
      {!skins && <RangeSlider name="stars" label="Stars" limit={10} step={0.1} minimum={get("starsMin")} maximum={get("starsMax")} onChange={(endpoint, value) => update(`stars${endpoint}`, value)} />}
      <Select label="Mode" value={String(ruleset)} options={rulesets} onChange={value => update("mode", value)} />
      {skins ? <Select label="Source" value={provider} options={[["1", "osuskins.net"], ["2", "skins.osuck.net"], ["all", "All sources"]]} onChange={value => update("provider", value)} /> : <Select label="Status" value={get("status", "ranked")} options={[["any", "Any status"], ["ranked", "Ranked"], ["loved", "Loved"], ["qualified", "Qualified"], ["pending", "Pending"], ["graveyard", "Graveyard"]]} onChange={value => update("status", value)} />}
      <Select label="Sort by" value={get("sort", defaultSort)} options={skins ? skinSort : beatmapSort} onChange={value => update("sort", value)} />
      {skins && <><Select label="Order" value={get("direction", "desc")} options={[["desc", "Descending"], ["asc", "Ascending"]]} onChange={value => update("direction", value)} /><label>Creator<input value={get("creator")} maxLength={128} onChange={event => update("creator", event.target.value)} /></label><label>Player<input value={get("player")} maxLength={128} onChange={event => update("player", event.target.value)} /></label></>}
      <Button onClick={() => setParams({})} disabled={params.size === 0}>Reset filters</Button>
    </div>
    {!skins && <details className="catalog-ranges"><summary>More filters {activeRanges > 0 && <span className="catalog-filter-count">{activeRanges} active</span>}</summary><div className="catalog-range-grid">{ranges.slice(1).map(([key, label, limit, step]) => <RangeSlider key={key} name={key} label={label} limit={limit} step={step} minimum={get(`${key}Min`)} maximum={get(`${key}Max`)} onChange={(endpoint, value) => update(`${key}${endpoint}`, value)} />)}</div></details>}
    {validation ? <p role="alert" className="catalog-notice">{validation}</p> : skins ? <SkinResults key={key} request={request as SearchSkinsRequest} /> : <BeatmapResults key={key} request={request as SearchBeatmapItemsRequest} />}
  </PageSection></PageStack>;
}

function Pagination({ page, hasNext, loading, previous, next }: { page: number; hasNext: boolean; loading: boolean; previous: () => void; next: () => void }) {
  return <nav aria-label="Catalog pages" className="catalog-pagination"><Button disabled={page === 0 || loading} onClick={previous}>Previous</Button><span>Page {page + 1}</span><Button disabled={!hasNext || loading} onClick={next}>Next</Button></nav>;
}
function BeatmapResults({ request }: { request: SearchBeatmapItemsRequest }) {
  const [pages, setPages] = useState<ProviderCursor[][]>([[]]);
  const [page, setPage] = useState(0);
  const [params, setParams] = useSearchParams();
  const selected = params.get("item");
  function select(id?: string, replace = false) { const next = new URLSearchParams(params); if (id) next.set("item", id); else next.delete("item"); setParams(next, { replace }); }
  const search = new SearchBeatmapItemsRequest({ ...request, pageTokens: pages[page] });
  const result = useCatalogRequest(`beatmaps:${search.toJsonString()}`, signal => osuClient.searchBeatmapItems(search, { signal }));
  function changePage(value: number) { select(undefined, true); setPage(value); }
  const unavailable = result.error || result.data?.providers.some(status => !status.available);
  return <div className="catalog-results" aria-busy={result.loading}>
    {result.loading && <CatalogLoading />}
    {unavailable && <Unavailable name="osu!" source="https://osu.ppy.sh/beatmapsets" retry={result.retry} />}
    {result.data && <><p className="hub-results">{result.data.items.length} beatmapsets · osu!</p>
      {!result.data.items.length && !unavailable && <p className="catalog-notice" role="status">No beatmaps found. Try another search or reset your filters.</p>}
      <div className="catalog-layout"><div className="catalog-list" role="region" aria-label="Beatmap results" tabIndex={0}>{result.data.items.map(item => <button className="catalog-map" key={`${item.provider}:${item.sourceId}`} aria-pressed={selected === item.sourceId} onClick={() => select(item.sourceId)}>
        <Cover src={item.coverUrl} name="" className="catalog-map-cover" /><span className="catalog-item-text"><strong>{item.title}</strong><span>{item.artist} · {item.creator}</span><span>{item.status} · {item.minimumStars.toFixed(1)}{item.minimumStars !== item.maximumStars ? `–${item.maximumStars.toFixed(1)}` : ""} stars · {item.beatmapCount} {item.beatmapCount === 1 ? "difficulty" : "difficulties"}</span></span>
      </button>)}</div>{selected && beatmapLinks(selected) && <BeatmapDetail key={selected} id={selected} filters={request.filters} close={() => select()} />}</div>
    </>}
    <Pagination page={page} loading={result.loading} hasNext={!!result.data?.nextPageTokens.length} previous={() => changePage(page - 1)} next={() => { if (result.data?.nextPageTokens.length) { setPages([...pages.slice(0, page + 1), result.data.nextPageTokens]); changePage(page + 1); } }} />
  </div>;
}
function DetailFrame({ title, close, children }: { title: string; close: () => void; children: ReactNode }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus({ preventScroll: true });
    if (window.matchMedia("(max-width: 1000px), (max-height: 649px)").matches) heading.current?.scrollIntoView({ block: "start" });
  }, []);
  return <section className="catalog-detail" aria-label={title} tabIndex={0}><div className="catalog-detail-heading"><h2 ref={heading} tabIndex={-1}>{title}</h2><Button onClick={close} aria-label="Close details" title="Close details">×</Button></div>{children}</section>;
}
function AudioPreview({ url }: { url: string }) {
  const audio = useRef<HTMLAudioElement>(null);
  const [error, setError] = useState(false);
  useEffect(() => { const element = audio.current; return () => { if (element) { element.pause(); element.removeAttribute("src"); element.load(); } }; }, [url]);
  return <div className="catalog-audio"><audio ref={audio} src={url} controls preload="none" aria-label="Beatmap audio preview" onError={() => setError(true)} />{error && <p role="status">Audio preview unavailable.</p>}</div>;
}
function BeatmapDetail({ id, filters, close }: { id: string; filters?: BeatmapSearchFilters; close: () => void }) {
  const result = useCatalogRequest(`beatmap:${id}`, signal => osuClient.getBeatmapItem({ provider: Provider.OSU_OFFICIAL, sourceId: id }, { signal }), 0);
  const item = result.data?.item;
  const links = beatmapLinks(id);
  const preview = item && mediaUrl(item.previewUrl);
  return <DetailFrame title={item?.title || "Beatmap details"} close={close}>
    {result.loading ? <CatalogLoading /> : result.error || !item ? <Unavailable name="osu!" source={links?.source ?? "https://osu.ppy.sh/beatmapsets"} retry={result.retry} /> : <>
      <Cover src={item.coverUrl} name={`${item.artist} - ${item.title}`} className="catalog-detail-cover" />
      <p>{item.artist} · mapped by {item.creator}</p><p className="text-muted">{item.status} · {item.minimumBpm}{item.minimumBpm !== item.maximumBpm ? `–${item.maximumBpm}` : ""} BPM</p>
      <p className="text-muted">{item.playCount.toLocaleString()} plays · {item.favouriteCount.toLocaleString()} favourites</p>
      {links && <div className="catalog-actions">{item.downloadHandoff?.available && <Button href={links.osu}>Open in osu!</Button>}<Button href={links.aimmod}>Open in AimMod</Button><Button to="/app/osu">Get AimMod</Button><External href={links.source}>View on osu!</External></div>}
      {preview && <AudioPreview key={preview} url={preview} />}
      <h3>All difficulties</h3><div className="catalog-difficulties">{item.difficulties.map(diff => {
        const matches = (!filters?.ruleset || filters.ruleset === diff.ruleset) && ranges.every(([key]) => { const range = filters?.[key]; const value = diff[key]; return !range || ((range.minimum === undefined || value >= range.minimum) && (range.maximum === undefined || value <= range.maximum)); });
        return <div key={diff.beatmapId}><strong>{diff.name}</strong><span>{modeName(diff.ruleset)} · {diff.stars.toFixed(2)} stars · {Math.floor(diff.lengthSeconds / 60)}:{String(diff.lengthSeconds % 60).padStart(2, "0")}</span><span>AR {diff.approachRate} · CS {diff.circleSize} · OD {diff.overallDifficulty} · HP {diff.drainRate} · {diff.bpm} BPM</span>{!matches && <span>Outside selected difficulty filters</span>}</div>;
      })}</div>
      {!item.difficulties.length && <p>No difficulties available.</p>}
      {!!item.tags.length && <p className="catalog-tags text-muted">{item.tags.join(" · ")}</p>}
    </>}
  </DetailFrame>;
}
function SkinResults({ request }: { request: SearchSkinsRequest }) {
  const [pages, setPages] = useState<SkinProviderCursor[][]>([[]]);
  const [page, setPage] = useState(0);
  const [params, setParams] = useSearchParams();
  const selected = params.get("item");
  const selectedProvider = Number(params.get("itemProvider") ?? request.providers[0]) as SkinProvider;
  function select(item?: SkinItem, replace = false) { const next = new URLSearchParams(params); if (item) { next.set("item", item.sourceId); next.set("itemProvider", String(item.provider)); } else { next.delete("item"); next.delete("itemProvider"); } setParams(next, { replace }); }
  const search = new SearchSkinsRequest({ ...request, pageTokens: pages[page] });
  const result = useCatalogRequest(`skins:${search.toJsonString()}`, signal => osuClient.searchSkins(search, { signal }));
  function changePage(value: number) { select(undefined, true); setPage(value); }
  const unavailable = result.data?.providers.filter(status => !status.available) ?? [];
  return <div className="catalog-results" aria-busy={result.loading}>
    {result.loading && <CatalogLoading skins />}
    {result.error && request.providers.map(provider => { const source = skinSource(provider)!; return <Unavailable key={provider} name={source.name} source={source.url} retry={result.retry} />; })}
    {unavailable.map(status => { const source = skinSource(status.provider); return source && <Unavailable key={status.provider} name={source.name} source={source.url} retry={result.retry} />; })}
    {result.data && <><p className="hub-results">{result.data.items.length} skins</p>
      {!result.data.items.length && !unavailable.length && <p className="catalog-notice" role="status">No skins found. Try another search or reset your filters.</p>}
      <div className="catalog-layout"><div className="catalog-skins" role="region" aria-label="Skin results" tabIndex={0}>{result.data.items.map(item => <button className="catalog-skin" key={`${item.provider}:${item.sourceId}`} aria-pressed={selected === item.sourceId && selectedProvider === item.provider} onClick={() => select(item)}>
        <span className="catalog-skin-media"><Cover src={item.thumbnailUrl} name="" className="catalog-skin-cover" /></span><span className="catalog-item-text"><strong>{item.name}</strong>{item.creator && <span>{item.creator}</span>}<span>{[skinSource(item.provider)?.name, item.rulesets.map(modeName).join(", ")].filter(Boolean).join(" · ")}</span></span>
      </button>)}</div>{selected && skinLinks(selectedProvider, selected) && <SkinDetail key={`${selectedProvider}:${selected}`} provider={selectedProvider} id={selected} close={() => select()} />}</div>
    </>}
    <Pagination page={page} loading={result.loading} hasNext={!!result.data?.nextPageTokens.length} previous={() => changePage(page - 1)} next={() => { if (result.data?.nextPageTokens.length) { setPages([...pages.slice(0, page + 1), result.data.nextPageTokens]); changePage(page + 1); } }} />
  </div>;
}
function SkinDetail({ provider, id, close }: { provider: SkinProvider; id: string; close: () => void }) {
  const result = useCatalogRequest(`skin:${provider}:${id}`, signal => osuClient.getSkin({ provider, sourceId: id }, { signal }), 0);
  const [index, setIndex] = useState(0);
  const item = result.data?.item;
  const source = skinSource(provider)!;
  const links = skinLinks(provider, id);
  const screenshots = item?.screenshots.filter(screenshot => mediaUrl(screenshot.imageUrl)) ?? [];
  const shot = screenshots[index];
  return <DetailFrame title={item?.name || "Skin details"} close={close}>
    {result.loading ? <CatalogLoading skins /> : result.error || !item ? <Unavailable name={source.name} source={links?.source ?? source.url} retry={result.retry} /> : <>
      <Cover key={shot?.imageUrl ?? item.thumbnailUrl} src={shot?.imageUrl ?? item.thumbnailUrl} name={shot?.label || item.name} className="catalog-gallery-image" />
      {!!screenshots.length && <div className="catalog-gallery" aria-label="Skin screenshots">{screenshots.map((screenshot, n) => <button key={`${screenshot.imageUrl}:${n}`} aria-label={screenshot.label || `Screenshot ${n + 1}`} aria-pressed={n === index} onClick={() => setIndex(n)}><Cover src={screenshot.imageUrl} name="" /></button>)}</div>}
      {shot?.label && <p className="text-muted">{shot.label}</p>}
      {item.creator && <p>By {item.creator}</p>}<p className="text-muted">{item.rulesets.map(modeName).join(" · ")}{item.aspectRatios.length ? ` · ${item.aspectRatios.join(", ")}` : ""}</p>
      <p className="text-muted">{item.countsAreApproximate ? "About " : ""}{item.viewCount.toLocaleString()} views · {item.countsAreApproximate ? "about " : ""}{item.downloadCount.toLocaleString()} downloads</p>
      {item.fileSizeBytes > 0n && <p>{item.fileSizeIsApproximate ? "About " : ""}{(Number(item.fileSizeBytes) / 1_000_000).toFixed(1)} MB</p>}
      {!!item.players.length && <p>Players: {item.players.join(", ")}</p>}
      <p>Source: <a href={links?.source ?? source.url} target="_blank" rel="noopener noreferrer" className="text-cyan underline">{source.name}</a></p>
      <div className="catalog-actions">{links && <Button href={links.aimmod}>Open in AimMod</Button>}<External href={links?.source ?? source.url}>View on {source.name}</External></div>
      <h3>Downloads</h3>
      <div className="catalog-actions">{(item.sources.length ? item.sources.map(source => ({ handoff: source.downloadHandoff, variant: source.variant })) : [{ handoff: item.downloadHandoff, variant: "" }]).filter((entry, index, all) => entry.handoff?.available && mediaUrl(entry.handoff.uri) && all.findIndex(other => other.handoff?.uri === entry.handoff?.uri && other.variant === entry.variant) === index).map(({ handoff, variant }) => <External key={`${handoff!.uri}:${variant}`} href={handoff!.uri}>{handoff!.kind === SkinDownloadHandoffKind.DIRECT_URL ? "Download .osk" : "Download on host"}{variant ? ` · ${variant}` : ""}</External>)}</div>
      {item.downloadHandoff?.requiresInteractiveVerification && <p className="text-muted">Complete download verification on {source.name}.</p>}
      {!!item.tags.length && <p className="catalog-tags text-muted">{item.tags.join(" · ")}</p>}
    </>}
  </DetailFrame>;
}
