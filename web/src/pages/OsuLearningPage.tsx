import { Link, useParams, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { ArrowDown, ArrowLeft, ArrowRight, BookOpen, ChevronRight, Clock3, Crosshair, ExternalLink, Layers3, List, Play, Search, SlidersHorizontal, Target, TrendingUp, X } from "lucide-react";
import { PageSeo } from "../components/PageSeo";
import content from "../../../api/internal/seo/content.json";
import { socialPreviewImage } from "../lib/socialPreview";

type Guide = typeof content.guides[number];
const topics = ["All topics", "Aim & reading", "Timing & tapping", "Practice & progress", "Scores & settings"];
const levels = ["All levels", "Foundations", "Skill building", "Deep dives"];
const topicIcons = [BookOpen, Crosshair, Layers3, TrendingUp, SlidersHorizontal];
function guideTopic(guide: Guide) {
  if (/aim-misses|reading|slider|cursor|jump/.test(guide.slug)) return topics[1];
  if (/timing|stream|burst|tapping|rhythm/.test(guide.slug)) return topics[2];
  if (/points|difficulty|stable|lazer|mods|accuracy-combo|score|settings/.test(guide.slug)) return topics[4];
  return topics[3];
}
function guideLevel(guide: Guide) {
  if (/points|stable|mods|accuracy-combo|choose-maps|build-a-practice/.test(guide.slug)) return levels[1];
  if (/plateau|reviewing|timing-bias|consistency/.test(guide.slug)) return levels[3];
  return levels[2];
}
function readMinutes(guide: Guide) { return Math.max(1, Math.ceil(guide.sections.map(section => section.body).join(" ").split(/\s+/).length / 220)); }
function sourceInfo(url: string) {
  try {
    const parsed = new URL(url), host = parsed.hostname.replace(/^www\./, "");
    const video = host === "youtube.com" || host === "youtu.be";
    const id = host === "youtu.be" ? parsed.pathname.slice(1) : parsed.searchParams.get("v");
    return { host, video, thumbnail: video && id && /^[\w-]{11}$/.test(id) ? `https://i.ytimg.com/vi/${id}/hqdefault.jpg` : undefined };
  } catch { return { host: "Reference", video: false, thumbnail: undefined }; }
}
const hasVideo = (guide: Guide) => guide.sources.some(source => sourceInfo(source.url).video);
const paths = [
  { title: "Build your foundations", description: "Understand your scores. Choose your next map.", icon: BookOpen, slugs: ["accuracy-combo-and-score", "performance-points-and-difficulty", "choose-maps-for-your-goal"] },
  { title: "Find the cause of a miss", description: "Follow the cursor, the rhythm and the evidence.", icon: Crosshair, slugs: ["reviewing-replay-mistakes", "aim-misses-and-cursor-control", "timing-bias-and-unstable-rate"] },
  { title: "Make practice count", description: "Turn a difficult section into lasting progress.", icon: TrendingUp, slugs: ["build-a-practice-session", "jump-stream-and-burst-practice", "full-map-consistency"] },
];
function GuideCard({ guide }: { guide: Guide }) {
  return <article className="okb-guide"><div className="okb-guide-meta"><span>{guideTopic(guide)}</span><span><Clock3 size={13}/>{readMinutes(guide)} min</span></div><h3><Link to={`/osu/learn/${guide.slug}`}>{guide.title}</Link></h3><p>{guide.description}</p><div className="okb-guide-bottom"><span>{guideLevel(guide)}</span>{hasVideo(guide) && <span><Play size={12}/>Video resources</span>}<ArrowRight size={18} aria-hidden="true"/></div></article>;
}
export function OsuLearningPage() {
  const { slug } = useParams();
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const topic = topics.includes(params.get("topic") ?? "") ? params.get("topic")! : topics[0];
  const level = levels.includes(params.get("level") ?? "") ? params.get("level")! : levels[0];
  const videosOnly = params.get("video") === "1";
  const [activeSection, setActiveSection] = useState("section-1");
  const [mobileContents, setMobileContents] = useState(false);
  const guide = content.guides.find(item => item.slug === slug);
  const index = content.routes["/osu/learn"];
  const matching = content.guides.filter(item => (topic === topics[0] || guideTopic(item) === topic) && (level === levels[0] || guideLevel(item) === level) && (!videosOnly || hasVideo(item)) && `${item.title} ${item.description} ${item.sections.map(section => section.body).join(" ")}`.toLowerCase().includes(query.trim().toLowerCase()));
  function filter(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value); else next.delete(key);
    setParams(next, { replace: true });
  }
  useEffect(() => {
    setActiveSection("section-1"); setMobileContents(false);
    if (!guide) return;
    const observer = new IntersectionObserver(entries => { for (const entry of entries) if (entry.isIntersecting) setActiveSection(entry.target.id); }, { rootMargin: "-90px 0px -55% 0px", threshold: 0 });
    document.querySelectorAll(".okb-article-section").forEach(section => observer.observe(section));
    return () => observer.disconnect();
  }, [slug, guide]);
  if (slug && !guide) return <section className="okb okb-empty"><PageSeo title="Guide not found · AimMod Hub" description="This osu! guide is unavailable." noindex/><BookOpen size={32}/><h1>Guide not found</h1><Link to="/osu/learn"><ArrowLeft size={16}/>Back to the knowledge base</Link></section>;
  const related = guide ? [...content.guides].filter(item => item.slug !== guide.slug).sort((a,b) => Number(guideTopic(b) === guideTopic(guide)) - Number(guideTopic(a) === guideTopic(guide))).slice(0,3) : [];
  const toc = guide && <><p className="okb-eyebrow">In this guide</p><ol>{guide.sections.map((section, i) => <li key={section.title}><a aria-current={activeSection === `section-${i+1}` ? "location" : undefined} href={`#section-${i+1}`} onClick={() => { setActiveSection(`section-${i+1}`); setMobileContents(false); }}><span>{String(i+1).padStart(2,"0")}</span>{section.title}</a></li>)}</ol><a className="okb-toc-sources" href="#guide-sources" onClick={() => setMobileContents(false)}>Sources & videos<ArrowDown size={14}/></a></>;
  return <div className={`okb ${guide ? "okb-detail" : "okb-library"}`}>
    <PageSeo title={guide ? `${guide.title} · AimMod Hub` : index.title} description={guide?.description ?? index.description} type={guide ? "article" : "website"} schema={guide ? { "@context": "https://schema.org", "@type": "Article", headline: guide.title, description: guide.description, mainEntityOfPage: `https://aimmod.app/osu/learn/${guide.slug}`, datePublished: content.updatedAt, dateModified: content.updatedAt, author: { "@type": "Organization", name: "AimMod Hub", url: "https://aimmod.app" }, image: socialPreviewImage(`/osu/learn/${guide.slug}`), citation: guide.sources.map(source => source.url) } : undefined}/>
    <nav aria-label="Breadcrumb" className="okb-breadcrumb"><Link to="/osu">osu!</Link><ChevronRight size={13}/><Link to="/osu/learn">Knowledge base</Link>{guide && <><ChevronRight size={13}/><span>{guideTopic(guide)}</span></>}</nav>
    {guide ? <>
      <header className="okb-article-header"><div className="okb-eyebrow"><span>{guideTopic(guide)}</span><span>{guideLevel(guide)}</span></div><h1>{guide.title}</h1><p>{guide.description}</p><div className="okb-byline"><span><BookOpen size={15}/>AimMod Guides</span><span><Clock3 size={15}/>{readMinutes(guide)} min read</span><span>Updated {content.updatedAt}</span></div></header>
      <div className="okb-article-layout"><article className="okb-article">
        <div className="okb-mobile-toc"><button type="button" aria-expanded={mobileContents} onClick={() => setMobileContents(!mobileContents)}><List size={18}/>In this guide<span>{guide.sections.length} sections</span><ArrowDown size={15}/></button>{mobileContents && <nav aria-label="Mobile guide contents" className="okb-toc">{toc}</nav>}</div>
        {guide.sections.map((section,i) => <section id={`section-${i+1}`} className={`okb-article-section ${/practice|exercise|try|measure/i.test(section.title) ? "okb-exercise" : ""}`} key={section.title}><div className="okb-section-number">{String(i+1).padStart(2,"0")}</div><h2>{section.title}</h2>{section.body.split(/\n\s*\n/).map((paragraph,index) => <p key={index}>{paragraph}</p>)}</section>)}
        <section id="guide-sources" className="okb-sources"><div className="okb-section-heading"><div><p className="okb-eyebrow">Keep exploring</p><h2>Sources & further learning</h2></div><span>{guide.sources.length} resources</span></div><div className="okb-resource-list">{guide.sources.map(source => { const info = sourceInfo(source.url); return <a className={`okb-resource ${info.video ? "okb-video" : ""}`} href={source.url} target="_blank" rel="noreferrer" key={source.url}>{info.video ? <div className="okb-video-image">{info.thumbnail && <img src={info.thumbnail} alt="" loading="lazy"/>}<Play size={24}/></div> : <BookOpen className="okb-resource-icon" size={20}/>}<div><span>{info.video ? "Watch" : "Reference"} / {info.host}</span><h3>{source.title}</h3></div><ExternalLink size={16}/></a>; })}</div></section>
      </article><aside className="okb-contents"><nav className="okb-toc" aria-label="In this guide">{toc}<div className="okb-toc-footer"><Clock3 size={15}/>{readMinutes(guide)} min read<span>{guide.sections.length} sections</span></div><Link className="okb-back" to="/osu/learn"><ArrowLeft size={14}/>All guides</Link></nav></aside></div>
      <section className="okb-related"><div className="okb-section-heading"><div><p className="okb-eyebrow">Your next read</p><h2>Keep building on this</h2></div><Link to="/osu/learn">All guides<ArrowRight size={16}/></Link></div><div className="okb-related-grid">{related.map(item => <GuideCard guide={item} key={item.slug}/>)}</div></section>
    </> : <>
      <header className="okb-library-header"><div><p className="okb-eyebrow"><BookOpen size={16}/>Learn / Practice / Improve</p><h1>osu! knowledge base</h1><p>Understand your play. Find your next focus.</p></div><div className="okb-library-count"><strong>{content.guides.length}</strong><span>guides for your next session</span></div></header>
      <section className="okb-paths" aria-labelledby="learning-paths"><div className="okb-section-heading"><h2 id="learning-paths">A place to start</h2><span>Three paths, one step at a time</span></div><div className="okb-path-grid">{paths.map(route => { const guides = route.slugs.map(slug => content.guides.find(guide => guide.slug === slug)).filter((guide): guide is Guide => Boolean(guide)); const Icon = route.icon; return <details className="okb-path" key={route.title}><summary><Icon size={23}/><span><strong>{route.title}</strong><small>{guides.length} guides / {guides.reduce((total,guide) => total+readMinutes(guide),0)} min</small></span><ChevronRight size={18}/></summary><p>{route.description}</p><ol>{guides.map((item,i) => <li key={item.slug}><span>{i+1}</span><Link to={`/osu/learn/${item.slug}`}>{item.title}<ArrowRight size={14}/></Link></li>)}</ol></details>; })}</div></section>
      <section className="okb-browse" aria-labelledby="guide-library"><div className="okb-section-heading"><h2 id="guide-library">The guide library</h2><span role="status">{matching.length} {matching.length === 1 ? "guide" : "guides"}</span></div>
        <div className="okb-filters"><label className="okb-search"><Search size={19}/><input aria-label="Find a guide" type="search" placeholder="Search a skill, a question, a challenge..." value={query} onChange={event => filter("q",event.target.value)}/>{query && <button type="button" title="Clear search" aria-label="Clear search" onClick={() => filter("q","")}><X size={16}/></button>}</label><label className="okb-level"><span>Guide level</span><select aria-label="Guide level" value={level} onChange={event => filter("level",event.target.value === levels[0] ? "" : event.target.value)}>{levels.map(level => <option key={level}>{level}</option>)}</select></label><label className="okb-video-filter"><input type="checkbox" checked={videosOnly} onChange={event => filter("video",event.target.checked ? "1" : "")}/><Play size={14}/>With videos</label></div>
        <div className="okb-browse-layout"><nav className="okb-topics" aria-label="Guide topics"><p className="okb-eyebrow">Browse by focus</p>{topics.map((name,index) => { const Icon = topicIcons[index]; return <button type="button" key={name} aria-pressed={name === topic} onClick={() => filter("topic",name === topics[0] ? "" : name)}><Icon size={17}/><span>{name}</span><small>{name === topics[0] ? content.guides.length : content.guides.filter(guide => guideTopic(guide) === name).length}</small></button>; })}</nav><div><div className="okb-guide-grid">{matching.map(item => <GuideCard key={item.slug} guide={item}/>)}</div>{!matching.length && <div className="okb-empty"><Search size={28}/><h3>No guides match</h3><p>Try a different topic or search.</p><button type="button" onClick={() => setParams({},{replace:true})}>Clear filters</button></div>}</div></div>
      </section>
    </>}
    <nav className="okb-footer-links" aria-label="Continue exploring"><Link to="/osu/beatmaps"><Target size={17}/>Find a beatmap<ArrowRight size={15}/></Link><Link to="/osu/replays"><Play size={17}/>Review a replay<ArrowRight size={15}/></Link><Link to="/learn">KovaaK's guides<ExternalLink size={14}/></Link></nav>
  </div>;
}
