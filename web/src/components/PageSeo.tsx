import { useLocation } from "react-router-dom";
import { Helmet } from "../lib/helmet";
import content from "../../../api/internal/seo/content.json";

export function PageSeo({ title, description, type = "website", noindex = false, schema }: {
  title: string; description: string; type?: string; noindex?: boolean; schema?: Record<string, unknown>;
}) {
  const { pathname } = useLocation();
  const canonical = `https://aimmod.app${pathname.replace(/\/$/, "") || "/"}`;
  return <Helmet>
    <title>{title}</title><meta name="description" content={description} />
    <link rel="canonical" href={canonical} />
    <meta name="robots" content={noindex ? "noindex, nofollow" : "index, follow"} />
    <meta property="og:title" content={title} /><meta property="og:description" content={description} />
    <meta property="og:type" content={type} /><meta property="og:url" content={canonical} />
    <meta property="og:site_name" content="AimMod Hub" />
    <meta property="og:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
    <meta property="og:image:width" content="1200" /><meta property="og:image:height" content="630" />
    <meta property="og:image:type" content="image/png" /><meta property="og:image:alt" content="AimMod Hub" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content={title} /><meta name="twitter:description" content={description} />
    <meta name="twitter:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
    <meta name="twitter:image:alt" content="AimMod Hub" />
    {schema && !noindex ? <script type="application/ld+json">{JSON.stringify(schema).replace(/</g, "\\u003c")}</script> : null}
  </Helmet>;
}

export function RouteSeo() {
  const { pathname } = useLocation();
  const route = pathname.replace(/\/$/, "") || "/";
  const page = (content.routes as Record<string, { title: string; description: string }>)[route];
  // These pages own their complete head, including loading/error visibility states.
  if (/^\/(learn(?:\/|$)|osu\/(learn(?:\/|$)|(?:replays|profiles)\/))/.test(route)) return null;
  const restricted = /^\/(admin(?:\/|$)|account(?:\/|$)|link-device(?:\/|$)|search(?:\/|$))/.test(route);
  const dynamic = /^\/(profiles|runs|scenarios|u|benchmarks)\//.test(route);
  const existingTitle = Boolean(page && route !== "/benchmarks") || dynamic;
  if (!existingTitle) return <PageSeo title={page?.title ?? "AimMod Hub"} description={page?.description ?? "AimMod analysis, shared practice data and coaching guides."} noindex={restricted || !page} />;
  const existingDescription = !["/osu", "/osu/players", "/osu/beatmaps", "/osu/skins"].includes(route) && !/^\/(u|benchmarks)\//.test(route);
  const existingSocial = /^\/(profiles|runs|scenarios)\//.test(route) || route === "/app/osu" || route === "/app/kovaaks";
  return <Helmet>
    <link rel="canonical" href={`https://aimmod.app${route}`} /><meta name="robots" content={restricted ? "noindex, nofollow" : "index, follow"} />
    {!existingDescription && <meta name="description" content={page?.description ?? "Public practice results and comparisons on AimMod Hub."} />}
    {!existingSocial && page && <meta property="og:title" content={page.title} />}
    {!existingSocial && page && <meta property="og:description" content={page.description} />}
    <meta property="og:url" content={`https://aimmod.app${route}`} />
    <meta property="og:site_name" content="AimMod Hub" />
    <meta property="og:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
  </Helmet>;
}
