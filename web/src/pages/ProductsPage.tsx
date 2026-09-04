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

      <PageSection className="overflow-hidden border-cyan/20 bg-[linear-gradient(135deg,rgba(7,22,18,0.98),rgba(5,13,10,0.98))] px-5 py-7 md:px-8 md:py-10">
        <div className="text-[11px] uppercase tracking-normal text-cyan">AimMod desktop</div>
        <h1 className="my-3 max-w-[18ch] text-[clamp(30px,5vw,58px)] leading-[0.98] tracking-normal">
          Choose where you practice.
        </h1>
        <p className="max-w-[720px] text-[14px] leading-6 text-muted md:text-[16px] md:leading-7">
          Each AimMod client is built around its game and ships through its own release channel. Pick a product to see supported platforms, features, and downloads.
        </p>
      </PageSection>

      <div className="grid gap-4 lg:grid-cols-2">
        {products.map((product) => (
          <PageSection key={product.name} className={`flex min-h-[280px] flex-col p-5 md:p-7 ${product.accent}`}>
            <div className={`text-[10px] uppercase tracking-normal ${product.label}`}>{product.platform}</div>
            <h2 className="mt-4 max-w-[14ch] text-[clamp(24px,3.4vw,38px)] leading-none tracking-normal">{product.name}</h2>
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
