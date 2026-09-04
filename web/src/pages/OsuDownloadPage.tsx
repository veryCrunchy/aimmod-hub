import { useEffect, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import {
  fetchOsuReleaseManifest,
  findOsuInstaller,
  findOsuReleaseAsset,
  formatFileSize,
  type OsuReleaseChannel,
  type OsuReleaseManifest,
  type OsuReleasePlatform,
} from "../lib/osuReleases";

const REPOSITORY_URL = "https://github.com/veryCrunchy/aimmod/tree/feat/native-osu-companion";

const features = [
  ["Beatmap intelligence", "Inspect every difficulty separately, compare skill demand, and find maps that match the way you actually play."],
  ["Replay analysis", "Review exact judgements and notable moments, then compare repeated mistakes across multiple attempts of the same map."],
  ["Global coaching", "Build a recent skill profile across maps, with timeframe controls and clear priorities that lead back to the relevant replays."],
  ["PP targets", "See expected and realistic PP per difficulty, filter recommendations, and open or save maps directly from the finder."],
  ["Practice maps", "Turn difficult jump, stream, and reading sections into longer drills with lead-in, repetition, and osu!lazer import."],
  ["Local and online data", "Combine detailed local history with osu! scores so recommendations reflect the strongest available view of your play."],
] as const;

const platforms: Array<{
  id: OsuReleasePlatform;
  name: string;
  detail: string;
  packageName: string;
}> = [
  { id: "windows", name: "Windows", detail: "Windows 10 or 11 · x64", packageName: "Setup executable" },
  { id: "linux", name: "Linux", detail: "64-bit Linux · x64", packageName: "Self-updating AppImage" },
];

function useRelease(channel: OsuReleaseChannel) {
  const [manifest, setManifest] = useState<OsuReleaseManifest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();
    setManifest(null);
    setError(null);
    setLoading(true);

    void fetchOsuReleaseManifest(channel, controller.signal)
      .then((next) => setManifest(next))
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) {
          setError(reason instanceof Error ? reason.message : "Release information is temporarily unavailable.");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });

    return () => controller.abort();
  }, [channel]);

  return { manifest, error, loading };
}

function PlatformDownload({ platform, manifest, loading }: {
  platform: typeof platforms[number];
  manifest: OsuReleaseManifest | null;
  loading: boolean;
}) {
  const installer = manifest ? findOsuInstaller(manifest, platform.id) : null;
  const portable = manifest ? findOsuReleaseAsset(manifest, platform.id) : null;

  return (
    <div className="grid gap-4 border-t border-line py-5 first:border-t-0 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <h3 className="text-[18px] tracking-normal">{platform.name}</h3>
          <span className="text-[11px] text-muted-2">{platform.detail}</span>
        </div>
        <div className="mt-2 text-[12px] text-muted">
          {loading ? "Checking release channel..." : installer && manifest ? `${platform.packageName} · ${formatFileSize(installer.size)} · v${manifest.version}` : "No build is available in this channel."}
        </div>
        {installer ? (
          <details className="mt-3 text-[10px] text-muted-2">
            <summary className="cursor-pointer select-none text-cyan">Verify download</summary>
            <code className="mt-2 block max-w-full overflow-x-auto rounded-md bg-black/25 px-2.5 py-2 text-[9px] text-muted">SHA-256 {installer.sha256}</code>
          </details>
        ) : null}
        {portable ? <a href={portable.downloadUrl} className="mt-2 inline-block text-[10px] text-muted-2 hover:text-cyan">Portable {portable.format}</a> : null}
      </div>
      {installer ? (
        <Button href={installer.downloadUrl} download variant="primary" className="w-full md:w-auto">Download for {platform.name}</Button>
      ) : (
        <Button type="button" disabled className="w-full cursor-not-allowed opacity-45 md:w-auto">Unavailable</Button>
      )}
    </div>
  );
}

