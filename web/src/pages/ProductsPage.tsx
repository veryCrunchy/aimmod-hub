import { Helmet } from "../lib/helmet";
import { Button } from "../components/ui/Button";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";

const products = [
  {
    name: "AimMod for osu!",
    platform: "Native desktop app · Windows and Linux",
    body: "Turn beatmaps, local replays, and online scores into map recommendations, PP targets, coaching priorities, and focused practice maps.",
    to: "/app/osu",
    action: "Explore osu!",
    accent: "border-[#ff66aa]/35 bg-[#ff66aa]/6",
    label: "text-[#ff9bc7]",
  },
  {
    name: "AimMod for KovaaK's",
    platform: "Desktop companion · Windows",
    body: "Capture live runs, study mouse-path replays, understand your aim fingerprint, and turn scenario history into practical coaching.",
    to: "/app/kovaaks",
    action: "Explore KovaaK's",
    accent: "border-mint/25 bg-mint/5",
    label: "text-cyan",
  },
];

export function ProductsPage() {
  return (
    <PageStack>
      <Helmet>
        <title>Download AimMod · osu! and KovaaK's</title>
        <meta name="description" content="Choose AimMod for osu! on Windows or Linux, or AimMod for KovaaK's on Windows." />
      </Helmet>

      <PageSection className="p-0">
        <img src="/images/aimmod-brand-banner.png" alt="AimMod" width="1676" height="419" className="mb-6 block w-full" />
        <h1 className="text-3xl font-semibold">Download AimMod</h1>
        <p className="mt-2 text-sm text-muted">Choose your game to find downloads and supported platforms.</p>
      </PageSection>

      <div className="grid gap-4 lg:grid-cols-2">
        {products.map((product) => (
          <PageSection key={product.name} className="flex flex-col border-t border-line pt-5">
            <div className={`text-[10px] uppercase tracking-normal ${product.label}`}>{product.platform}</div>
            <h2 className="mt-4 text-2xl leading-snug tracking-normal">{product.name}</h2>
            <p className="mt-4 max-w-[58ch] text-[13px] leading-6 text-muted">{product.body}</p>
            <div className="mt-auto pt-7">
              <Button to={product.to} variant="primary">{product.action}</Button>
            </div>
          </PageSection>
        ))}
      </div>
    </PageStack>
  );
}
