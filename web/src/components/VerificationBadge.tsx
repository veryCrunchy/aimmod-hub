export function VerificationBadge({ verified }: { verified: boolean }) {
  return (
    <span
      className={[
        "inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em]",
        verified
          ? "border-mint/30 bg-mint/10 text-mint"
          : "border-gold/25 bg-gold/10 text-gold",
      ].join(" ")}
      title={verified ? "This profile is linked to a verified account." : "This profile has not been linked and verified yet."}
    >
      {verified ? "Verified" : "Unverified"}
    </span>
  );
}