export function OsuDownloadPage() {
  const [channel, setChannel] = useState<OsuReleaseChannel>("stable");
  const { manifest, error, loading } = useRelease(channel);

  return (
    <PageStack>
      <Helmet>
        <title>AimMod for osu! · Windows and Linux</title>
        <meta name="description" content="Download AimMod for osu! on Windows or Linux for beatmap and replay analysis, global coaching, PP targets, and practice maps." />
        <meta property="og:title" content="AimMod for osu! · Windows and Linux" />
        <meta property="og:description" content="Native osu! analysis and coaching built around your beatmaps, replays, and scores." />
      </Helmet>

      <section className="relative flex min-h-[520px] items-end overflow-hidden border-y border-[#ff66aa]/25 bg-[#070b13] p-5 md:p-8">
        <img
          src="/images/aimmod-osu-beatmaps.png"
          alt="AimMod for osu! beatmap analysis with difficulty statistics, PP at accuracy, and a skill-demand graph"
          className="absolute inset-0 h-full w-full object-cover object-center"
        />
        <div className="absolute inset-0 bg-[#070b13]/75" aria-hidden />
        <div className="relative z-10 min-w-0 max-w-[680px] pb-3">
            <div className="text-[11px] uppercase tracking-normal text-[#ff9bc7]">Native analysis and coaching</div>
            <h1 className="my-3 max-w-[12ch] text-[clamp(34px,5.5vw,64px)] leading-[0.94] tracking-normal">AimMod for osu!</h1>
            <p className="max-w-[620px] text-[14px] leading-6 text-[#e4ccd8] md:text-[16px] md:leading-7">
              Understand why plays break down, find maps that fit your current skill, and turn replay evidence into focused practice.
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              <Button to="#downloads" variant="primary">Get AimMod for osu!</Button>
              <Button href={REPOSITORY_URL} target="_blank" rel="noreferrer">View source</Button>
              <Button to="/app/kovaaks">Looking for KovaaK's?</Button>
            </div>
            <div className="mt-5 flex flex-wrap gap-x-5 gap-y-2 text-[11px] text-muted">
              <span>Windows x64</span><span>Linux x64</span><span>osu!lazer</span><span>Self-contained</span>
            </div>
        </div>
      </section>

      <PageSection id="downloads" className="scroll-mt-36 p-5 md:p-7">
        <div className="flex flex-col gap-4 border-b border-line pb-5 md:flex-row md:items-end md:justify-between">
          <div>
            <div className="text-[11px] uppercase tracking-normal text-[#ff9bc7]">Downloads</div>
            <h2 className="mt-2 text-[clamp(22px,3vw,32px)] leading-none tracking-normal">
              {manifest ? `AimMod for osu! v${manifest.version}` : "Choose your platform"}
            </h2>
            <p className="mt-2 text-[12px] leading-5 text-muted">
              {channel === "stable" ? "Recommended builds for everyday use." : "Early builds for testing upcoming changes."}
            </p>
          </div>
          <div className="inline-flex w-fit rounded-full border border-line bg-black/20 p-1" aria-label="Release channel">
            {(["stable", "preview"] as const).map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => setChannel(item)}
                aria-pressed={channel === item}
                className={`min-h-8 rounded-full px-3 text-[11px] capitalize transition-colors ${channel === item ? "bg-[#ff66aa] text-[#140910]" : "text-muted hover:text-text"}`}
              >
                {item}
              </button>
            ))}
          </div>
        </div>

        <div aria-live="polite">
          {platforms.map((platform) => <PlatformDownload key={platform.id} platform={platform} manifest={manifest} loading={loading} />)}
        </div>

        {error ? (
          <div className="border-t border-line pt-4 text-[12px] text-muted">
            <span className="text-text">{error}</span> Check the channel again soon or review its status on GitHub.
          </div>
        ) : manifest ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4 text-[10px] text-muted-2">
            <span>Channel: {manifest.channel} · Release tag: {manifest.tag}</span>
            <a href={manifest.releaseUrl} target="_blank" rel="noreferrer" className="text-cyan hover:underline">Release notes</a>
          </div>
        ) : null}
      </PageSection>

      <section aria-labelledby="osu-features-title" className="py-2">
        <div className="mb-5 px-1">
          <div className="text-[11px] uppercase tracking-normal text-[#ff9bc7]">Built around osu!</div>
          <h2 id="osu-features-title" className="mt-2 text-[clamp(24px,3.5vw,38px)] leading-none tracking-normal">From a single play to the bigger picture.</h2>
        </div>
        <div className="grid border-y border-line md:grid-cols-2 lg:grid-cols-3">
          {features.map(([title, body], index) => (
            <div key={title} className={`min-w-0 border-line px-4 py-5 md:px-5 md:py-6 ${index > 0 ? "border-t" : ""} ${index < 2 ? "md:border-t-0" : "md:border-t"} ${index % 2 ? "md:border-l" : "md:border-l-0"} ${index < 3 ? "lg:border-t-0" : "lg:border-t"} ${index % 3 ? "lg:border-l" : "lg:border-l-0"}`}>
              <div className="text-[10px] tabular-nums text-[#ff9bc7]">0{index + 1}</div>
              <h3 className="mt-3 text-[15px] tracking-normal">{title}</h3>
              <p className="mt-2 text-[12px] leading-5 text-muted">{body}</p>
            </div>
          ))}
        </div>
      </section>

      <PageSection className="border-mint/20 p-5 md:p-7">
        <div className="grid gap-6 md:grid-cols-2 md:gap-0">
          <div className="md:pr-7">
            <div className="text-[10px] uppercase tracking-[0.1em] text-[#ff9bc7]">AimMod for osu!</div>
            <h2 className="mt-2 text-[22px] tracking-[-0.04em]">Beatmaps, replays, PP, and practice.</h2>
            <p className="mt-3 text-[12px] leading-5 text-muted">A native osu! companion for analysis and training on Windows and Linux.</p>
          </div>
          <div className="border-t border-line pt-6 md:border-l md:border-t-0 md:pl-7 md:pt-0">
            <div className="text-[10px] uppercase tracking-[0.1em] text-cyan">AimMod for KovaaK's</div>
            <h2 className="mt-2 text-[22px] tracking-[-0.04em]">Live HUDs, mouse paths, and scenarios.</h2>
            <p className="mt-3 text-[12px] leading-5 text-muted">The Windows companion for KovaaK's remains a separate product with its own installer and updates.</p>
            <Button to="/app/kovaaks" className="mt-5">Explore KovaaK's</Button>
          </div>
        </div>
      </PageSection>
    </PageStack>
  );
}
