import type { PropsWithChildren } from "react";
import { Link, NavLink } from "react-router-dom";
import { useApiHealth } from "../lib/useApiHealth";
import { cn } from "../lib/cn";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import { Button } from "./ui/Button";

const navItems = [
  { to: "/", label: "Home" },
  { to: "/community", label: "Community" },
  { to: "/account", label: "Account" }
];

export function AppShell({ children }: PropsWithChildren) {
  const { healthLabel, statusLabel, online } = useApiHealth();
  const auth = useAuth();

  return (
    <div className="min-h-screen overflow-x-clip">
      <header className="sticky top-0 z-10 grid grid-cols-[auto_1fr_auto] items-center gap-6 border-b border-white/8 bg-[rgba(3,6,17,0.82)] px-6 py-[22px] backdrop-blur-xl max-[1100px]:grid-cols-1 max-[1100px]:justify-items-start">
        <Link to="/" className="grid gap-1">
          <span className="text-[11px] uppercase tracking-[0.12em] text-cyan">AimMod Hub</span>
          <span className="text-[13px] text-text">shared practice intelligence</span>
        </Link>

        <nav className="flex flex-wrap justify-center gap-2.5 max-[1100px]:justify-start" aria-label="Primary">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "rounded-full border border-transparent px-[14px] py-2.5 text-sm text-muted transition-colors",
                  "hover:border-line hover:bg-white/3 hover:text-text",
                  isActive && "border-line bg-white/3 text-text"
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className="flex flex-wrap items-center justify-end gap-3">
          <div
            className={cn(
              "inline-flex items-center gap-2.5 rounded-full border border-line bg-white/3 px-[14px] py-2.5 text-[13px]"
            )}
          >
            <span
              className={cn(
                "h-[9px] w-[9px] rounded-full bg-muted-2",
                online ? "bg-mint shadow-[0_0_18px_rgba(50,240,170,0.45)]" : "bg-danger"
              )}
            />
            <span>{statusLabel}</span>
            <span className="text-muted">{healthLabel}</span>
          </div>

          {auth.loading ? null : auth.authenticated && auth.user ? (
            <div className="inline-flex items-center gap-2">
              <Link to="/account" className="rounded-full border border-line bg-white/3 px-[14px] py-2.5 text-[13px] text-text">
                {auth.user.displayName || auth.user.username}
              </Link>
              <Button onClick={() => void auth.signOut()}>Sign out</Button>
            </div>
          ) : (
            <Button href={discordStartUrl("/account")} variant="primary">
              Sign in with Discord
            </Button>
          )}
        </div>
      </header>

      <main className="mx-auto min-h-0 w-[min(1380px,calc(100vw-40px))] px-0 py-[34px] pb-20 max-[720px]:w-[min(1380px,calc(100vw-24px))] max-[720px]:pt-5">
        {children}
      </main>
    </div>
  );
}
