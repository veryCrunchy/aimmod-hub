# AimMod Playback Assets

The AimMod playback skin is original artwork drawn by `src/lib/osuPlaybackSkin.ts`.
The twelve percussion samples are original synthesized waveforms reproducible with
`node scripts/generate-playback-sounds.mjs`. These generated samples and skin artwork
are available under the MIT license. No YUGEN, osu! default-skin artwork, music,
or third-party hitsound recordings are redistributed here.

The replay picker also offers YUGEN by Garin, WhiteCat 1.0 NM by cyperdark,
and Rafis HDDT 2018 (DDK RPK / Rafis). These are separate third-party works,
not covered by the license of AimMod's generated assets. Their source pages are:

- https://osu.ppy.sh/community/forums/topics/365036
- https://osu.ppy.sh/community/forums/topics/986201
- https://gist.github.com/thomazgg/5fbaf92bed0eac290a7123f5b308dcb0

The API fetches checksum-pinned releases on demand, retaining original gameplay
asset bytes and font configuration while excluding menu artwork and unrelated
files. No third-party skin archives are committed to this repository. YUGEN uses
the author's linked download; WhiteCat and Rafis use a commit-pinned public mirror
at https://github.com/praeludiumOrbis/whitecat-skins . The allowlist and release
checksums live in `api/internal/http/osu_playback_skins.go`. A changed upstream
release fails closed; updating a preset requires reviewing its source and assets.
Custom `.osk` imports are processed in the browser and are never uploaded.

Playback engine: replayviewer-js 0.1.1, MIT,
https://github.com/daladal/replayviewer-js . The library performs replay and beatmap
parsing, slider paths, mod processing, stacking, input judgements and rendering.
The browser reconstruction is not the native osu! client; uploaded judgement
analysis remains authoritative for coaching.
