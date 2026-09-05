export const brandRoot = "/brand/aimmod-kit";
export const brandAssets = [
  { name: "Primary logo", file: "horizontal", category: "Logos", description: "Mint mark with chalk lettering. The primary lockup for dark backgrounds." },
  { name: "Mint logo", file: "horizontal-mint", category: "Logos", description: "A single-colour lockup for dark backgrounds." },
  { name: "White logo", file: "horizontal-white", category: "Logos", description: "For dark backgrounds where colour is limited." },
  { name: "Black logo", file: "horizontal-black", category: "Logos", description: "For white and light backgrounds.", light: true },
  { name: "Japanese lockup", file: "horizontal-japanese", category: "Logos", description: "The extended lockup. Use at 480 px wide or larger." },
  { name: "Mint mark", file: "mark-mint", category: "Marks", description: "The standalone AimMod monogram for compact spaces." },
  { name: "White mark", file: "mark-white", category: "Marks", description: "A monochrome mark for dark backgrounds." },
  { name: "Black mark", file: "mark-black", category: "Marks", description: "A monochrome mark for light backgrounds.", light: true },
  { name: "Mint wordmark", file: "wordmark-mint", category: "Wordmarks", description: "Custom AimMod lettering in mint." },
  { name: "White wordmark", file: "wordmark-white", category: "Wordmarks", description: "Custom AimMod lettering in white." },
  { name: "Black wordmark", file: "wordmark-black", category: "Wordmarks", description: "Custom AimMod lettering in black.", light: true },
  { name: "Avatar", file: "avatar", category: "Social & artwork", description: "Square profile artwork." },
  { name: "Wide banner", file: "banner-wide-2560x640", category: "Social & artwork", description: "2560 × 640 · wide headers", wide: true },
  { name: "Compact banner", file: "banner-compact-1200x400", category: "Social & artwork", description: "1200 × 400 · compact headers", wide: true },
  { name: "Social header", file: "social-header-1500x500", category: "Social & artwork", description: "1500 × 500 · social profiles", wide: true },
  { name: "Repository cover", file: "repository-cover-1280x640", category: "Social & artwork", description: "1280 × 640 · project covers", wide: true },
  { name: "Share card", file: "share-card-1200x630", category: "Social & artwork", description: "1200 × 630 · shared links", wide: true },
  { name: "Splash", file: "splash-1920x1080", category: "Social & artwork", description: "1920 × 1080 · presentation artwork", wide: true },
  { name: "Wallpaper", file: "wallpaper-3840x2160", category: "Social & artwork", description: "3840 × 2160 · desktop wallpaper", wide: true },
] as const;
export const brandColours = [
  { name: "Mint", hex: "#27E4A1" }, { name: "Ink", hex: "#040D09" },
  { name: "Chalk", hex: "#C8F3E0" }, { name: "Deep green", hex: "#0D7350" },
  { name: "Sage", hex: "#77BD98" },
];
export const iconSizes = [16, 24, 32, 48, 64, 128, 256, 512, 1024];
