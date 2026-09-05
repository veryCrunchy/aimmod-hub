import type { SkinAssets } from "replayviewer-js";

// Original AimMod sprites. No third-party skin artwork or sounds are bundled.
export async function createAimModPlaybackSkin(): Promise<SkinAssets> {
  const images = new Map<string, ImageBitmap>();
  const sprite = async (name: string, width: number, height: number, draw: (ctx: OffscreenCanvasRenderingContext2D) => void) => {
    const canvas = new OffscreenCanvas(width, height);
    const ctx = canvas.getContext("2d")!;
    draw(ctx);
    images.set(`${name}.png`, canvas.transferToImageBitmap());
  };
  const ring = (ctx: OffscreenCanvasRenderingContext2D, radius: number, line: number, colour = "#ffffff") => {
    ctx.beginPath(); ctx.arc(64, 64, radius, 0, Math.PI * 2); ctx.lineWidth = line; ctx.strokeStyle = colour; ctx.stroke();
  };
  await sprite("hitcircle", 128, 128, ctx => { ctx.fillStyle = "#ffffff"; ctx.beginPath(); ctx.arc(64, 64, 59, 0, Math.PI * 2); ctx.fill(); });
  await sprite("hitcircleoverlay", 128, 128, ctx => {
    const shade = ctx.createRadialGradient(48, 40, 4, 64, 64, 56);
    shade.addColorStop(0, "rgba(10,18,24,0.64)"); shade.addColorStop(1, "rgba(5,10,16,0.86)");
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
  return {
    images, sounds: new Map(), spinnerImages: new Map(),
    config: { name: "AimMod", version: "2.7", comboColors: ["#69d9cc", "#f593be", "#e9c66a", "#82bdef"],
      hitCircleOverlap: 0, hitCirclePrefix: "default", scorePrefix: "score", comboPrefix: "score",
      sliderBorder: "#eff8f6", sliderTrackOverride: "#174943", allowSliderBallTint: false, maniaSections: [] },
  };
}

export function disposeAimModPlaybackSkin(skin: SkinAssets): void {
  for (const bitmap of new Set([...skin.images.values(), ...skin.spinnerImages.values()])) bitmap.close();
}
