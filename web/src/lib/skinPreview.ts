/** Eight-second synthetic spinner, using lazer's legacy layer ratios and progress effects. */
export function spinnerPreviewState(elapsedSeconds: number) {
  const elapsed = Math.max(0, elapsedSeconds) % 8;
  const rpm = 240 + 35 * Math.sin(elapsed * 1.6);
  const spins = 4 * elapsed + 35 / 96 * (1 - Math.cos(elapsed * 1.6));
  const progress = Math.min(1, spins / 24);
  const bonusPhase = spins - Math.floor(spins);
  return { rpm: Math.floor(rpm), rotation: spins * Math.PI * 2, progress,
    scale: .8 + (1 - (1 - progress) ** 2) * .2,
    approachScale: 1.86 + (.1 - 1.86) * elapsed / 8,
    cleared: progress >= 1,
    flash: progress >= 1 ? Math.max(0, 1 - bonusPhase / .8) : 0,
    promptAlpha: Math.max(0, 1 - spins * 2),
  };
}

export function sliderPreviewPosition(progress: number): [number, number] {
  const t = Math.min(1, Math.max(0, progress)), u = 1 - t;
  return [u ** 3 * 285 + 3 * u * u * t * 370 + 3 * u * t * t * 465 + t ** 3 * 560,
    u ** 3 * 280 + 3 * u * u * t * 225 + 3 * u * t * t * 320 + t ** 3 * 235];
}
