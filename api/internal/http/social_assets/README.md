# Social Preview Assets

- `aimmod-256.png`: existing AimMod v9 brand export, copied unchanged from `web/public/brand/aimmod-v9/aimmod-256.png`.
- `NotoSansJP-Regular.otf`: unmodified static Noto Sans JP Regular from the official Noto CJK repository: https://raw.githubusercontent.com/notofonts/noto-cjk/main/Sans/SubsetOTF/JP/NotoSansJP-Regular.otf
- `OFL.txt`: font license from https://raw.githubusercontent.com/notofonts/noto-cjk/main/Sans/LICENSE. The font retains its embedded copyright/name metadata.

The renderer embeds these assets at build time. It never downloads fonts or images during a request. Latin, Greek and Cyrillic text uses the Go fonts in `golang.org/x/image/font/gofont`; Japanese characters fall back per glyph to Noto Sans JP. Scripts or emoji absent from both fonts are represented by an ellipsis, not transliterated.
