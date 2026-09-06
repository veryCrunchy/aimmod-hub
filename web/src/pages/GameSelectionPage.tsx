import { ArrowRight, Crosshair, Disc3, Download, Search, TrendingUp } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { PageSeo } from "../components/PageSeo";

export function GameSelectionPage() {
  return <div className="game-home">
    <PageSeo title="AimMod · Choose your game" description="Find osu! beatmaps for PP, explore replays and skins, or improve your KovaaK's practice with AimMod." />
    <header className="game-home-heading"><span className="home-eyebrow">YOUR NEXT GOOD SESSION</span><h1>What are you playing?</h1><p>Pick your game. Find your next challenge.</p></header>
    <div className="game-choices">
      <section className="game-choice game-choice-osu" aria-labelledby="osu-choice">
        <div className="game-choice-top"><span className="game-emblem"><Disc3 size={30} aria-hidden="true" /></span><span>RHYTHM & PRECISION</span></div>
        <h2 id="osu-choice">osu!</h2><p>Find your next PP play.</p><span className="game-description">Explore beatmaps, compare PP at your accuracy, and make the game your own.</span>
        <Link className="home-primary" to="/osu">Explore osu! <ArrowRight size={19} aria-hidden="true" /></Link>
        <div className="game-shortcuts"><Link to="/osu/pp-targets">Find PP beatmaps <ArrowRight size={16} /></Link><Link to="/osu/skin-builder">Build a skin <ArrowRight size={16} /></Link></div>
      </section>
      <section className="game-choice game-choice-kovaaks" aria-labelledby="kovaaks-choice">
        <div className="game-choice-top"><span className="game-emblem"><Crosshair size={30} aria-hidden="true" /></span><span>AIM & CONSISTENCY</span></div>
        <h2 id="kovaaks-choice">KovaaK’s</h2><p>Make your practice count.</p><span className="game-description">Review runs, compare benchmarks, and find the next skill to work on.</span>
        <Link className="home-primary" to="/kovaaks">Explore KovaaK’s <ArrowRight size={19} aria-hidden="true" /></Link>
        <div className="game-shortcuts"><Link to="/benchmarks">Benchmarks <ArrowRight size={16} /></Link><Link to="/replays">Watch replays <ArrowRight size={16} /></Link></div>
      </section>
    </div>
    <div className="home-bottom"><span><TrendingUp size={18} aria-hidden="true" /> A little more progress, every session.</span><Link to="/app"><Download size={17} aria-hidden="true" /> Get AimMod <ArrowRight size={16} aria-hidden="true" /></Link></div>
  </div>;
}

export function OsuQuickStart() {
  const navigate = useNavigate();
  return <section className="osu-start" aria-labelledby="osu-start-title">
    <div className="osu-start-heading"><div><span className="home-eyebrow">OSU! / FIND YOUR NEXT PLAY</span><h1 id="osu-start-title">A new personal best starts here.</h1><p>Find ranked beatmaps and compare full-combo PP at your accuracy.</p></div><Link to="/app/osu" className="home-download"><Download size={18} /> Get AimMod</Link></div>
    <form action="/osu/pp-targets" className="osu-quick-search" onSubmit={event => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const min = String(data.get("min") ?? "4");
      navigate(`/osu/pp-targets?${new URLSearchParams({ q: String(data.get("q") ?? ""), min, max: String(Number(min) + 2), acc: "98", mods: "NM", scoring: "lazer", sort: "pp" })}`);
    }}>
      <label className="osu-query">Beatmap search<div><Search size={19} aria-hidden="true" /><input name="q" type="search" placeholder="Song, artist or mapper" maxLength={256} /></div></label>
      <label>Star range<select name="min" defaultValue="4"><option value="2">2–4 ★</option><option value="3">3–5 ★</option><option value="4">4–6 ★</option><option value="5">5–7 ★</option><option value="6">6–8 ★</option></select></label>
      <input type="hidden" name="acc" value="98" />
      <button className="home-primary" type="submit">Find PP beatmaps <ArrowRight size={18} /></button>
    </form>
    <div className="osu-start-links"><Link to="/osu/beatmaps">Browse all beatmaps <ArrowRight size={16} /></Link><span>Or explore</span><Link to="/osu/skins">Skins</Link><Link to="/osu/replays">Replays</Link><Link to="/osu/learn">Learning guides</Link></div>
  </section>;
}
