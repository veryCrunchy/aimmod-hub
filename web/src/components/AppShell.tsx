import type { PropsWithChildren } from "react";
import { useEffect, useRef } from "react";
import { Link, NavLink } from "react-router-dom";
import { cn } from "../lib/cn";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import { Button } from "./ui/Button";
import { HeaderSearch } from "./HeaderSearch";

const supportLinks = [
  { href: "https://ko-fi.com/verycrunchy", label: "Ko-fi" },
  { href: "https://github.com/sponsors/veryCrunchy", label: "GitHub Sponsors" }
];

export function AppShell({ children }: PropsWithChildren) {
  const auth = useAuth();
  const searchRef = useRef<HTMLInputElement>(null);
  const isAdmin = Boolean(auth.user?.isAdmin ?? auth.isAdmin);
  const navItems = [
    { to: "/", label: "Home", title: "Overview: stats, top scenarios, recent runs" },
    { to: "/app", label: "App", title: "Download the AimMod desktop app" },
    { to: "/community", label: "Community", title: "Browse all scenarios and players" },
    { to: "/replays", label: "Replays", title: "Watch replay videos and mouse paths" },
    { to: "/leaderboard", label: "Leaderboard", title: "All-time records and top 100 scores" },
    ...(isAdmin ? [{ to: "/admin", label: "Admin", title: "Admin panel" }] : []),
    { to: "/account", label: "Account", title: "Your profile, linked devices, and settings" },
  ];

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
        return;
      }
      const tag = (document.activeElement as HTMLElement)?.tagName;
      if (e.key === "/" && tag !== "INPUT" && tag !== "TEXTAREA") {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <div className="min-h-screen overflow-x-clip">
      <header className="sticky top-0 z-10 border-b border-line bg-[linear-gradient(180deg,rgba(2,8,6,0.96),rgba(4,12,9,0.92))] px-3 py-3 backdrop-blur-xl md:px-5 md:py-3.5">
        <div className="mx-auto flex w-[min(1380px,100%)] min-w-0 flex-col gap-2.5 xl:grid xl:grid-cols-[auto_auto_minmax(220px,320px)_auto] xl:items-center xl:gap-4">
          <div className="flex min-w-0 items-center justify-between gap-3 xl:contents">
            <Link to="/" className="grid min-w-0 gap-0.5">
              <span className="text-[10px] uppercase tracking-[0.12em] text-cyan">AimMod Hub</span>
              <span className="truncate text-[12px] text-text max-[640px]:hidden">shared practice intelligence</span>
            </Link>

            <div className="flex flex-wrap items-center justify-end gap-2 xl:contents">
              <nav className="hidden min-w-0 flex-wrap items-center gap-1.5 xl:flex" aria-label="Primary">
                {navItems.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    title={item.title}
                    className={({ isActive }) =>
                      cn(
                        "rounded-full border border-transparent px-3 py-2 text-[13px] text-muted transition-colors",
                        "hover:border-line hover:bg-[rgba(121,201,151,0.08)] hover:text-text",
                        isActive && "border-line bg-[rgba(121,201,151,0.1)] text-text"
                      )
                    }
                  >
                    {item.label}
                  </NavLink>
                ))}
              </nav>

              {auth.loading ? null : auth.authenticated && auth.user ? (
                <div className="inline-flex min-w-0 items-center gap-2">
                  <Link to="/account" className="max-w-[44vw] truncate rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-3 py-2 text-[12px] text-text md:max-w-[240px] md:px-[14px] md:text-[13px]">
                    {auth.user.displayName || auth.user.username}
                  </Link>
                  <Button onClick={() => void auth.signOut()} className="shrink-0">
                    Sign out
                  </Button>
                </div>
              ) : (
                <Button href={discordStartUrl("/account")} variant="primary" className="shrink-0">
                  Sign in with Discord
                </Button>
              )}
            </div>
          </div>

          <HeaderSearch ref={searchRef} />

          <div className="flex flex-wrap items-center gap-1.5 xl:hidden">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              title={item.title}
              className={({ isActive }) =>
                cn(
                  "rounded-full border border-transparent px-3 py-1.5 text-[12px] text-muted transition-colors",
                  "hover:border-line hover:bg-[rgba(121,201,151,0.08)] hover:text-text",
                  isActive && "border-line bg-[rgba(121,201,151,0.1)] text-text"
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
          </div>
        </div>
      </header>

      <main className="mx-auto min-h-0 w-[min(1380px,calc(100vw-20px))] px-0 py-3 pb-12 md:w-[min(1380px,calc(100vw-32px))] md:py-5 md:pb-16">
        {children}
      </main>

      <footer className="border-t border-line bg-[rgba(2,8,6,0.7)]">
        <div className="mx-auto flex w-[min(1380px,calc(100vw-24px))] flex-wrap items-center justify-between gap-3 py-4 text-[13px] text-muted md:w-[min(1380px,calc(100vw-32px))] md:text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-text">Support veryCrunchy</span>
            <span className="text-muted">if AimMod helps your practice.</span>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {supportLinks.map((link) => (
              <Button
                key={link.href}
                href={link.href}
                target="_blank"
                rel="noreferrer"
              >
                {link.label}
              </Button>
            ))}
          </div>
        </div>
      </footer>
    </div>
  );
}
