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
import { discordStartUrl, updateProfileSettings } from "../lib/auth";

export function AccountPage() {
  const auth = useAuth();
  const isAdmin = Boolean(auth.user?.isAdmin ?? auth.isAdmin);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [handleInput, setHandleInput] = useState("");
  const [discordDomainTokenInput, setDiscordDomainTokenInput] = useState("");

  useEffect(() => {
    setHandleInput(auth.user?.profileHandle || auth.user?.username || "");
    setDiscordDomainTokenInput(auth.user?.discordDomainToken || "");
  }, [auth.user?.discordDomainToken, auth.user?.profileHandle, auth.user?.username]);

  async function handleRevokeToken(id: number) {
    setBusy(true);
    setError(null);
    try {
      await auth.revokeToken(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not remove this device");
    } finally {
      setBusy(false);
    }
  }

  async function handleSaveProfileSettings() {
    if (!auth.user) return;
    setBusy(true);
    setError(null);
    try {
      await updateProfileSettings(handleInput, discordDomainTokenInput);
      await auth.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update profile settings");
    } finally {
      setBusy(false);
    }
  }

  if (auth.loading) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader title="Account" body="Checking your sign-in…" />
        </PageSection>
      </PageStack>
    );
  }

  if (!auth.authenticated || !auth.user) {
    return (
      <PageStack>
        <PageSection>
          <SectionHeader level={1} title="Account" />
          <EmptyState title="Sign in to link AimMod to your profile." body="Your runs then appear on your Hub profile.">
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
  const activeHandle = handleInput.trim() || auth.user.profileHandle || auth.user.username;
  const profileSubdomainUrl =
    typeof window !== "undefined" && window.location.hostname && window.location.hostname !== "localhost"
      ? `http://${activeHandle}.${window.location.host}`
      : "";
  const discordWellKnownUrl = profileSubdomainUrl ? `${profileSubdomainUrl}/.well-known/discord` : "";
  const settingsUnchanged =
    handleInput.trim() === (auth.user.profileHandle || auth.user.username) &&
    discordDomainTokenInput.trim().replace(/^dh=/, "") === (auth.user.discordDomainToken || "");

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          level={1}
          title={auth.user.displayName || auth.user.username}
          body={`Signed in with Discord as ${auth.user.username}.`}
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
                onClick={() => void handleSaveProfileSettings()}
                disabled={busy || settingsUnchanged}
              >
                Save
              </Button>
            </div>
            <div className="mt-3">
              <div className="text-[12px] uppercase tracking-[0.08em] text-muted">Discord domain proof</div>
              <input
                value={discordDomainTokenInput}
                onChange={(event) => setDiscordDomainTokenInput(event.target.value)}
                placeholder="dh=cc314c71a5e9dd72cfca630a75aef2779fd239cc"
                className="mt-2 w-full rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-3 py-2 text-sm text-text outline-none"
              />
            </div>
            <p className="mt-2 text-xs text-muted">Public profile URL: /profiles/{auth.user.profileHandle || auth.user.username}</p>
            {profileSubdomainUrl ? <p className="mt-2 text-xs text-muted">Handle subdomain: {profileSubdomainUrl}</p> : null}
            {discordWellKnownUrl ? <p className="mt-2 text-xs text-muted">Discord verification URL: {discordWellKnownUrl}</p> : null}
            <p className="mt-2 text-xs text-muted">Paste the token Discord gives you. AimMod serves it from `/.well-known/discord` on your handle subdomain as `dh=&lt;token&gt;`.</p>
            <p className="mt-3 text-sm leading-6 text-muted">Your profile is verified once a KovaaK's or Steam account is linked through AimMod.</p>
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

          </div>
          <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
            <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Linked devices</div>
            <div className="mt-2 text-2xl text-mint">{auth.tokens?.length ?? 0}</div>
          </div>
          {isAdmin ? (
            <div className="rounded-[18px] border border-line bg-white/2 p-[18px]">
              <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">Admin access</div>
              <div className="mt-2 text-2xl text-text">Enabled</div>
              
            </div>
          ) : null}
        </Grid>
      </PageSection>

      <PageSection>
        <SectionHeader
          title="Linked devices"
          body="Removing a device stops it uploading runs straight away."
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
            title="No devices linked yet."
            body="Start linking from AimMod in KovaaK's, then approve it here."
          />
        )}
      </PageSection>
    </PageStack>
  );
}
