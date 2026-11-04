import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { FootballPitch } from "../components/FootballPitch";
import { CaptaincyControls } from "../components/CaptaincyControls";
import { chipNames, type Tactics } from "./ChipsPage";
import { api } from "../lib/api";
import { filterPlayers, type PlayerSort } from "../lib/playerCatalogue";
import { formations, inferFormation, type SquadPlayer } from "../lib/squad";

type OwnedPlayer = SquadPlayer & { purchasePrice: number };
type Squad = { seasonId: string; revision: number; bank: number; freeTransfers: number; players: OwnedPlayer[]; starters: string[]; bench: string[]; captainId: string; viceCaptainId: string };
type Action = { kind: "transfer" | "substitute"; player: OwnedPlayer };

export function ManageTeamPage() {
  const cache = useQueryClient();
  const squad = useQuery({ queryKey: ["squad"], queryFn: () => api<Squad>("/squad"), retry: false });
  const tactics = useQuery({ queryKey: ["tactics"], queryFn: () => api<Tactics>("/squad/tactics"), retry: false });
  const [action, setAction] = useState<Action | null>(null);
  const [replacement, setReplacement] = useState("");
  const [search, setSearch] = useState("");
  const [club, setClub] = useState("");
  const [sort, setSort] = useState<PlayerSort>("price-asc");
  const [notice, setNotice] = useState("");
  const catalogue = useQuery({
    queryKey: ["players", squad.data?.seasonId],
    queryFn: () => api<{ data: SquadPlayer[] }>(`/players?seasonId=${squad.data!.seasonId}`),
    enabled: action?.kind === "transfer" && Boolean(squad.data?.seasonId)
  });
  const change = useMutation({
    mutationFn: () => {
      const data = squad.data!;
      const outgoing = action!.player.playerId;
      const isStarter = data.starters.includes(outgoing);
      return api<Squad>(action!.kind === "transfer" ? "/transfers/confirm" : "/squad/substitute", {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify(action!.kind === "transfer"
          ? { expectedRevision: data.revision, playerOutId: outgoing, playerInId: replacement }
          : { expectedRevision: data.revision, starterId: isStarter ? outgoing : replacement, benchId: isStarter ? replacement : outgoing })
      });
    },
    onSuccess: (data) => {
      cache.setQueryData(["squad"], data);
      void cache.invalidateQueries({ queryKey: ["tactics"] });
      setNotice(action?.kind === "transfer" ? "Transfer saved." : "Substitution saved.");
      setAction(null); setReplacement("");
    },
    onError: () => { void cache.invalidateQueries({ queryKey: ["squad"] }); }
  });
  if (squad.isLoading) return <p>Loading your saved squad…</p>;
  if (!squad.data) return <div className="card p-6"><h1 className="text-2xl font-bold">Your squad</h1><p className="my-4">{squad.error?.message === "resource not found" ? "Build your first squad to start managing your team." : "Your squad could not be loaded. Please try again."}</p><button className="button-primary" onClick={() => void squad.refetch()}>Retry</button> <Link to="/onboarding" className="text-edge">Build squad</Link></div>;
  const data = squad.data;
  const activeChip = tactics.data?.activeChip ?? "";
  const unlimited = activeChip === "wildcard" || activeChip === "freehit";
  const starters = data.starters.map(id => data.players.find(p => p.playerId === id)!);
  const sale = action ? Math.min(action.player.currentPrice, action.player.purchasePrice + Math.floor(Math.max(0, action.player.currentPrice - action.player.purchasePrice) / 2)) : 0;
  const candidates = action?.kind === "transfer"
    ? filterPlayers(catalogue.data?.data ?? [], search, action.player.position, club, sort).filter(p =>
      !data.players.some(owned => owned.playerId === p.playerId) && p.currentPrice <= data.bank + sale &&
      data.players.filter(owned => owned.playerId !== action.player.playerId && owned.clubId === p.clubId).length < 3)
    : action ? data.players.filter(p => {
      const outgoingStarter = data.starters.includes(action.player.playerId);
      if (data.starters.includes(p.playerId) === outgoingStarter) return false;
      const ids = data.starters.map(id => id === (outgoingStarter ? action.player.playerId : p.playerId) ? (outgoingStarter ? p.playerId : action.player.playerId) : id);
      const next = ids.map(id => data.players.find(owned => owned.playerId === id)!);
      const formation = ["DEF", "MID", "FWD"].map(pos => next.filter(p => p.position === pos).length).join("-");
      return next.filter(p => p.position === "GKP").length === 1 && formations.some(f => f === formation);
    }) : [];
  const incoming = candidates.find(p => p.playerId === replacement);
  const clubs = Array.from(new Map((catalogue.data?.data ?? []).map(p => [p.clubId, p.clubFullName ?? p.clubName])).entries()).sort((a,b) => a[1].localeCompare(b[1]));
  function begin(kind: Action["kind"], player: OwnedPlayer) {
    setAction({ kind, player }); setReplacement(""); setSearch(""); setClub(""); setNotice(""); change.reset();
  }
  return <div className="mx-auto max-w-6xl">
    <h1 className="text-3xl font-black">Manage my team</h1>
    <p className="mt-3 text-slate-400">£{(data.bank / 10).toFixed(1)}m in bank · {data.freeTransfers} free transfers · {inferFormation(data.players, data.starters)}</p>
    <p className="mt-2 text-sm text-slate-400">Transfers replace an owned player. Substitutions swap your starting XI and bench for free. Changes apply to your FPL Edge team only.</p>
    {notice && <p role="status" className="my-4 text-edge">{notice}</p>}
    <div className="card my-5 p-4"><Link className="font-bold text-edge" to="/chips">{activeChip ? `${chipNames[activeChip]} active · Manage chips →` : "Use Bench Boost, Triple Captain, Free Hit or Wildcard →"}</Link>{activeChip === "3xc" && <p className="mt-2 text-sm">Captain multiplier: ×3</p>}{activeChip === "bboost" && <p className="mt-2 text-sm">All 15 players included in your scoring selection.</p>}</div>
    <CaptaincyControls key={data.revision} team={data} />
    <div className="my-6 max-w-2xl"><FootballPitch players={starters} formation={inferFormation(data.players,data.starters)} captainId={data.captainId} viceCaptainId={data.viceCaptainId} /></div>
    {action && <section className="card mb-6 p-5" aria-label="Player change">
      <h2 className="text-xl font-bold">{action.kind === "transfer" ? "Transfer out" : "Substitute"} {action.player.name}</h2>
      <p className="my-3 text-sm text-slate-400">{action.kind === "transfer" ? `Available: £${((data.bank + sale) / 10).toFixed(1)}m. ${unlimited ? "Unlimited chip transfers: no points hit or free transfer used." : `This transfer uses ${data.freeTransfers > 0 ? "one free transfer" : "a 4-point hit"}.`}` : "Choose a player below. The captain or vice-captain role follows the incoming starter when applicable."}</p>
      {action.kind === "transfer" && <div className="grid gap-3 sm:grid-cols-3">
        <input className="field" aria-label="Search replacements" placeholder="Search player or club" value={search} onChange={e => setSearch(e.target.value)} />
        <select className="field" aria-label="Club" value={club} onChange={e => setClub(e.target.value)}><option value="">All clubs</option>{clubs.map(([id,name]) => <option key={id} value={id}>{name}</option>)}</select>
        <select className="field" aria-label="Sort replacements" value={sort} onChange={e => setSort(e.target.value as PlayerSort)}><option value="price-asc">Price: low to high</option><option value="price-desc">Price: high to low</option><option value="most-selected">Most selected</option></select>
      </div>}
      {action.kind === "transfer" && catalogue.isLoading ? <p className="my-3">Loading replacements…</p> : action.kind === "transfer" && catalogue.isError ? <p role="alert">Could not load replacements. <button onClick={() => void catalogue.refetch()}>Retry</button></p> : <label className="my-4 block">Choose replacement
        <select className="field mt-2" value={replacement} disabled={change.isPending} onChange={e => setReplacement(e.target.value)}><option value="">Select a player</option>{candidates.map(p => <option key={p.playerId} value={p.playerId}>{p.name} · {p.clubName} · {p.position} · £{(p.currentPrice / 10).toFixed(1)}m</option>)}</select>
        {!candidates.length && <span className="text-sm text-amber-200">No eligible players match these filters and squad rules.</span>}
      </label>}
      {incoming && <p className="my-3">{action.player.name} → {incoming.name}{action.kind === "transfer" && ` · Bank after: £${((data.bank + sale - incoming.currentPrice) / 10).toFixed(1)}m`}</p>}
      {change.error && <p role="alert" className="my-3 text-red-300">{change.error.message}</p>}
      <button className="button-primary" disabled={!incoming || change.isPending} onClick={() => change.mutate()}>{change.isPending ? "Saving…" : action.kind === "transfer" ? "Confirm transfer" : "Confirm substitution"}</button>
      <button className="ml-4 text-slate-400" disabled={change.isPending} onClick={() => setAction(null)}>Cancel</button>
    </section>}
    {(["Starting XI", "Bench"] as const).map((label,index) => <section key={label} className="mb-6"><h2 className="mb-3 text-xl font-bold">{label}</h2><div className="grid gap-3 sm:grid-cols-2">
      {(index === 0 ? data.starters : data.bench).map(id => { const player=data.players.find(p => p.playerId === id)!; return <div key={id} className="card p-4">
        <h3 className="font-bold">{player.name} {id === data.captainId ? "(C)" : id === data.viceCaptainId ? "(VC)" : ""}</h3>
        <p className="my-2 text-sm text-slate-400">{player.position} · {player.clubName} · £{(player.currentPrice / 10).toFixed(1)}m</p>
        <button disabled={change.isPending} className="rounded-lg bg-edge/10 px-3 py-2 text-sm text-edge" onClick={() => begin("transfer",player)}>Transfer</button>
        <button disabled={change.isPending} className="ml-3 rounded-lg border border-white/10 px-3 py-2 text-sm" onClick={() => begin("substitute",player)}>Substitute</button>
      </div>; })}
    </div></section>)}
  </div>;
}
