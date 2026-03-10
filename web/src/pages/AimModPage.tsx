import { useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { Grid, PageStack } from "../components/ui/Stack";
import { SectionHeader } from "../components/SectionHeader";

const LATEST_RELEASE_URL = "https://github.com/veryCrunchy/aimmod/releases/latest";
const REPO_URL = "https://github.com/veryCrunchy/aimmod";
const HOMEPAGE_URL = "https://aimmod.app";

function useLatestRelease() {
  const [downloadUrl, setDownloadUrl] = useState<string | null>(null);
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    fetch("https://api.github.com/repos/veryCrunchy/aimmod/releases?per_page=10", {
      headers: { Accept: "application/vnd.github+json" },
    })
      .then((r) => r.json())
      .then((releases: { tag_name: string; prerelease: boolean; assets: { name: string; browser_download_url: string }[] }[]) => {
        const stableReleases = releases.filter((r) => !r.prerelease);
        for (const release of stableReleases) {
          const exe = release.assets.find(
            (a) => a.name.endsWith(".exe") && !a.name.includes("debug")
          );
          if (exe) {
            setVersion(release.tag_name);
            setDownloadUrl(exe.browser_download_url);
            return;
          }
        }
      })
      .catch(() => {});
  }, []);

  return { downloadUrl, version };
}

const SCREENSHOTS = [
  { src: "https://raw.githubusercontent.com/veryCrunchy/aimmod/main/public/NzXmg9xdc9.png", label: "Live challenge HUD" },
  { src: "https://raw.githubusercontent.com/veryCrunchy/aimmod/main/public/cD4yvwyuz8.png", label: "Scenario summary" },
  { src: "https://raw.githubusercontent.com/veryCrunchy/aimmod/main/public/tvxcCZoOfC.png", label: "Scenario coaching" },
  { src: "https://raw.githubusercontent.com/veryCrunchy/aimmod/main/public/uSIwrmIcw1.png", label: "Focused replay moment" },
  { src: "https://raw.githubusercontent.com/veryCrunchy/aimmod/main/public/WuKkgKOWX6.png", label: "Full-run replay review" },
];

const FEATURES = [
  {
    eyebrow: "In-game overlay",
    title: "Real-time HUDs while you play",
    body: "Live challenge HUDs for score, timing, pace, accuracy, and scenario state. Smoothness and mouse-control feedback during runs, coaching toasts, and a post-session overview. Drag-and-scale layout mode with saved positions.",
    accent: "cyan" as const,
  },
  {
    eyebrow: "Session stats",
    title: "Deep per-scenario analysis",
    body: "Global overview of all your recent practice. Per-scenario pages for summary, mechanics, coaching, replay, and leaderboard views. Practice profile, scenario comparison tools, and SQL-backed session history.",
    accent: "mint" as const,
  },
  {
    eyebrow: "Replay analysis",
    title: "Review every moment in detail",
    body: "Mouse path replay for the full run or selected moments. Saved focus moments, quick notes, and replay navigation. Timeline-by-second review, shot detail context, and video replay capture alongside the mouse path.",
    accent: "gold" as const,
  },
  {
    eyebrow: "Coaching and profiling",
    title: "Understand your aim style",
    body: "Aim fingerprint and aim-style summaries. Warm-up and practice-pattern insights. Scenario-specific coaching cards. Trend, floor, peak-zone, and consistency views.",
    accent: "violet" as const,
  },
];

const HOTKEYS = [
  { key: "F8", action: "Open settings" },
  { key: "F10", action: "Toggle HUD layout mode" },
];

const ACCENT_CLASSES = {
  cyan: "text-cyan border-cyan/20 bg-cyan/5",
  mint: "text-mint border-mint/20 bg-mint/5",
  gold: "text-gold border-[rgba(212,175,55,0.2)] bg-[rgba(212,175,55,0.05)]",
  violet: "text-violet border-violet/20 bg-violet/5",
};

