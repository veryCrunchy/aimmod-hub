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
  { label: "osu!", links: [["/osu", "Overview"], ["/osu/beatmaps", "Beatmaps"], ["/osu/players", "Players"], ["/osu/replays", "Replay library"], ["/osu/community", "Community activity"]] },
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
        <Link to={game === "osu" ? "/osu" : "/"} className="hub-brand" aria-label="AimMod Hub home"><img className="hub-brand-mark" src="/images/aimmod-logo.png" width="36" height="36" alt="" /><strong>AimMod <span>Hub</span></strong></Link>
        <select className="hub-game-selector" aria-label="Game" value={game} onChange={e => { const next = e.target.value as HubGame; setPreferredGame(next); navigate(next === "osu" ? "/osu" : "/"); }}>
          <option value="osu">osu!</option><option value="kovaaks">KovaaK's</option>
        </select>
        <div className="hub-header-search"><HeaderSearch key={game} ref={searchRef} game={game} /></div>
        <div className="hub-account">
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
        <footer className="hub-footer"><span>AimMod Hub</span><div><a href="https://ko-fi.com/verycrunchy" target="_blank" rel="noreferrer">Support AimMod</a><a href="https://github.com/veryCrunchy/aimmod" target="_blank" rel="noreferrer">GitHub</a><a href="https://github.com/sponsors/veryCrunchy" target="_blank" rel="noreferrer">Sponsors</a></div></footer>
      </div>
    </div>
  );
}
