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
  const orb1Ref = useRef<HTMLDivElement>(null);
  const orb2Ref = useRef<HTMLDivElement>(null);
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

  // Keyboard shortcut to focus search
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

  // Cursor-following ambient orbs
  useEffect(() => {
    let rafId: number;

    // Orbit centers — drift with mouse and stay wherever they end up
    let cx1 = 30, cy1 = 35;
    let cx2 = 68, cy2 = 62;

    // Orbit angles (radians)
    let a1 = 0, a2 = Math.PI;

    // Smoothed mouse velocity (% of viewport per frame)
    let mvx = 0, mvy = 0;
    let prevX = -1, prevY = -1;

    const onMove = (e: MouseEvent) => {
      if (prevX >= 0) {
        const dx = ((e.clientX - prevX) / window.innerWidth) * 100;
        const dy = ((e.clientY - prevY) / window.innerHeight) * 100;
        mvx = mvx * 0.55 + dx * 0.45;
        mvy = mvy * 0.55 + dy * 0.45;
      }
      prevX = e.clientX;
      prevY = e.clientY;
    };

    const tick = () => {
      // Decay velocity each frame
      mvx *= 0.88;
      mvy *= 0.88;

      // Drift orbit centers with mouse — different rates for parallax
      cx1 = Math.max(10, Math.min(90, cx1 + mvx * 0.10));
      cy1 = Math.max(10, Math.min(90, cy1 + mvy * 0.10));
      cx2 = Math.max(10, Math.min(90, cx2 + mvx * 0.06));
      cy2 = Math.max(10, Math.min(90, cy2 + mvy * 0.06));

      // Advance orbit angles — opposite directions
      a1 += 0.0018;
      a2 -= 0.0013;

      // Orbit radii (% of viewport)
      const rx1 = 4, ry1 = 3;
      const rx2 = 3, ry2 = 2.5;

      const x1 = cx1 + rx1 * Math.cos(a1);
      const y1 = cy1 + ry1 * Math.sin(a1);
      const x2 = cx2 + rx2 * Math.cos(a2);
      const y2 = cy2 + ry2 * Math.sin(a2);

      const vw = window.innerWidth;
      const vh = window.innerHeight;

      if (orb1Ref.current) {
        const size = vw * 0.52;
        orb1Ref.current.style.transform =
          `translate(${(x1 / 100) * vw - size / 2}px, ${(y1 / 100) * vh - size / 2}px)`;
      }
      if (orb2Ref.current) {
        const size = vw * 0.58;
        orb2Ref.current.style.transform =
          `translate(${(x2 / 100) * vw - size / 2}px, ${(y2 / 100) * vh - size / 2}px)`;
      }

      rafId = requestAnimationFrame(tick);
    };

    document.addEventListener("mousemove", onMove, { passive: true });
    rafId = requestAnimationFrame(tick);
    return () => {
      document.removeEventListener("mousemove", onMove);
      cancelAnimationFrame(rafId);
    };
  }, []);

  return (
    <div className="min-h-screen overflow-x-clip">
      {/* Cursor-following ambient orbs — fixed layer behind all content */}
      <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden>
        <div
          ref={orb1Ref}
          className="absolute left-0 top-0 rounded-full"
          style={{
            width: "52vw",
            height: "52vw",
            background: "radial-gradient(circle, rgba(121,201,151,0.06) 0%, rgba(121,201,151,0.02) 38%, transparent 68%)",
            filter: "blur(52px)",
            willChange: "transform",
          }}
        />
        <div
          ref={orb2Ref}
          className="absolute left-0 top-0 rounded-full"
          style={{
            width: "58vw",
            height: "58vw",
            background: "radial-gradient(circle, rgba(194,169,255,0.05) 0%, rgba(184,255,225,0.02) 40%, transparent 68%)",
            filter: "blur(72px)",
            willChange: "transform",
          }}
        />
      </div>

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
