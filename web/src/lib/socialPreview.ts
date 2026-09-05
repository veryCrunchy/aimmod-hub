import content from "../../../api/internal/seo/content.json";

export const brandPreviewImage = "https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png";

export function socialPreviewImage(pathname: string, noindex = false): string {
  const route = pathname.split(/[?#]/, 1)[0].replace(/\/+$/, "") || "/";
  if (noindex || ["/", "/app", "/search"].includes(route)) return brandPreviewImage;
  const published = Object.hasOwn(content.routes, route);
  const detail = /^\/(?:osu\/(?:learn|replays|profiles)|learn(?:\/topics)?|profiles|runs|scenarios)\/[^/]+$/.test(route);
  return published || detail
    ? `https://aimmod.app/social-preview.png?path=${encodeURIComponent(route)}&v=1`
    : brandPreviewImage;
}
