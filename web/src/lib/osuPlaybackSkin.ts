import { buildSkin, loadSkin, type SkinAssets } from "replayviewer-js";
import { prepareSkinArchive } from "./osuSkinArchive";
import { API_BASE_URL } from "./config";
import { fetchPlaybackBytes } from "./osuPlayback";

// Original AimMod sprites. No third-party skin artwork or sounds are bundled.
export async function createAimModPlaybackSkin(): Promise<SkinAssets> {
  const images = new Map<string, ImageBitmap>();
  const sprite = async (name: string, width: number, height: number, draw: (ctx: OffscreenCanvasRenderingContext2D) => void) => {
    const density = name.endsWith("@2x") || name.startsWith("score-") ? 1 : 2;
    const canvas = new OffscreenCanvas(width * density, height * density);
    const ctx = canvas.getContext("2d")!;
    ctx.scale(density, density);
    draw(ctx);
    images.set(`${name}${density === 2 ? "@2x" : ""}.png`, canvas.transferToImageBitmap());
  };
  const ring = (ctx: OffscreenCanvasRenderingContext2D, radius: number, line: number, colour = "#ffffff") => {
    ctx.beginPath(); ctx.arc(64, 64, radius, 0, Math.PI * 2); ctx.lineWidth = line; ctx.strokeStyle = colour; ctx.stroke();
  };
  await sprite("hitcircle", 128, 128, ctx => { ctx.fillStyle = "#ffffff"; ctx.beginPath(); ctx.arc(64, 64, 59, 0, Math.PI * 2); ctx.fill(); });
  await sprite("hitcircleoverlay", 128, 128, ctx => {
    const shade = ctx.createRadialGradient(48, 40, 4, 64, 64, 56);
    shade.addColorStop(0, "rgba(255,255,255,0.25)"); shade.addColorStop(1, "rgba(5,10,16,0.62)");
    ctx.fillStyle = shade; ctx.beginPath(); ctx.arc(64, 64, 54, 0, Math.PI * 2); ctx.fill();
    ring(ctx, 59, 4); ring(ctx, 54, 1, "#ffffff70");
  });
  await sprite("approachcircle", 128, 128, ctx => { ring(ctx, 60, 5, "#07101990"); ring(ctx, 60, 3); });
  await sprite("sliderb0", 128, 128, ctx => {
    ctx.fillStyle = "#a9eee524"; ctx.beginPath(); ctx.arc(64, 64, 50, 0, Math.PI * 2); ctx.fill();
    ring(ctx, 53, 6); ring(ctx, 47, 2, "#17242ccc"); ring(ctx, 45, 2, "#ffffffb0");
  });
  await sprite("sliderfollowcircle", 256, 256, ctx => { ctx.translate(64, 64); ring(ctx, 106, 2, "#ffffff80"); });
  await sprite("cursor", 32, 32, ctx => {
    ctx.fillStyle = "#f5d867"; ctx.beginPath(); ctx.arc(16, 16, 9, 0, Math.PI * 2); ctx.fill();
    ctx.lineWidth = 2; ctx.strokeStyle = "#0a121c"; ctx.stroke();
    ctx.beginPath(); ctx.arc(16, 16, 7, 0, Math.PI * 2); ctx.lineWidth = 1; ctx.strokeStyle = "#ffffff"; ctx.stroke();
    ctx.fillStyle = "#ffffff"; ctx.beginPath(); ctx.arc(16, 16, 2, 0, Math.PI * 2); ctx.fill();
  });
  await sprite("cursortrail", 16, 16, ctx => { ctx.fillStyle = "#f5d86799"; ctx.beginPath(); ctx.arc(8, 8, 4, 0, Math.PI * 2); ctx.fill(); });
  for (let i = 0; i <= 9; i++) {
    await sprite(`default-${i}`, 48, 64, ctx => { ctx.font = "600 55px Arial"; ctx.fillStyle = "#ffffff"; ctx.textAlign = "center"; ctx.textBaseline = "middle"; ctx.fillText(String(i), 24, 33); });
  }
  // HUD glyphs scale by aspect ratio; punctuation needs the same baseline and
  // full canvas height as digits. Supply both densities to avoid font fallback.
  const glyphs = [
    ...Array.from({ length: 10 }, (_, i) => ({ suffix: String(i), text: String(i), width: 30 })),
    { suffix: "dot", text: ".", width: 12 },
    { suffix: "percent", text: "%", width: 40 },
    { suffix: "x", text: "x", width: 26 },
    { suffix: "comma", text: ",", width: 12 },
  ];
  for (const { suffix, text, width } of glyphs) {
    for (const density of [1, 2]) {
      await sprite(`score-${suffix}${density === 2 ? "@2x" : ""}`, width * density, 40 * density, ctx => {
        ctx.scale(density, density);
        ctx.font = "700 38px Arial, sans-serif";
        ctx.textAlign = "center"; ctx.textBaseline = "alphabetic";
        ctx.lineJoin = "round"; ctx.lineWidth = 2.5; ctx.strokeStyle = "#071019";
        ctx.strokeText(text, width / 2, 33, width - 4);
        ctx.fillStyle = "#f5fbff"; ctx.fillText(text, width / 2, 33, width - 4);
      });
      if (/^\d$/.test(suffix)) {
        images.set(`scoreentry-${suffix}${density === 2 ? "@2x" : ""}.png`, images.get(`score-${suffix}${density === 2 ? "@2x" : ""}.png`)!);
      }
    }
  }
  // Keep the renderer's native 46px key geometry; its pressed-state tint is
  // multiplied onto this border while the counter is drawn independently.
  await sprite("inputoverlay-key@2x", 92, 92, ctx => {
    ctx.scale(2, 2);
    ctx.beginPath(); ctx.roundRect(2, 2, 42, 42, 5);
    ctx.fillStyle = "#15232d"; ctx.fill();
    ctx.lineWidth = 2; ctx.strokeStyle = "#d7f5ef"; ctx.stroke();
    ctx.beginPath(); ctx.roundRect(5, 5, 36, 36, 3);
    ctx.lineWidth = 1; ctx.strokeStyle = "#ffffff24"; ctx.stroke();
  });
  await sprite("reversearrow", 128, 128, ctx => {
    ctx.beginPath(); ctx.moveTo(35, 64); ctx.lineTo(69, 33); ctx.lineTo(69, 51); ctx.lineTo(96, 51); ctx.lineTo(96, 77); ctx.lineTo(69, 77); ctx.lineTo(69, 95); ctx.closePath();
    ctx.lineWidth = 5; ctx.lineJoin = "round"; ctx.strokeStyle = "#20242b"; ctx.stroke(); ctx.fillStyle = "white"; ctx.fill();
  });
  await sprite("followpoint", 16, 16, ctx => {
    const glow = ctx.createRadialGradient(8, 8, 1, 8, 8, 7); glow.addColorStop(0, "#fff"); glow.addColorStop(1, "#ffffff00");
    ctx.fillStyle = glow; ctx.fillRect(0, 0, 16, 16);
  });
  await sprite("sliderscorepoint", 16, 16, ctx => { ctx.fillStyle = "white"; ctx.beginPath(); ctx.arc(8, 8, 4, 0, Math.PI * 2); ctx.fill(); });
  for (const [value, colour] of [["300", "#77dfff"], ["100", "#8cff78"], ["50", "#ffce68"], ["0", "#ff577c"]]) {
    await sprite(`hit${value}`, 110, 56, ctx => {
      ctx.font = "italic 800 42px Arial"; ctx.textAlign = "center"; ctx.textBaseline = "middle";
      ctx.lineWidth = 4; ctx.strokeStyle = "#10131c"; ctx.strokeText(value === "0" ? "X" : value, 55, 28);
      ctx.fillStyle = colour; ctx.fillText(value === "0" ? "X" : value, 55, 28);
    });
  }
  await sprite("spinner-circle", 460, 460, ctx => {
    ctx.translate(230, 230);
    const shade = ctx.createRadialGradient(0, 0, 20, 0, 0, 220); shade.addColorStop(0, "#202b38dd"); shade.addColorStop(.85, "#50677c88"); shade.addColorStop(1, "#ffffff33");
    ctx.fillStyle = shade; ctx.beginPath(); ctx.arc(0, 0, 216, 0, Math.PI * 2); ctx.fill();
    for (const radius of [214, 190, 52]) { ctx.beginPath(); ctx.arc(0, 0, radius, 0, Math.PI * 2); ctx.lineWidth = 3; ctx.strokeStyle = "#ffffffbb"; ctx.stroke(); }
    for (let i = 0; i < 12; i++) { ctx.rotate(Math.PI / 6); ctx.fillStyle = "#ffffff99"; ctx.fillRect(-2, 164, 4, 21); }
  });
  await sprite("spinner-approachcircle", 460, 460, ctx => { ctx.beginPath(); ctx.arc(230, 230, 220, 0, Math.PI * 2); ctx.lineWidth = 6; ctx.strokeStyle = "#b7e6ff"; ctx.stroke(); });
  for (const [name, text] of [["spinner-spin", "SPIN!"], ["spinner-clear", "CLEAR!"]]) {
    await sprite(name, 260, 64, ctx => { ctx.font = "italic 800 50px Arial"; ctx.textAlign = "center"; ctx.fillStyle = "white"; ctx.fillText(text, 130, 50); });
  }
  return buildSkin({
    images, sounds: new Map(), spinnerImages: new Map(),
    config: { name: "AimMod Classic", version: "2.7", comboColors: ["#69d9cc", "#f593be", "#e9c66a", "#82bdef"],
      hitCircleOverlap: 0, hitCirclePrefix: "default", scorePrefix: "score", comboPrefix: "score",
      sliderBorder: "#eff8f6", sliderTrackOverride: "#10151d", allowSliderBallTint: false, maniaSections: [] },
  });
}

