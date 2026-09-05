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
    ctx.fillStyle = "rgba(0,0,0,0.65)"; ctx.beginPath(); ctx.arc(64, 64, 54, 0, Math.PI * 2); ctx.fill(); ring(ctx, 59, 4); ring(ctx, 50, 1, "#ffffff60");
  });
  await sprite("approachcircle", 128, 128, ctx => ring(ctx, 60, 3));
  await sprite("sliderb0", 128, 128, ctx => { ring(ctx, 53, 6); ring(ctx, 45, 2, "#ffffff80"); });
  await sprite("sliderfollowcircle", 256, 256, ctx => { ctx.translate(64, 64); ring(ctx, 106, 2, "#ffffff80"); });
  await sprite("cursor", 32, 32, ctx => {
    ctx.fillStyle = "#f5d867"; ctx.beginPath(); ctx.arc(16, 16, 9, 0, Math.PI * 2); ctx.fill();
    ctx.lineWidth = 2; ctx.strokeStyle = "#ffffff"; ctx.stroke();
  });
  await sprite("cursortrail", 16, 16, ctx => { ctx.fillStyle = "#f5d86799"; ctx.beginPath(); ctx.arc(8, 8, 4, 0, Math.PI * 2); ctx.fill(); });
  for (let i = 0; i <= 9; i++) {
    await sprite(`default-${i}`, 48, 64, ctx => { ctx.font = "600 55px Arial"; ctx.fillStyle = "#ffffff"; ctx.textAlign = "center"; ctx.textBaseline = "middle"; ctx.fillText(String(i), 24, 33); });
  }
  return {
    images, sounds: new Map(), spinnerImages: new Map(),
    config: { name: "AimMod", version: "2.7", comboColors: ["#69d9cc", "#f593be", "#e9c66a", "#82bdef"],
      hitCircleOverlap: 0, hitCirclePrefix: "default", scorePrefix: "score", comboPrefix: "score",
      sliderBorder: "#eff8f6", sliderTrackOverride: "#172523", allowSliderBallTint: false, maniaSections: [] },
  };
}

export function disposeAimModPlaybackSkin(skin: SkinAssets): void {
  for (const bitmap of new Set([...skin.images.values(), ...skin.spinnerImages.values()])) bitmap.close();
}
