import { Link, useSearchParams } from "react-router-dom";
import { Helmet } from "../lib/helmet";
import { Button } from "../components/ui/Button";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageStack } from "../components/ui/Stack";

const RELEASES = "https://github.com/verycrunchy/aimmod/releases";
// Permanent channel links: each always serves the newest installer of its channel.
export const BETA_SETUP_URL = "https://github.com/verycrunchy/aimmod/releases/download/aimmod-ingame-beta/AimMod-Setup.exe";
export const STABLE_SETUP_URL = "https://github.com/verycrunchy/aimmod/releases/download/aimmod-ingame-stable/AimMod-Setup.exe";
// Flip once the first Stable in-game release is published.
const STABLE_AVAILABLE = false;

const steps = [
  { title: "Download AimMod-Setup.exe", body: "One small file. It carries no game files itself: it always downloads the newest AimMod release for the channel you pick." },
  { title: "Run it and allow SmartScreen", body: "The installer is not code-signed, so Windows may say \"Windows protected your PC\". Click More info, check the file name is AimMod-Setup.exe, then Run anyway." },
  { title: "Let it find KovaaK's", body: "It finds the game through your Steam library, or you pick the folder. No administrator rights are needed unless the game folder is protected." },
  { title: "Install, then start KovaaK's from Steam", body: "Close KovaaK's first; if it is running, the installer waits and continues once the game closes. AimMod starts with the game from then on." },
];

const inGame = [
  { title: "Workspace in the game menu", body: "Score history and trends for 7, 30 and 90 days, practice time, per-run analysis and coaching cards, opened from KovaaK's own menu." },
  { title: "Live stats HUD", body: "Score, accuracy, score per minute, kills per second and time left while you play, in a compact bar you can move." },
  { title: "Hub sync", body: "Link the game to your Hub account once. Runs upload as you play, your live status shows on the Hub, and your KovaaK's account links to your profile so benchmark ranks appear." },
  { title: "Benchmark ranks in game", body: "Search your benchmark ranks and see each scenario's thresholds without leaving KovaaK's." },
  { title: "Discord presence", body: "Shows your scenario, state, timer and, if you allow it, score and personal best." },
  { title: "Replays", body: "A native replay recorder is in development in the Beta channel; expect changes." },
];

const lifecycle = [
  { title: "Updates", body: "AimMod checks for updates when the game starts and every few hours, downloads in the background and installs after you close KovaaK's. The workspace shows \"Update ready\" first. Turn updates off or switch channel in Settings > Updates & repair." },
  { title: "Verified downloads", body: "Every download is checked against SHA-256 hashes published with the release: the package, its manifest and every installed file. Anything that does not match is refused and nothing is installed." },
  { title: "After a game update", body: "If a KovaaK's update or \"Verify integrity of game files\" breaks the mod, AimMod offers a one-click repair. If it does not load at all, run %LOCALAPPDATA%\\AimMod\\Repair-AimMod.cmd with the game closed; it reinstalls from the copy saved at install time." },
  { title: "Uninstall", body: "Use AimMod-Setup.exe > Uninstall or Windows Settings > Apps > AimMod for KovaaK's. Files AimMod replaced are restored. Your history, replays and settings stay unless you tick \"Also remove my AimMod data\"." },
];

function Card({ title, body }: { title: string; body: string }) {
  return (
    <li className="rounded-md border border-line bg-panel px-4 py-3">
      <h3 className="text-sm font-semibold">{title}</h3>
      <p className="mt-1 text-sm leading-6 text-muted">{body}</p>
    </li>
  );
}

