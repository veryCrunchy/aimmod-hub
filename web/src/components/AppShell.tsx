import type { FormEvent, PropsWithChildren } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { cn } from "../lib/cn";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import { Button } from "./ui/Button";

const navItems = [
  { to: "/", label: "Home" },
  { to: "/community", label: "Community" },
  { to: "/account", label: "Account" }
];

const supportLinks = [
  { href: "https://ko-fi.com/verycrunchy", label: "Ko-fi" },
  { href: "https://github.com/sponsors/veryCrunchy", label: "GitHub Sponsors" }
];

export function AppShell({ children }: PropsWithChildren) {
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const query = String(form.get("q") ?? "").trim();
    if (!query) {
      return;
    }
    navigate(`/search?q=${encodeURIComponent(query)}`);
  }

  return (
    <div className="min-h-screen overflow-x-clip">
      <header className="sticky top-0 z-10 border-b border-line bg-[linear-gradient(180deg,rgba(2,8,6,0.94),rgba(4,12,9,0.9))] px-6 py-[18px] backdrop-blur-xl">
        <div className="mx-auto grid w-[min(1380px,100%)] grid-cols-[auto_auto_minmax(280px,420px)_auto] items-center gap-5 max-[1260px]:grid-cols-[auto_1fr] max-[1260px]:gap-4 max-[860px]:grid-cols-1">
        <Link to="/" className="grid gap-1">
          <span className="text-[11px] uppercase tracking-[0.12em] text-cyan">AimMod Hub</span>
          <span className="text-[13px] text-text">shared practice intelligence</span>
        </Link>

          <nav className="flex flex-wrap items-center gap-2.5 max-[1260px]:order-3" aria-label="Primary">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  cn(
                    "rounded-full border border-transparent px-[14px] py-2.5 text-sm text-muted transition-colors",
                    "hover:border-line hover:bg-[rgba(121,201,151,0.08)] hover:text-text",
                    isActive && "border-line bg-[rgba(121,201,151,0.1)] text-text"
                  )
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          <form onSubmit={handleSearch} className="flex min-w-0 items-center gap-2 max-[1260px]:order-2">
            <input
              key={location.pathname === "/search" ? location.search : "global-search"}
              name="q"
              defaultValue={location.pathname === "/search" ? new URLSearchParams(location.search).get("q") ?? "" : ""}
              placeholder="Search"
              className="min-w-0 flex-1 rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-4 py-2 text-sm text-text outline-none transition-colors placeholder:text-muted focus:border-mint/70"
            />
            <Button type="submit" className="min-h-9 shrink-0 px-3 py-2">
              Go
            </Button>
          </form>

          <div className="flex flex-wrap items-center justify-end gap-3 max-[1260px]:order-4 max-[1260px]:justify-start">
          {auth.loading ? null : auth.authenticated && auth.user ? (
            <div className="inline-flex items-center gap-2">
              <Link to="/account" className="rounded-full border border-line bg-[rgba(255,255,255,0.03)] px-[14px] py-2.5 text-[13px] text-text">
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
        </div>
      </header>

      <main className="mx-auto min-h-0 w-[min(1380px,calc(100vw-40px))] px-0 py-[34px] pb-20 max-[720px]:w-[min(1380px,calc(100vw-24px))] max-[720px]:pt-5">
        {children}
      </main>

      <footer className="border-t border-line bg-[rgba(2,8,6,0.7)]">
        <div className="mx-auto flex w-[min(1380px,calc(100vw-40px))] flex-wrap items-center justify-between gap-4 py-5 text-sm text-muted max-[720px]:w-[min(1380px,calc(100vw-24px))]">
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
                className="min-h-10"
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
