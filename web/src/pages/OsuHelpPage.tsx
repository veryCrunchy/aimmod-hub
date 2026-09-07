import { useEffect, useState } from "react";
import { Link, NavLink, useParams } from "react-router-dom";
import { PageSeo } from "../components/PageSeo";
import { desktopGuides, searchDesktopGuides } from "../lib/desktopGuides";


export function OsuHelpPage() {
  const { slug } = useParams();
  const [query, setQuery] = useState("");
  const guide = desktopGuides.find(item => item.slug === slug);
  const results = searchDesktopGuides(query);
  const index = guide ? desktopGuides.indexOf(guide) : -1;
  useEffect(() => { setQuery(""); window.scrollTo(0, 0); }, [slug]);
  return <div className="desktop-help">
    <PageSeo title={`${guide?.title ?? (slug ? "Guide not found" : "AimMod app guide")} · AimMod`} description={guide?.summary ?? "Learn to set up AimMod, review osu! replays, find PP targets, create practice sets, and track your progress."} noindex={Boolean(slug && !guide)} />
    <header className="desktop-help-header">
      <div><Link className="desktop-help-eyebrow" to="/osu/help">AIMMOD APP GUIDE</Link><h1>{guide?.title ?? (slug ? "Guide not found" : "Make your next session count")}</h1><p>{guide?.summary ?? "Set up your app, understand a play, and turn it into focused practice."}</p></div>
      <Link className="desktop-help-download" to="/app/osu">Get AimMod ↗</Link>
    </header>
    <div className="desktop-help-layout">
      <aside className="desktop-help-nav">
        <label htmlFor="guide-search">Find a guide</label>
        <input id="guide-search" type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Try accuracy or cleanup" />
        <nav aria-label="App guide topics"><NavLink to="/osu/help" end>All guides</NavLink>{results.map(item => <NavLink key={item.slug} to={`/osu/help/${item.slug}`}>{item.title}</NavLink>)}</nav>
        {results.length === 0 && <p role="status">No matching guides. Try a shorter search.</p>}
      </aside>
      <div className="desktop-help-content">
        {guide ? <article>
          <p className="desktop-help-location">IN THE APP <span>{guide.location}</span></p>
          <nav className="desktop-help-contents" aria-label="On this page"><strong>On this page</strong>{guide.sections.map((section, i) => <a href={`#section-${i + 1}`} key={section.title}>{section.title}</a>)}</nav>
          {guide.sections.map((section, i) => <section className="desktop-help-section" id={`section-${i + 1}`} key={section.title}><h2>{section.title}</h2>{section.paragraphs?.map(paragraph => <p key={paragraph}>{paragraph}</p>)}{section.steps && <ol>{section.steps.map(step => <li key={step}>{step}</li>)}</ol>}</section>)}
          <nav className="desktop-help-pagination" aria-label="Continue reading">{index > 0 && <Link to={`/osu/help/${desktopGuides[index - 1].slug}`}>← {desktopGuides[index - 1].title}</Link>}{index < desktopGuides.length - 1 && <Link to={`/osu/help/${desktopGuides[index + 1].slug}`}>{desktopGuides[index + 1].title} →</Link>}</nav>
        </article> : slug ? <p>This guide could not be found. <Link to="/osu/help">Browse all guides.</Link></p> : <>
          <div className="desktop-help-start"><h2>New here? Start with one map.</h2><p>Connect your osu! library, choose a play you want to improve, and follow its progress in Coaching.</p><Link to="/osu/help/getting-started">Set up AimMod →</Link></div>
          <div className="desktop-help-grid">{results.map((item, i) => <Link className="desktop-help-card" to={`/osu/help/${item.slug}`} key={item.slug}><span>{String(i + 1).padStart(2, "0")}</span><h2>{item.title}</h2><p>{item.summary}</p><small>{item.location}</small></Link>)}</div>
        </>}
        <p className="desktop-help-support">Still stuck? <a href="/join" target="_blank" rel="noopener noreferrer">Ask in the AimMod Discord ↗</a></p>
      </div>
    </div>
  </div>;
}
