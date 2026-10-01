import { BadgeCheck } from "lucide-react";

/** A small check for verified profiles. Unverified profiles show nothing. */
export function VerificationBadge({ verified }: { verified: boolean }) {
  if (!verified) return null;
  return (
    <span className="inline-flex shrink-0 items-center text-mint" title="Linked to a verified account">
      <BadgeCheck aria-hidden="true" size={16} strokeWidth={2} />
      <span className="sr-only">Verified</span>
    </span>
  );
}
