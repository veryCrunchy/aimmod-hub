import { Link, useParams } from "react-router-dom";
import { useState } from "react";
import { PageSeo } from "../components/PageSeo";
import content from "../../../api/internal/seo/content.json";
import { socialPreviewImage } from "../lib/socialPreview";

export function OsuLearningPage() {
  const { slug } = useParams();
  const [query, setQuery] = useState("");
  const [videosOnly, setVideosOnly] = useState(false);
  const isVideo = (url: string) => new URL(url).hostname === "www.youtube.com";
  const matching = content.guides.filter(item => (!videosOnly || item.sources.some(source => isVideo(source.url))) &&
    `${item.title} ${item.description} ${item.sections.map(section => section.body).join(" ")}`.toLowerCase().includes(query.trim().toLowerCase()));
  const guide = content.guides.find(item => item.slug === slug);
  const index = content.routes["/osu/learn"];
  if (slug && !guide) return <section className="py-8"><PageSeo title="Guide not found · AimMod Hub" description="This osu! guide is unavailable." noindex /><h1 className="text-2xl">Guide not found</h1><Link className="text-cyan" to="/osu/learn">osu! knowledge base</Link></section>;
  return <div className="mx-auto max-w-4xl py-5">
    <PageSeo title={guide ? `${guide.title} · AimMod Hub` : index.title} description={guide?.description ?? index.description} type={guide ? "article" : "website"}
      schema={guide ? { "@context": "https://schema.org", "@type": "Article", headline: guide.title, description: guide.description,
        mainEntityOfPage: `https://aimmod.app/osu/learn/${guide.slug}`, datePublished: content.updatedAt, dateModified: content.updatedAt,
        author: { "@type": "Organization", name: "AimMod Hub", url: "https://aimmod.app" },
        image: socialPreviewImage(`/osu/learn/${guide.slug}`), citation: guide.sources.map(source => source.url) } : undefined} />
    <nav aria-label="Breadcrumb" className="mb-5 text-sm text-muted"><Link to="/osu" className="text-cyan">osu!</Link> / <Link to="/osu/learn" className="text-cyan">Knowledge base</Link></nav>
    <header className="border-b border-line pb-6"><h1 className="text-3xl leading-tight break-words">{guide?.title ?? "osu! knowledge base"}</h1><p className="mt-3 max-w-3xl text-muted leading-7">{guide?.description ?? index.description}</p></header>
    {guide ? <article>
      <p className="mt-4 text-xs text-muted">AimMod Hub · Updated {content.updatedAt}</p>
      <nav aria-label="In this guide" className="border-b border-line py-5"><h2 className="text-sm mb-3">In this guide</h2><ol className="space-y-2">{guide.sections.map((section, index) => <li key={section.title}><a className="text-cyan hover:underline" href={`#section-${index + 1}`}>{section.title}</a></li>)}</ol></nav>
      {guide.sections.map((section, index) => <section id={`section-${index + 1}`} className="py-5 scroll-mt-24" key={section.title}><h2 className="text-xl mb-3">{section.title}</h2><p className="text-muted leading-7">{section.body}</p></section>)}
      <section className="border-t border-line py-5"><h2 className="text-lg mb-3">Sources and further learning</h2><ul className="space-y-4">{guide.sources.map(source => <li key={source.url}><span className="block text-xs text-muted mb-1">{isVideo(source.url) ? "Video · Community coaching" : "Reference"}</span><a className="text-cyan break-words hover:underline" href={source.url} rel="noreferrer">{source.title}</a></li>)}</ul></section>
      <nav aria-label="Related guides" className="border-t border-line py-5"><h2 className="text-lg mb-3">More osu! guides</h2><ul className="space-y-2">{content.guides.filter(item => item !== guide).map(item => <li key={item.slug}><Link className="text-cyan hover:underline" to={`/osu/learn/${item.slug}`}>{item.title}</Link></li>)}</ul></nav>
    </article> : <>
      <div className="flex flex-wrap gap-4 items-end py-5 border-b border-line">
        <label className="flex-1 min-w-0 basis-64"><span className="block text-sm mb-2">Find a guide</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Aim, streams, accuracy, practice..." className="w-full min-w-0 rounded border border-line bg-panel px-3 py-2" /></label>
        <label className="flex items-center gap-2 py-2 text-sm"><input type="checkbox" checked={videosOnly} onChange={event => setVideosOnly(event.target.checked)} />With video resources</label>
      </div>
      <p role="status" className="text-xs text-muted mt-4">{matching.length} guides</p>
      <div className="divide-y divide-line">{matching.map(item => <article className="py-6" key={item.slug}><h2 className="text-xl"><Link className="text-cyan hover:underline" to={`/osu/learn/${item.slug}`}>{item.title}</Link></h2><p className="mt-2 text-muted leading-7">{item.description}</p>{item.sources.some(source => isVideo(source.url)) && <p className="mt-2 text-xs text-muted">Exercises and video resources</p>}</article>)}</div>
      {!matching.length && <p className="py-8 text-muted">No guides match these filters.</p>}
    </>}
    <nav className="border-t border-line py-5 flex flex-wrap gap-5 text-sm text-cyan"><Link to="/osu/beatmaps">Browse beatmaps</Link><Link to="/osu/replays">Review shared replays</Link><Link to="/learn">KovaaK's knowledge base</Link></nav>
  </div>;
}
