import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl, updateProfileHandle } from "../lib/auth";

export function AccountPage() {
  const auth = useAuth();
  const isAdmin = Boolean(auth.user?.isAdmin ?? auth.isAdmin);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [handleInput, setHandleInput] = useState("");

  useEffect(() => {
    setHandleInput(auth.user?.profileHandle || auth.user?.username || "");
  }, [auth.user?.profileHandle, auth.user?.username]);

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

  async function handleSaveProfileHandle() {
    if (!auth.user) return;
    setBusy(true);
    setError(null);
    try {
      await updateProfileHandle(handleInput);
      await auth.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update profile handle");
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

  const identityRows = [
    { label: "AimMod user ID", value: auth.user.aimmodUserId },
    { label: "Legacy external key", value: auth.user.userExternalId },
    { label: "Discord ID", value: auth.user.discordUserId },
    { label: "Steam ID", value: auth.user.steamId },
    { label: "Steam display", value: auth.user.steamDisplayName },
    { label: "KovaaK's user ID", value: auth.user.kovaaksUserId },
    { label: "KovaaK's username", value: auth.user.kovaaksUsername },
  ].filter((row) => row.value && row.value.trim().length > 0);

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Account"
          title={auth.user.displayName || auth.user.username}
          body={`Connected as ${auth.user.username}. This page manages desktop access and the linked account behind your AimMod data.`}
          aside={`AimMod ID · ${auth.user.aimmodUserId || "Pending"}`}
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Profile</div>
            <div className="mt-2 text-2xl text-text">@{auth.user.profileHandle || auth.user.username}</div>
            <div className="mt-3 flex items-center gap-2">
              <VerificationBadge verified={Boolean(auth.user.profileVerified)} />
              <span className="text-sm text-muted">@{auth.user.profileHandle || auth.user.username}</span>
            </div>
            <div className="mt-4 flex gap-2">
              <input
                value={handleInput}
                onChange={(event) => setHandleInput(event.target.value)}
                placeholder="choose-your-handle"
                className="min-w-0 flex-1 rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-3 py-2 text-sm text-text outline-none"
              />
              <Button
                onClick={() => void handleSaveProfileHandle()}
                disabled={busy || handleInput.trim() === (auth.user.profileHandle || auth.user.username)}
              >
                Save
              </Button>
            </div>
            <p className="mt-2 text-xs text-muted">Public profile URL: /profiles/{auth.user.profileHandle || auth.user.username}</p>
            <p className="mt-3 text-sm leading-7 text-muted">Discord signs you in. Your verified in-game identity is what claims and owns the public training profile.</p>
          </div>
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Linked identities</div>
            <div className="mt-3 grid gap-3">
              {identityRows.map((row) => (
                <div key={row.label} className="rounded-[14px] border border-line/80 bg-black/10 px-3 py-2">
                  <div className="text-[11px] uppercase tracking-[0.08em] text-muted">{row.label}</div>
                  <div className="mt-1 break-all text-sm text-text">{row.value}</div>
                </div>
              ))}
            </div>
            <p className="mt-3 text-sm leading-7 text-muted">
              These values come from the linked Discord account plus any Steam or KovaaK&apos;s identities the hub has attached to your profile.
            </p>
          </div>
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Connected devices</div>
            <div className="mt-2 text-2xl text-mint">{auth.tokens?.length ?? 0}</div>
            <p className="mt-3 text-sm leading-7 text-muted">Each linked desktop app gets its own access record. Remove one here if you want to disconnect a machine and stop future uploads from it.</p>
          </div>
          {isAdmin ? (
            <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
              <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Admin access</div>
              <div className="mt-2 text-2xl text-text">Enabled</div>
              <p className="mt-3 text-sm leading-7 text-muted">This linked Discord account matches the admin account configured on the API.</p>
            </div>
          ) : null}
        </Grid>
      </PageSection>

      <PageSection>
        <SectionHeader
          eyebrow="Desktop access"
          title="Linked devices"
          body="Desktop apps linked through the browser show up here. Removing one immediately stops it from uploading new runs."
        />
        <div className="mb-4 flex flex-wrap gap-3">
          <Link
            to={`/profiles/${auth.user.profileHandle || auth.user.username}`}
            className="rounded-full border border-line bg-transparent px-[14px] py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
          >
            View my profile
          </Link>
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
