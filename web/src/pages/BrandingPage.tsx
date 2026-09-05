import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Check, Copy, Download } from "lucide-react";
import { PageSeo } from "../components/PageSeo";
import { brandAssets, brandColours, brandRoot, iconSizes } from "../lib/brandAssets";

function DownloadLink({ file, children }: { file: string; children: React.ReactNode }) {
  return <a className="brand-download" href={`${brandRoot}/${file}`} download><Download size={15} aria-hidden="true" />{children}</a>;
}
export function BrandingPage() {
  const [params, setParams] = useSearchParams();
  const categories = ["All assets", "Logos", "Marks", "Wordmarks", "Social & artwork"];
  const category = categories.includes(params.get("category") ?? "") ? params.get("category")! : "All assets";
  const update = (key: string, value: string) => setParams(previous => { const next = new URLSearchParams(previous); next.set(key, value); return next; }, { replace: true });
  const setCategory = (value: string) => update("category", value);
  const [copied, setCopied] = useState("");
  const [copyError, setCopyError] = useState("");
  const requestedSize = Number(params.get("iconSize") ?? 256);
  const iconSize = iconSizes.includes(requestedSize) ? requestedSize : 256;
  const setIconSize = (value: number) => update("iconSize", String(value));
  async function copy(hex: string) {
    try { await navigator.clipboard.writeText(hex); setCopied(hex); setCopyError(""); }
    catch { setCopyError("Clipboard access is unavailable. Select the colour code to copy it."); }
  }
  return <div className="brand-page">
    <PageSeo title="AimMod Branding & Assets" description="Download AimMod logos, icons, banners and social artwork. Brand colours, clear space and logo usage guidelines." />
    <header className="brand-header">
      <div><p className="brand-eyebrow">Brand resources</p><h1>AimMod branding</h1><p>Logos, artwork and the details that keep AimMod recognisable.</p></div>
      <a className="brand-download brand-download-primary" href="/brand/aimmod-brand-kit.zip" download><Download size={18} aria-hidden="true" />Download brand kit <span>ZIP</span></a>
    </header>
    <div className="brand-signature"><img src={`${brandRoot}/horizontal.svg`} alt="AimMod" /></div>
    <nav className="brand-anchor-nav" aria-label="Brand resources"><a href="#assets">Assets</a><a href="#icons">App icons</a><a href="#colours">Colours</a><a href="#usage">Usage</a></nav>
    <section id="assets"><div className="brand-section-heading"><h2>Logo & artwork library</h2><span>SVG and PNG downloads</span></div>
      <div className="brand-tabs" role="group" aria-label="Asset category">{["All assets", "Logos", "Marks", "Wordmarks", "Social & artwork"].map(item => <button key={item} aria-pressed={category === item} onClick={() => setCategory(item)}>{item}</button>)}</div>
      <div className="brand-grid">{brandAssets.filter(asset => category === "All assets" || asset.category === category).map(asset => <article key={asset.file} className="brand-asset">
        <div className={`brand-asset-preview${"light" in asset ? " brand-light" : ""}${"wide" in asset ? " brand-wide" : ""}`}><img src={`${brandRoot}/${asset.file}.${"wide" in asset ? "png" : "svg"}`} alt={asset.name} loading="lazy" /></div>
        <div className="brand-asset-info"><h3>{asset.name}</h3><p>{asset.description}</p><div className="brand-asset-downloads"><DownloadLink file={`${asset.file}.svg`}>SVG</DownloadLink><DownloadLink file={`${asset.file}.png`}>PNG</DownloadLink></div></div>
      </article>)}</div>
    </section>
    <section id="icons" className="brand-icon-section"><div><h2>App icons</h2><p>Use the icon exports at small sizes to preserve the shape of the mark.</p><div className="brand-icon-actions"><label>PNG size<select value={iconSize} onChange={event => setIconSize(Number(event.target.value))}>{iconSizes.map(size => <option key={size} value={size}>{size} × {size}</option>)}</select></label><DownloadLink file={`icons/aimmod-${iconSize}.png`}>PNG</DownloadLink><DownloadLink file="icons/aimmod.ico">Windows ICO</DownloadLink><DownloadLink file="app-icon.svg">SVG</DownloadLink></div></div><img src={`${brandRoot}/icons/aimmod-256.png`} width="128" height="128" alt="AimMod app icon" /></section>
    <section id="colours"><h2>Brand colours</h2><div className="brand-colours">{brandColours.map(colour => <div key={colour.hex}><button style={{ backgroundColor: colour.hex }} onClick={() => void copy(colour.hex)} aria-label={`Copy ${colour.name} ${colour.hex}`} title={`Copy ${colour.hex}`}>{copied === colour.hex ? <Check size={20} /> : <Copy size={18} />}</button><strong>{colour.name}</strong><code>{colour.hex}</code></div>)}</div><p role="status" className="brand-copy-status">{copyError || (copied ? `${copied} copied` : "")}</p></section>
    <section id="usage"><h2>Using the brand</h2><div className="brand-guidelines">
      <div><h3>Keep the name intact</h3><p>Write AimMod with a capital A and M. Use the supplied wordmark artwork rather than retyping or reconstructing the lettering.</p></div>
      <div><h3>Give it room</h3><p>Leave at least one stem width of clear space around the mark: approximately 50 units on the 320 × 220 master canvas. Keep text and other logos outside that space.</p></div>
      <div><h3>Choose a readable size</h3><p>Keep wordmarks at least 220 px wide and Japanese lockups at least 480 px wide. For smaller placements, use the standalone mark or an app icon.</p></div>
      <div><h3>Keep strong contrast</h3><p>Use mint or white on dark backgrounds, and black on light backgrounds. Avoid busy artwork behind the logo.</p></div>
      <div><h3>Preserve the artwork</h3><p>Do not stretch, rotate, recolour, add effects or alter the cutouts and letter spacing. Scale proportionally and keep the complete logo visible.</p></div>
      <div><h3>Represent AimMod clearly</h3><p>Use these assets when referring to AimMod. Do not imply an official partnership or endorsement, or use the brand to impersonate the project. Keep third-party game branding separate.</p></div>
    </div><DownloadLink file="usage.md">Download usage guide</DownloadLink></section>
  </div>;
}
