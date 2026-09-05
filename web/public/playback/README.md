# AimMod Playback Assets

The AimMod playback skin is original artwork drawn by `src/lib/osuPlaybackSkin.ts`.
The twelve percussion samples are original synthesized waveforms reproducible with
`node scripts/generate-playback-sounds.mjs`. These generated samples and skin artwork
are available under the MIT license. No YUGEN, osu! default-skin artwork, music,
or third-party hitsound recordings are redistributed here.

Playback engine: replayviewer-js 0.1.1, MIT,
https://github.com/daladal/replayviewer-js . The library performs replay and beatmap
parsing, slider paths, mod processing, stacking, input judgements and rendering.
The browser reconstruction is not the native osu! client; uploaded judgement
analysis remains authoritative for coaching.