export function AimModPage() {
  const [params] = useSearchParams();
  const claim = params.get("claim")?.trim() ?? "";
  return (
    <PageStack>
      <Helmet>
        <title>AimMod for KovaaK's · Windows</title>
        <meta name="description" content="Install AimMod in KovaaK's with AimMod-Setup.exe: a workspace in the game menu, a live stats HUD and Hub sync. Updates install themselves after you close the game." />
        <meta property="og:title" content="AimMod for KovaaK's · Windows" />
        <meta property="og:description" content="AimMod runs inside KovaaK's. One installer, verified downloads, automatic updates." />
      </Helmet>

      <PageHeader
        title="AimMod for KovaaK's"
        meta="Runs inside the game · Windows 10 and 11 · KovaaK's on Steam"
      />

      <section className="grid gap-4 rounded-md border border-mint/30 bg-mint/[0.04] p-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center md:p-5">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold">Get the installer</h2>
          <p className="mt-1 max-w-[62ch] text-sm leading-6 text-muted">
            AimMod-Setup.exe installs UE4SS and the AimMod mods into your KovaaK's folder, keeps a backup of every file it replaces, and adds AimMod to Windows Apps so it can be removed like any program.
          </p>
          <p className="mt-2 text-xs text-muted-2">
            {STABLE_AVAILABLE ? "Stable is recommended. Beta gets new features first." : "AimMod in KovaaK's is in Beta. A Stable channel follows; the installer can switch channels at any time."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2 md:justify-end">
          {STABLE_AVAILABLE ? <Button href={STABLE_SETUP_URL} variant="primary">Download Stable</Button> : null}
          <Button href={BETA_SETUP_URL} variant={STABLE_AVAILABLE ? "secondary" : "primary"}>Download Beta</Button>
          <Button href={RELEASES} target="_blank" rel="noreferrer">All releases</Button>
        </div>
      </section>

      {claim ? (
        <section id="link-account" className="rounded-md border border-cyan/40 bg-cyan/[0.05] p-4">
          <h2 className="text-base font-semibold">Linking your KovaaK's account</h2>
          <p className="mt-1 max-w-[70ch] text-sm leading-6 text-muted">
            The Hub links a KovaaK's account when AimMod sees you play signed in to that Steam account, so nobody can claim someone else's ranks. To link this account:
          </p>
          <ol className="mt-2 grid list-decimal gap-1 pl-5 text-sm text-muted">
            <li>Sign in to the Hub with Discord.</li>
            <li>Install AimMod with the installer above and start KovaaK's on the Steam account you want to link.</li>
            <li>Open the AimMod workspace in the game and link it to your Hub account.</li>
            <li>Play any scenario. Your profile then shows your benchmark ranks and uploaded runs.</li>
          </ol>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button to="/account">Sign in or open your account</Button>
            <Button to={`/u/${encodeURIComponent(claim)}`}>Back to the KovaaK's player page</Button>
          </div>
        </section>
      ) : null}

      <Section title="Install in four steps">
        <ol className="grid gap-2 md:grid-cols-2">
          {steps.map((step, i) => (
            <li key={step.title} className="flex gap-3 rounded-md border border-line bg-panel px-4 py-3">
              <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full border border-line text-xs tabular-nums">{i + 1}</span>
              <div>
                <h3 className="text-sm font-semibold">{step.title}</h3>
                <p className="mt-1 text-sm leading-6 text-muted">{step.body}</p>
              </div>
            </li>
          ))}
        </ol>
        <p className="mt-2 text-xs text-muted-2">
          Without the installer: unzip AimMod-InGame-&lt;version&gt;.zip from a release, close KovaaK's and run Install-AimMod.cmd.
        </p>
      </Section>

      <Section title="What it adds to KovaaK's">
        <ul className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
          {inGame.map((item) => <Card key={item.title} {...item} />)}
        </ul>
        <p className="mt-2 text-xs text-muted-2">AimMod never changes ranked runs or KovaaK's leaderboards.</p>
      </Section>

      <Section title="Updates, repair and uninstall">
        <ul className="grid gap-2 md:grid-cols-2">
          {lifecycle.map((item) => <Card key={item.title} {...item} />)}
        </ul>
        <p className="mt-2 text-xs text-muted-2">
          Something wrong? The install log is in %LOCALAPPDATA%\AimMod\KovaaksNative\updates\install.log. Ask in the AimMod Discord.
        </p>
      </Section>

      <p className="text-sm text-muted">
        Looking for osu!? <Link className="text-cyan hover:underline" to="/app/osu">AimMod for osu!</Link>
      </p>
    </PageStack>
  );
}
