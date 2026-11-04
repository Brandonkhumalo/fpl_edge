import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { filterPlayers, type PlayerSort } from "../lib/playerCatalogue";
import type { Position, SquadPlayer } from "../lib/squad";

export function ScoutPage() {
 const season=useQuery({queryKey:["current-season"],queryFn:()=>api<{id:string;name:string}>("/seasons/current")});
 const players=useQuery({queryKey:["players",season.data?.id],queryFn:()=>api<{data:SquadPlayer[]}>(`/players?seasonId=${season.data!.id}`),enabled:!!season.data});
 const [search,setSearch]=useState("");const [club,setClub]=useState("");const [position,setPosition]=useState<Position | "ALL">("ALL");const [sort,setSort]=useState<PlayerSort>("most-selected");
 const clubs=Array.from(new Map((players.data?.data ?? []).map(p=>[p.clubId,p.clubFullName ?? p.clubName])).entries()).sort((a,b)=>a[1].localeCompare(b[1]));
 const visible=filterPlayers(players.data?.data ?? [],search,position,club,sort);
 return <div className="mx-auto max-w-6xl"><h1 className="text-3xl font-black">Player scout</h1><p className="my-3 text-slate-400">Explore players, prices and ownership for {season.data?.name ?? "the current season"}.</p>
 <div className="card my-5 grid gap-3 p-4 sm:grid-cols-2">
 <input className="field" aria-label="Search player or club" placeholder="Search player or club" value={search} onChange={e=>setSearch(e.target.value)} />
 <select className="field" aria-label="Club" value={club} onChange={e=>setClub(e.target.value)}><option value="">All clubs</option>{clubs.map(([id,name])=><option key={id} value={id}>{name}</option>)}</select>
 <select className="field" aria-label="Position" value={position} onChange={e=>setPosition(e.target.value as Position | "ALL")}>{["ALL","GKP","DEF","MID","FWD"].map(p=><option key={p}>{p}</option>)}</select>
 <select className="field" aria-label="Sort players" value={sort} onChange={e=>setSort(e.target.value as PlayerSort)}><option value="most-selected">Most selected</option><option value="price-asc">Price: low to high</option><option value="price-desc">Price: high to low</option><option value="default">Total points</option></select>
 </div>
 {season.isLoading || players.isLoading ? <p>Loading players…</p> : season.isError || players.isError ? <p role="alert">Player data could not be loaded. <button onClick={()=>{void season.refetch();void players.refetch();}}>Retry</button></p> : <><p className="mb-4 text-sm text-slate-400">{visible.length} players · <Link className="text-edge" to="/transfers">Go to transfers to change your squad →</Link></p><div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">{visible.map(p=><article className="card p-4" key={p.playerId}><h2 className="font-bold">{p.name}</h2><p className="my-2 text-sm text-slate-400">{p.clubFullName ?? p.clubName} · {p.position}</p><p>£{(p.currentPrice/10).toFixed(1)}m <span className="text-sm text-edge"> · {p.selectedByPercent==null?"Ownership unavailable":`${p.selectedByPercent.toFixed(1)}% selected`}</span></p></article>)}</div></>}
 </div>;
}
