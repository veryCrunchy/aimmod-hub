import { useState } from "react";
import { SectionHeader } from "../components/SectionHeader";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";

export function AccountPage() {
  const auth = useAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleRevokeToken(id: number) {
    setBusy(true);
    setError(null);
    try {
      await auth.revokeToken(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove desktop access");
    } finally {
      setBusy(false);
    }
  }

  if (auth.loading) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader eyebrow="Account" title="Checking session" />
        </PageSection>
      </PageStack>
    );
  }

  if (!auth.authenticated || !auth.user) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader
            eyebrow="Account"
            title="Sign in to link AimMod"
            body="Discord is the first linked identity. Once you sign in, AimMod can link directly from the desktop app and start sending real run data into the hub."
          />
          <EmptyState
            title="Not signed in"
            body="Website auth is backed by Discord. After you sign in, the desktop app can connect through the browser without asking you to paste secrets manually."
          >
            <Button href={discordStartUrl("/account")} variant="primary">
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
          eyebrow="Account"
          title={auth.user.displayName || auth.user.username}
          body={`Connected as ${auth.user.username}. This page manages desktop access and the linked account behind your AimMod data.`}
          aside={`User ID · ${auth.user.userExternalId}`}
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Discord</div>
            <div className="mt-2 text-2xl text-text">@{auth.user.username}</div>
            <p className="mt-3 text-sm leading-7 text-muted">This linked identity will drive profile ownership, sharing, and future social/community surfaces.</p>
          </div>
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Connected devices</div>
            <div className="mt-2 text-2xl text-mint">{auth.tokens?.length ?? 0}</div>
            <p className="mt-3 text-sm leading-7 text-muted">Each linked desktop app gets its own access record. Remove one here if you want to disconnect a machine and stop future uploads from it.</p>
          </div>
        </Grid>
      </PageSection>

      <PageSection>
        <SectionHeader
          eyebrow="Desktop access"
          title="Linked devices"
          body="Desktop apps linked through the browser show up here. Removing one immediately stops it from uploading new runs."
        />
        <div className="mb-4 flex flex-wrap gap-3">
          <Button onClick={() => void auth.signOut()}>Sign out</Button>
        </div>
        {error ? <p className="mb-4 text-sm text-danger">{error}</p> : null}
        {auth.tokens && auth.tokens.length > 0 ? (
          <ScrollArea className="max-h-[min(64vh,820px)] pr-2">
            <div className="grid gap-3">
            {auth.tokens.map((token) => (
              <div key={token.id} className="rounded-[18px] border border-line bg-white/2 p-[18px]">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <strong className="block text-text">{token.label}</strong>
                    <p className="mt-1 text-sm text-muted">•••• {token.lastFour}</p>
                    <p className="mt-3 text-sm text-muted">
                      Linked {new Date(token.createdAt).toLocaleString()}
                      {token.lastUsedAt ? ` · last seen ${new Date(token.lastUsedAt).toLocaleString()}` : ""}
                    </p>
                  </div>
                  <Button onClick={() => void handleRevokeToken(token.id)} disabled={busy}>
                    Remove
                  </Button>
                </div>
              </div>
            ))}
            </div>
          </ScrollArea>
        ) : (
          <EmptyState
            title="No linked desktop apps"
            body="That is fine. Start the link from AimMod and approve it in the browser when you are ready to sync runs."
          />
        )}
      </PageSection>
    </PageStack>
  );
}
