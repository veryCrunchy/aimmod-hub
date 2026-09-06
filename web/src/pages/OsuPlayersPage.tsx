import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { PageSeo } from "../components/PageSeo";
import { SectionHeader } from "../components/SectionHeader";
import { PageStack } from "../components/ui/Stack";
import { PageSection } from "../components/ui/PageSection";
import { Button } from "../components/ui/Button";
import { Skeleton } from "../components/ui/Skeleton";
import { EmptyState } from "../components/ui/EmptyState";
import { API_BASE_URL } from "../lib/config";
import type { OsuPublicProfile } from "../lib/osuCommunity";

type PlayersPage = { items: OsuPublicProfile[]; nextPage: number };
export function OsuPlayersPage() {
 const [params,setParams]=useSearchParams();
 const query=params.get("q")??"";
 const requestedMode=params.get("mode")??"osu";
 const mode=["osu","taiko","fruits","mania"].includes(requestedMode)?requestedMode:"osu";
 const page=Math.max(1,Math.min(200,Math.floor(Number(params.get("page")))||1));
 const [input,setInput]=useState(query);
 const [result,setResult]=useState<PlayersPage|null>(null);
 const [error,setError]=useState(false);
 const [attempt,setAttempt]=useState(0);
 useEffect(()=>{setInput(query)},[query]);
 useEffect(()=>{
  const controller=new AbortController();setResult(null);setError(false);
  const search=new URLSearchParams({q:query,mode,page:String(page)});
  fetch(`${API_BASE_URL}/api/osu/v1/players?${search}`,{signal:controller.signal})
   .then(response=>{if(!response.ok)throw new Error("Players unavailable");return response.json() as Promise<PlayersPage>})
   .then(value=>{if(!controller.signal.aborted)setResult(value)})
   .catch(()=>{if(!controller.signal.aborted)setError(true)});
  return ()=>controller.abort();
 },[query,mode,page,attempt]);
 const update=(key:string,value:string)=>setParams(previous=>{const next=new URLSearchParams(previous);next.set(key,value);if(key!=="page")next.delete("page");return next});
 return <PageStack>
  <PageSeo title="osu! players · AimMod Hub" description="Find public osu! players, rankings, scores and replays." />
  <PageSection>
   <SectionHeader level={1} eyebrow="osu!" title="Players" body="Explore public players, rankings, scores and replays." />
   <form className="hub-filters" onSubmit={event=>{event.preventDefault();update("q",input.trim())}}>
    <label>Find a player<input type="search" placeholder="osu! username or user ID" value={input} maxLength={64} onChange={event=>setInput(event.target.value)}/></label>
    <label>Ruleset<select value={mode} onChange={event=>update("mode",event.target.value)}><option value="osu">osu!</option><option value="taiko">osu!taiko</option><option value="fruits">osu!catch</option><option value="mania">osu!mania</option></select></label>
    <Button type="submit">Search players</Button>
    {query&&<Button type="button" onClick={()=>{setInput("");update("q","")}}>Clear search</Button>}
   </form>
   {error?<EmptyState title="Players are temporarily unavailable" body="Please try again in a moment."><Button onClick={()=>setAttempt(value=>value+1)}>Try again</Button></EmptyState>
    :!result?<div role="status" aria-label="Loading players"><Skeleton className="h-64"/></div>
    :result.items.length===0?<EmptyState title="No players found" body="Try another username."/>
    :<div className="divide-y divide-line border-y border-line">{result.items.map(player=><Link key={player.osuUserId} to={`/osu/profiles/${player.osuUserId}?mode=${mode}`} className="grid grid-cols-[48px_minmax(0,1fr)_auto] items-center gap-3 py-4 hover:bg-panel">
     {player.avatarUrl?<img src={player.avatarUrl} alt="" loading="lazy" className="h-12 w-12 rounded-md object-cover"/>:<span className="h-12 w-12 rounded-md bg-white/5"/>}
     <span className="min-w-0"><strong className="block truncate text-sm">{player.osuUsername}</strong><span className="text-xs text-muted">{player.countryCode}{player.globalRank?` · #${player.globalRank.toLocaleString()}`:""}</span></span>
     <span className="text-sm text-gold">{player.performancePoints==null?"View profile":`${Math.round(player.performancePoints).toLocaleString()}pp`}</span>
    </Link>)}</div>}
   {result&&<nav aria-label="Player pages" className="mt-5 flex items-center gap-4"><Button disabled={page<=1} onClick={()=>update("page",String(page-1))}>Previous</Button><span className="text-sm text-muted">Page {page}</span><Button disabled={!result.nextPage} onClick={()=>update("page",String(result.nextPage))}>Next</Button></nav>}
  </PageSection>
 </PageStack>;
}
