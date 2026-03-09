import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { approveDeviceLink, discordStartUrl } from "../lib/auth";

export function DeviceLinkPage() {
  const auth = useAuth();
  const location = useLocation();
  const [state, setState] = useState<"idle" | "approving" | "approved" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const userCode = useMemo(
    () => new URLSearchParams(location.search).get("user_code")?.trim().toUpperCase() ?? "",
    [location.search],
  );

  useEffect(() => {
    if (!userCode || auth.loading || !auth.authenticated || !auth.user || state === "approving" || state === "approved") {
      return;
    }

    let cancelled = false;
    setState("approving");
    setError(null);
    void approveDeviceLink(userCode)
      .then(() => {
        if (!cancelled) {
          setState("approved");
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setState("error");
          setError(err instanceof Error ? err.message : "Could not approve this device link.");
        }
      });

    return () => {
      cancelled = true;
    };
  }, [auth.authenticated, auth.loading, auth.user, state, userCode]);

  if (!userCode) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Device link" title="Missing link code" body="Open this page from AimMod so the link code is already filled in." />
        </PageSection>
      </PageStack>
    );
  }

  if (auth.loading) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Device link" title="Checking account" body="We are checking your AimMod Hub session before linking this desktop app." />
        </PageSection>
      </PageStack>
    );
  }

  if (!auth.authenticated || !auth.user) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Device link" title="Sign in to approve this desktop app" body={`Device code ${userCode} is waiting for approval.`} />
          <EmptyState title="Discord sign-in required" body="Sign in first, then this page will finish linking AimMod automatically.">
            <Button href={discordStartUrl(`${location.pathname}${location.search}`)} variant="primary">
              Continue with Discord
            </Button>
          </EmptyState>
        </PageSection>
      </PageStack>
    );
  }

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Device link"
          title={state === "approved" ? "AimMod is linked" : "Approve desktop app"}
          body={`Signed in as ${auth.user.displayName || auth.user.username}. Device code ${userCode} will be used for this link.`}
        />
        {state === "approved" ? (
          <EmptyState
            title="Desktop app connected"
            body="You can go back to AimMod now. It should finish linking on its own and start syncing pending runs."
          />
        ) : state === "error" ? (
          <EmptyState
            title="Could not approve this device"
            body={error ?? "Something went wrong while approving this desktop app. Try the link again from AimMod."}
          />
        ) : (
          <EmptyState
            title={state === "approving" ? "Approving device..." : "Ready to approve"}
            body={state === "approving"
              ? "This page is linking your desktop app now."
              : "This page will approve the desktop app automatically."}
          />
        )}
      </PageSection>
    </PageStack>
  );
}
