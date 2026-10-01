# Social Preview Assets

- `aimmod-256.png`: existing AimMod v9 brand export, copied unchanged from `web/public/brand/aimmod-v9/aimmod-256.png`.
- `NotoSansJP-Regular.otf`: unmodified static Noto Sans JP Regular from the official Noto CJK repository: https://raw.githubusercontent.com/notofonts/noto-cjk/main/Sans/SubsetOTF/JP/NotoSansJP-Regular.otf
- `OFL.txt`: font license from https://raw.githubusercontent.com/notofonts/noto-cjk/main/Sans/LICENSE. The font retains its embedded copyright/name metadata.

The renderer embeds these assets at build time. It never downloads fonts or images during a request. Latin, Greek and Cyrillic text uses the Go fonts in `golang.org/x/image/font/gofont`; Japanese characters fall back per glyph to Noto Sans JP. Scripts or emoji absent from both fonts are represented by an ellipsis, not transliterated.

## Discord invite cards (`/og/invite.png`)

- `Roboto-Bold.ttf`: unmodified Roboto Bold 3.009 (Roboto Classic, The Roboto Project Authors), SIL Open Font License 1.1, see `Roboto-OFL.txt`. Roboto is the typeface of AimMod's in-game UI.
- `Roboto-Medium.ttf`: unmodified Roboto Medium 2.137 (Google), Apache License 2.0, see `Roboto-Medium-LICENSE.txt`.
- `aimmod-horizontal.png` (mint mark, chalk wordmark) and `aimmod-wordmark-black.png`: the official lockups from `web/public/brand/aimmod-kit/horizontal.svg` and `wordmark-black.svg`, rasterised at 4x and scaled to 220 px and 120 px high. The cards never set "AimMod" in a typeface.

The cards use Roboto Bold for titles, labels and the player count (Roboto's figures are tabular), Medium for the map name and host, and tighter tracking on large text. The Workshop preview fetch (`ws=`) is the one network use; Japanese characters fall back to Noto Sans JP.