export function disposeAimModPlaybackSkin(skin: SkinAssets): void {
  for (const bitmap of new Set([...skin.images.values(), ...skin.spinnerImages.values()])) bitmap.close();
}


export const playbackSkins = [
  { id: "yugen", name: "YUGEN", creator: "Garin", source: "https://osu.ppy.sh/community/forums/topics/365036" },
  { id: "whitecat", name: "WhiteCat 1.0 NM", creator: "cyperdark", source: "https://osu.ppy.sh/community/forums/topics/986201" },
  { id: "rafis", name: "Rafis HDDT 2018", creator: "DDK RPK / Rafis", source: "https://gist.github.com/thomazgg/5fbaf92bed0eac290a7123f5b308dcb0" },
  { id: "classic", name: "AimMod Classic", creator: "AimMod", source: "" },
] as const;

export function savedPlaybackSkin(): string {
  try { const id = localStorage.getItem("osu-replay-skin"); if (playbackSkins.some(skin => skin.id === id)) return id!; } catch { /* Storage can be disabled. */ }
  return "yugen";
}

export async function loadPlaybackSkin(id: string, audio: AudioContext, signal: AbortSignal, file?: File): Promise<SkinAssets | null> {
  if (id === "classic") return null;
  if (!file && !playbackSkins.some(skin => skin.id === id)) throw new Error("Choose a replay skin.");
  if (file && file.size > 64 * 1024 * 1024) throw new Error("Choose a skin smaller than 64 MB.");
  const bytes = file ? await file.arrayBuffer() : await fetchPlaybackBytes(`${API_BASE_URL}/api/osu/v1/playback/skins/${id}`, 32 * 1024 * 1024, AbortSignal.any([signal, AbortSignal.timeout(65000)]));
  signal.throwIfAborted();
  const prepared = await prepareSkinArchive(bytes);
  signal.throwIfAborted();
  const skin = await loadSkin(prepared, audio);
  if (signal.aborted) { disposeAimModPlaybackSkin(skin); signal.throwIfAborted(); }
  if (![...skin.images.keys()].some(name => /^(hitcircle|cursor).*\.png$/.test(name))) {
    disposeAimModPlaybackSkin(skin);
    throw new Error("This skin's gameplay artwork could not be decoded.");
  }
  return skin;
}

export function composePlaybackSkin(base: SkinAssets, overlay: SkinAssets | null): SkinAssets {
  if (!overlay) return base;
  const skin = buildSkin(base, overlay);
  // A skin with no spinner artwork gets the classic spinner. Explicit 1px
  // sprites still suppress individual elements, as they do in osu!.
  if (skin.spinnerImages.size === 0) skin.spinnerImages = base.spinnerImages;
  return skin;
}
