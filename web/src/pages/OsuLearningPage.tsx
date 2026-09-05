import { Link, useParams } from "react-router-dom";
import { PageSeo } from "../components/PageSeo";
import content from "../../../api/internal/seo/content.json";

export function OsuLearningPage() {
  const { slug } = useParams();
  const guide = content.guides.find(item => item.slug === slug);
  const index = content.routes["/osu/learn"];
  if (slug && !guide) return <section className="py-8"><PageSeo title="Guide not found · AimMod Hub" description="This osu! guide is unavailable." noindex /><h1 className="text-2xl">Guide not found</h1><Link className="text-cyan" to="/osu/learn">osu! knowledge base</Link></section>;
  return <div className="mx-auto max-w-4xl py-5">
    <PageSeo title={guide ? `${guide.title} · AimMod Hub` : index.title} description={guide?.description ?? index.description} type={guide ? "article" : "website"}
      schema={guide ? { "@context": "https://schema.org", "@type": "Article", headline: guide.title, description: guide.description,
        mainEntityOfPage: `https://aimmod.app/osu/learn/${guide.slug}`, datePublished: content.updatedAt, dateModified: content.updatedAt,
        author: { "@type": "Organization", name: "AimMod Hub", url: "https://aimmod.app" },
        image: "https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png", citation: guide.sources.map(source => source.url) } : undefined} />
    <nav aria-label="Breadcrumb" className="mb-5 text-sm text-muted"><Link to="/osu" className="text-cyan">osu!</Link> / <Link to="/osu/learn" className="text-cyan">Knowledge base</Link></nav>
    <header className="border-b border-line pb-6"><h1 className="text-3xl leading-tight break-words">{guide?.title ?? "osu! knowledge base"}</h1><p className="mt-3 max-w-3xl text-muted leading-7">{guide?.description ?? index.description}</p></header>
    {guide ? <article>
      <p className="mt-4 text-xs text-muted">AimMod Hub · Updated {content.updatedAt}</p>
      {guide.sections.map(section => <section className="py-5" key={section.title}><h2 className="text-xl mb-3">{section.title}</h2><p className="text-muted leading-7">{section.body}</p></section>)}
      <section className="border-t border-line py-5"><h2 className="text-lg mb-3">Sources</h2><ul className="space-y-2">{guide.sources.map(source => <li key={source.url}><a className="text-cyan break-words hover:underline" href={source.url} rel="noreferrer">{source.title}</a></li>)}</ul></section>
      <nav aria-label="Related guides" className="border-t border-line py-5"><h2 className="text-lg mb-3">More osu! guides</h2><ul className="space-y-2">{content.guides.filter(item => item !== guide).map(item => <li key={item.slug}><Link className="text-cyan hover:underline" to={`/osu/learn/${item.slug}`}>{item.title}</Link></li>)}</ul></nav>
    </article> : <div className="divide-y divide-line">{content.guides.map(item => <article className="py-6" key={item.slug}><h2 className="text-xl"><Link className="text-cyan hover:underline" to={`/osu/learn/${item.slug}`}>{item.title}</Link></h2><p className="mt-2 text-muted leading-7">{item.description}</p></article>)}</div>}
    <nav className="border-t border-line py-5 flex flex-wrap gap-5 text-sm text-cyan"><Link to="/osu/beatmaps">Browse beatmaps</Link><Link to="/osu/replays">Review shared replays</Link><Link to="/learn">KovaaK's knowledge base</Link></nav>
  </div>;
}