export function AimModPage() {
  const [activeScreenshot, setActiveScreenshot] = useState(0);
  const { downloadUrl, version } = useLatestRelease();

  return (
    <PageStack>
      <Helmet>
        <title>Download AimMod · AimMod Hub</title>
        <meta name="description" content="KovaaK's overlay, replay, and coaching suite. Download the latest release for Windows." />
        <meta property="og:title" content="Download AimMod · AimMod Hub" />
        <meta property="og:description" content="KovaaK's overlay, replay, and coaching suite. Download the latest release for Windows." />
      </Helmet>
      {/* Hero */}
      <PageSection className="relative overflow-hidden border-cyan/20 bg-[radial-gradient(circle_at_top_left,rgba(0,200,255,0.14),transparent_28%),radial-gradient(circle_at_80%_20%,rgba(121,201,151,0.1),transparent_22%),linear-gradient(135deg,rgba(6,18,24,0.99),rgba(4,12,9,0.97)_55%,rgba(3,8,6,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.5)]">
        <div className="absolute inset-y-0 right-[6%] w-[32%] rounded-full bg-[radial-gradient(circle,rgba(0,200,255,0.1),transparent_65%)] blur-3xl" />
        <div className="relative text-[11px] uppercase tracking-[0.1em] text-cyan">AimMod {version ?? "v1.5.0"}</div>
        <h1 className="my-2.5 max-w-[18ch] break-words text-[clamp(26px,4.8vw,56px)] leading-[0.94] tracking-[-0.05em]">
          KovaaK's overlay, replay, and coaching suite.
        </h1>
        <p className="max-w-[680px] text-[14px] leading-6 text-[#cbe4d7] md:text-[16px] md:leading-7">
          AimMod runs a live in-session HUD while you play and a full post-session stats window for replay review, coaching, and scenario analysis — all synced to the same run.
        </p>
        <div className="relative mt-4 flex flex-wrap gap-2">
          <Button
            href={downloadUrl ?? LATEST_RELEASE_URL}
            download={downloadUrl ? true : undefined}
            target="_blank"
            rel="noreferrer"
            variant="primary"
          >
            Download latest release
          </Button>
          <Button href={HOMEPAGE_URL} target="_blank" rel="noreferrer">
            aimmod.app
          </Button>
          <Button href={REPO_URL} target="_blank" rel="noreferrer">
            GitHub
          </Button>
        </div>
        <div className="relative mt-4 flex flex-wrap items-center gap-3 text-[12px] text-muted">
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-mint/70" />
            Windows 10 / 11
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-cyan/70" />
            KovaaK's Aim Trainer (Steam)
          </span>
        </div>
      </PageSection>

      {/* Screenshots */}
      <PageSection>
        <SectionHeader
          eyebrow="Screenshots"
          title="See it in action"
        />
        <div className="grid gap-3">
          <div className="flex items-center justify-center overflow-hidden rounded-[14px] border border-line bg-black/40">
            <img
              src={SCREENSHOTS[activeScreenshot].src}
              alt={SCREENSHOTS[activeScreenshot].label}
              className="block h-auto max-h-[520px] w-full object-contain"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            {SCREENSHOTS.map((shot, i) => (
              <button
                key={shot.label}
                onClick={() => setActiveScreenshot(i)}
                className={`rounded-full border px-3 py-1.5 text-[12px] transition-colors ${
                  i === activeScreenshot
                    ? "border-cyan/40 bg-cyan/10 text-cyan"
                    : "border-line bg-white/2 text-muted hover:border-line-strong hover:text-text"
                }`}
              >
                {shot.label}
              </button>
            ))}
          </div>
        </div>
      </PageSection>

      {/* Features */}
      <Grid className="grid-cols-2 max-[900px]:grid-cols-1">
        {FEATURES.map((feature) => (
          <PageSection key={feature.eyebrow}>
            <div className={`mb-3 inline-flex rounded-full border px-2.5 py-1 text-[11px] uppercase tracking-[0.1em] ${ACCENT_CLASSES[feature.accent]}`}>
              {feature.eyebrow}
            </div>
            <h2 className="mb-2 text-[clamp(16px,2vw,22px)] font-medium leading-[1.1] tracking-[-0.025em]">
              {feature.title}
            </h2>
            <p className="text-[13px] leading-[1.65] text-muted">{feature.body}</p>
          </PageSection>
        ))}
      </Grid>

      {/* Quick start + hotkeys */}
      <Grid className="grid-cols-2 max-[800px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Quick start"
            title="Up and running in minutes"
          />
          <ol className="grid gap-3">
            {[
              "Download the latest build from the Releases page.",
              "Launch AimMod.",
              "Start KovaaK's.",
              "Open settings to choose which HUDs are visible or reposition them.",
              "Play a scenario.",
              "Open the stats window to review the run, replay key moments, and inspect scenario-specific coaching.",
            ].map((step, i) => (
              <li key={i} className="flex gap-3 text-[13px] leading-relaxed text-muted">
                <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-line bg-white/3 text-[11px] text-text">
                  {i + 1}
                </span>
                {step}
              </li>
            ))}
          </ol>
          <div className="mt-4">
            <Button
              href={downloadUrl ?? LATEST_RELEASE_URL}
              download={downloadUrl ? true : undefined}
              target="_blank"
              rel="noreferrer"
              variant="primary"
            >
              Download now
            </Button>
          </div>
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Default hotkeys"
            title="Keyboard shortcuts"
          />
          <div className="grid gap-2">
            {HOTKEYS.map(({ key, action }) => (
              <div key={key} className="flex items-center justify-between rounded-[12px] border border-line bg-white/2 px-4 py-3">
                <span className="text-[13px] text-muted">{action}</span>
                <kbd className="rounded-md border border-line bg-white/5 px-2.5 py-1 font-mono text-[12px] text-text">
                  {key}
                </kbd>
              </div>
            ))}
          </div>

          <div className="mt-5 border-t border-line pt-4">
            <SectionHeader
              eyebrow="Integration"
              title="Plays well with others"
              className="mb-3"
            />
            <ul className="grid gap-2 text-[13px] text-muted">
              {[
                "Discord Rich Presence — show your current scenario and stats",
                "UE4SS runtime bridge into KovaaK's for live data access",
                "Automatic stats import from KovaaK's run results",
              ].map((item) => (
                <li key={item} className="flex items-start gap-2">
                  <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan/60" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
        </PageSection>
      </Grid>
    </PageStack>
  );
}
