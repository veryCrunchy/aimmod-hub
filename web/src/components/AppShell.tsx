import { useEffect, useRef, useState, type PropsWithChildren } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { gameForPath, type HubGame } from "../lib/hubGame";
import { useAuth } from "../lib/AuthContext";
import { discordStartUrl } from "../lib/auth";
import { Button } from "./ui/Button";
import { HeaderSearch } from "./HeaderSearch";

const kovaaksGroups = [
  { label: "KovaaK's", links: [["/", "Overview"], ["/community", "Community"], ["/replays", "Replays"], ["/live", "Live activity"]] },
  { label: "Improve", links: [["/benchmarks", "Benchmarks"], ["/leaderboard", "Leaderboard"], ["/learn", "Learning library"]] },
  { label: "AimMod", links: [["/app/kovaaks", "Get AimMod for KovaaK's"], ["/app", "All downloads"]] },
];

const osuGroups = [
  { label: "Improve", links: [["/osu/pp-targets", "PP targets"], ["/osu/learn", "Knowledge base"]] },
  { label: "osu!", links: [["/osu", "Overview"], ["/osu/beatmaps", "Beatmaps"], ["/osu/skins", "Skins"], ["/osu/players", "Players"], ["/osu/replays", "Replay library"], ["/osu/community", "Community activity"]] },
  { label: "AimMod", links: [["/app/osu", "Get AimMod for osu!"], ["/app", "All downloads"]] },
];

export function AppShell({ children }: PropsWithChildren) {
  const auth = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [preferredGame, setPreferredGame] = useState<HubGame>("kovaaks");
  const game = gameForPath(location.pathname) ?? preferredGame;
  const groups = game === "osu" ? osuGroups : kovaaksGroups;
  const searchRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDetailsElement>(null);
  const isAdmin = Boolean(auth.user?.isAdmin ?? auth.isAdmin);

  useEffect(() => {
    const routeGame = gameForPath(location.pathname);
    try {
      if (routeGame) {
        localStorage.setItem("aimmod-hub-game", routeGame);
        setPreferredGame(routeGame);
      } else {
        setPreferredGame(localStorage.getItem("aimmod-hub-game") === "osu" ? "osu" : "kovaaks");
      }
    } catch { /* Browsing still works when storage is disabled. */ }
  }, [location.pathname]);

  useEffect(() => {
    if (menuRef.current) menuRef.current.open = false;
  }, [location.pathname]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const target = e.target as HTMLElement;
      const editing = target.isContentEditable || /INPUT|TEXTAREA|SELECT/.test(target.tagName);
      if (((e.ctrlKey || e.metaKey) && e.key === "k") || (e.key === "/" && !editing)) {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
      if (e.key === "Escape" && menuRef.current?.open) {
        menuRef.current.open = false;
        menuRef.current.querySelector("summary")?.focus();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  function navigation() {
    return <>
      {groups.map(group => <div className="hub-nav-group" key={group.label}>
        <span className="hub-nav-label">{group.label}</span>
        {group.links.map(([to, label]) => <NavLink key={to} to={to} end={to === "/" || to === "/app" || to === "/osu"}>{label}</NavLink>)}
      </div>)}
      <div className="hub-nav-group">
        <a href="/join" target="_blank" rel="noopener noreferrer">Join Discord</a>
        <span className="hub-nav-label">Account</span>
        <NavLink to="/account">Settings & devices</NavLink>
        {isAdmin && <NavLink to="/admin">Administration</NavLink>}
        {auth.authenticated && <button type="button" onClick={() => void auth.signOut()}>Sign out</button>}
      </div>
    </>;
  }

  return (
    <div className="hub-shell" data-game={game}>
      <a className="hub-skip" href="#main-content">Skip to content</a>
      <header className="hub-header">
        <Link to={game === "osu" ? "/osu" : "/"} className="hub-brand" aria-label="AimMod Hub home">
          <img className="hub-brand-wordmark" src="/brand/aimmod-v9/wordmark-white.svg" width="244" height="59" alt="" />
          <img className="hub-brand-mark" src="/brand/aimmod-v9/mark-mint.svg" width="44" height="31" alt="" />
          <span className="hub-brand-label" aria-hidden="true">Hub</span>
        </Link>
        <select className="hub-game-selector" aria-label="Game" value={game} onChange={e => { const next = e.target.value as HubGame; setPreferredGame(next); navigate(next === "osu" ? "/osu" : "/"); }}>
          <option value="osu">osu!</option><option value="kovaaks">KovaaK's</option>
        </select>
        <div className="hub-header-search"><HeaderSearch key={game} ref={searchRef} game={game} /></div>
        <div className="hub-account">
          <a className="hub-discord" href="/join" target="_blank" rel="noopener noreferrer" aria-label="Join AimMod on Discord" title="Join Discord">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor" aria-hidden="true">
              <path d="M20.317 4.37a19.792 19.792 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.211.375-.444.865-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.078.078 0 0 0-.079-.037A19.737 19.737 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.319 13.58.099 18.058a.082.082 0 0 0 .031.056 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.873-1.295 1.226-1.994a.076.076 0 0 0-.042-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128c.126-.094.252-.192.372-.291a.074.074 0 0 1 .078-.01c3.927 1.793 8.18 1.793 12.061 0a.074.074 0 0 1 .079.009c.12.1.246.198.372.292a.077.077 0 0 1-.006.128 12.3 12.3 0 0 1-1.873.891.077.077 0 0 0-.041.107c.36.699.772 1.362 1.225 1.993a.076.076 0 0 0 .084.029 19.84 19.84 0 0 0 6.003-3.03.077.077 0 0 0 .031-.055c.5-5.177-.838-9.674-3.548-13.66a.061.061 0 0 0-.031-.029ZM8.02 15.331c-1.182 0-2.157-1.086-2.157-2.419s.956-2.419 2.157-2.419c1.211 0 2.176 1.096 2.157 2.419 0 1.333-.956 2.419-2.157 2.419Zm7.975 0c-1.183 0-2.157-1.086-2.157-2.419s.955-2.419 2.157-2.419c1.21 0 2.175 1.096 2.157 2.419 0 1.333-.947 2.419-2.157 2.419Z" />
            </svg>
          </a>
          {auth.loading ? <span role="status" className="text-muted text-sm">Connecting...</span> : auth.authenticated && auth.user ? (
            <Link to={auth.user.profileHandle ? `/profiles/${auth.user.profileHandle}` : "/account"} className="hub-user">{auth.user.profileDisplayName || auth.user.displayName || auth.user.username}</Link>
          ) : <Button href={discordStartUrl("/account")}>Sign in</Button>}
        </div>
        <details className="hub-mobile-nav" ref={menuRef}>
          <summary>Menu</summary>
          <nav aria-label="Mobile navigation">{navigation()}</nav>
        </details>
      </header>
      <aside className="hub-sidebar"><nav aria-label="Primary">{navigation()}</nav></aside>
      <div className="hub-content">
        <main id="main-content" tabIndex={-1}>{children}</main>
        <footer className="hub-footer"><span>AimMod Hub</span><div><a href="/join" target="_blank" rel="noopener noreferrer">Join Discord</a><Link to="/branding">Branding</Link><a href="https://ko-fi.com/verycrunchy" target="_blank" rel="noreferrer">Support AimMod</a><a href="https://github.com/veryCrunchy/aimmod" target="_blank" rel="noreferrer">GitHub</a><a href="https://github.com/sponsors/veryCrunchy" target="_blank" rel="noreferrer">Sponsors</a></div></footer>
      </div>
    </div>
  );
}
