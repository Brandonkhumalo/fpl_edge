import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Bookmark, CalendarDays, Search, Sparkles, TrendingUp, Users } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { TeamShirt } from "../components/FootballPitch";
import { api } from "../lib/api";
import { useAdvice, type Prediction } from "../lib/advice";
import { differentialOwnershipLimit, rankScoutPlayers, type ScoutSort, type ScoutView } from "../lib/scout";
import type { Position, SquadPlayer } from "../lib/squad";

const pageSize = 24;
const positions: Array<Position | "ALL"> = ["ALL", "GKP", "DEF", "MID", "FWD"];

type Squad = { revision: number; players: SquadPlayer[] };

export function ScoutPage() {
  const season = useQuery({ queryKey: ["current-season"], queryFn: () => api<{ id: string; name: string }>("/seasons/current") });
  const catalogue = useQuery({
    queryKey: ["players", season.data?.id],
    queryFn: () => api<{ data: SquadPlayer[] }>(`/players?seasonId=${season.data!.id}`),
    enabled: Boolean(season.data)
  });
  const squad = useQuery({ queryKey: ["squad"], queryFn: () => api<Squad>("/squad"), retry: false });
  const advice = useAdvice(squad.data?.revision);
  const [search, setSearch] = useState("");
  const [clubId, setClubId] = useState("");
  const [position, setPosition] = useState<Position | "ALL">("ALL");
  const [view, setView] = useState<ScoutView>("top");
  const [sort, setSort] = useState<ScoutSort>("points-desc");
  const [limit, setLimit] = useState(pageSize);

  const forecasts = useMemo(() => new Map((advice.data?.players ?? []).map(player => [player.playerId, player])), [advice.data]);
  const players = useMemo(() => (catalogue.data?.data ?? []).map(player => ({
    ...player,
    nextFiveTotal: forecasts.get(player.playerId)?.nextFiveTotal
  })), [catalogue.data, forecasts]);
  const clubs = useMemo(() => Array.from(new Map(players.map(player => [player.clubId, player.clubFullName ?? player.clubName])).entries())
    .sort((first, second) => first[1].localeCompare(second[1])), [players]);
  const visible = useMemo(() => rankScoutPlayers(players, { search, position, clubId, view, sort }), [players, search, position, clubId, view, sort]);
  const owned = useMemo(() => new Set(squad.data?.players.map(player => player.playerId) ?? []), [squad.data]);
  const topTalent = useMemo(() => rankScoutPlayers(players, { search: "", position: "ALL", clubId: "", view: "top", sort: "points-desc" })[0], [players]);
  const topDifferential = useMemo(() => rankScoutPlayers(players, { search: "", position: "ALL", clubId: "", view: "differentials", sort: "points-desc" })[0], [players]);

  useEffect(() => setLimit(pageSize), [search, position, clubId, view, sort]);

  function chooseView(nextView: ScoutView) {
    setView(nextView);
    if (nextView === "top" || nextView === "differentials") setSort("points-desc");
  }

  if (season.isLoading || catalogue.isLoading) return <div className="card p-8" role="status">Loading the player market…</div>;
  if (season.isError || catalogue.isError) return <div className="card mx-auto max-w-lg p-8"><h1 className="text-2xl font-black">Player data is unavailable</h1><p className="my-4 text-sm text-slate-400">The current FPL catalogue could not be loaded.</p><button className="button-primary" onClick={() => { void season.refetch(); void catalogue.refetch(); }}>Retry</button></div>;

  return <div className="mx-auto max-w-[1500px] space-y-6">
    <header className="rounded-3xl border border-edge/20 bg-gradient-to-br from-edge/10 via-panel to-panel p-6 sm:p-9">
      <p className="text-xs font-bold uppercase tracking-[.22em] text-edge">Player market · {season.data?.name}</p>
      <h1 className="mt-3 text-3xl font-black tracking-tight sm:text-4xl">Scout points, value and the next five.</h1>
      <p className="mt-3 max-w-3xl leading-7 text-slate-300">Find proven FPL scorers, uncover productive low-owned differentials, compare price, and inspect each player’s next five scheduled matches before targeting a transfer.</p>
    </header>

    <section className="grid gap-4 md:grid-cols-3">
      <SummaryCard icon={<TrendingUp />} label="Top points scorer" player={topTalent} value={topTalent ? `${topTalent.totalPoints ?? 0} pts` : "Unavailable"} />
      <SummaryCard icon={<Sparkles />} label={`Top differential · ≤${differentialOwnershipLimit}%`} player={topDifferential} value={topDifferential ? `${topDifferential.totalPoints ?? 0} pts` : "Unavailable"} />
      <div className="card p-5"><div className="flex items-center justify-between"><p className="text-xs font-bold uppercase tracking-[.16em] text-slate-400">Forecast status</p><CalendarDays className="text-edge" size={20} /></div><p className="mt-3 text-xl font-black">{advice.isLoading ? "Calculating…" : advice.data ? `GW ${advice.data.gameweek}–${advice.data.horizonEnd}` : "Current points only"}</p><p className="mt-2 text-xs leading-5 text-slate-500">{advice.data ? "Next-five estimates use the latest validated fixture snapshot." : "Fixture forecasts appear when fresh provider data and a saved squad are available."}</p></div>
    </section>

    <section className="card p-4 sm:p-5" aria-label="Scout filters">
      <div className="flex flex-wrap gap-2">{(["top", "differentials", "all"] as ScoutView[]).map(item => <button key={item} onClick={() => chooseView(item)} className={`rounded-xl px-4 py-2.5 text-sm font-bold capitalize transition ${view === item ? "bg-edge text-ink" : "bg-white/5 text-slate-300 hover:bg-white/10"}`}>{item === "top" ? "Top talent" : item}</button>)}</div>
      <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(220px,1.4fr)_repeat(3,minmax(150px,.7fr))]">
        <label className="relative"><span className="sr-only">Search player or club</span><Search className="absolute left-3 top-3.5 text-slate-500" size={18} /><input className="field pl-10" placeholder="Search player or club" value={search} onChange={event => setSearch(event.target.value)} /></label>
        <select className="field" aria-label="Position" value={position} onChange={event => setPosition(event.target.value as Position | "ALL")}><option value="ALL">All positions</option>{positions.slice(1).map(item => <option key={item} value={item}>{item}</option>)}</select>
        <select className="field" aria-label="Club" value={clubId} onChange={event => setClubId(event.target.value)}><option value="">All clubs</option>{clubs.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select>
        <select className="field" aria-label="Sort players" value={sort} onChange={event => setSort(event.target.value as ScoutSort)}>
          <option value="points-desc">Points: high to low</option>
          <option value="forecast-desc">Next 5 xPts: high to low</option>
          <option value="price-desc">Price: high to low</option>
          <option value="price-asc">Price: low to high</option>
          <option value="ownership-asc">Ownership: low to high</option>
          <option value="ownership-desc">Ownership: high to low</option>
        </select>
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500"><span>{visible.length} matching players</span>{view === "differentials" && <span>Differential = 10% ownership or less with at least one FPL point</span>}</div>
    </section>

    {advice.isError && <p className="rounded-xl border border-amber-300/20 bg-amber-300/5 p-4 text-sm text-amber-100">Next-five forecasts are temporarily unavailable: {advice.error.message}. Current points, ownership, price, and transfer targeting remain available.</p>}

    <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {visible.slice(0, limit).map(player => <PlayerCard key={player.playerId} player={player} forecast={forecasts.get(player.playerId)} owned={owned.has(player.playerId)} hasSquad={Boolean(squad.data)} />)}
    </section>
    {visible.length === 0 && <div className="card p-10 text-center text-slate-400">No players match these scout filters.</div>}
    {limit < visible.length && <div className="text-center"><button className="button-secondary" onClick={() => setLimit(current => current + pageSize)}>Show more players</button></div>}
    <footer className="rounded-2xl border border-white/10 p-5 text-xs leading-6 text-slate-500">Current points and ownership come from the synchronized FPL catalogue. Next-five points are experimental estimates, not guaranteed returns. Selecting a player opens an internal FPL Edge transfer plan; it does not make an official FPL transfer.</footer>
  </div>;
}

function SummaryCard({ icon, label, player, value }: { icon: ReactNode; label: string; player?: SquadPlayer; value: string }) {
  return <div className="card p-5"><div className="flex items-center justify-between"><p className="text-xs font-bold uppercase tracking-[.16em] text-slate-400">{label}</p><span className="text-edge">{icon}</span></div><p className="mt-3 text-xl font-black">{player?.name ?? "No qualifying player"}</p><p className="mt-1 text-sm text-edge">{value}{player?.selectedByPercent != null ? ` · ${player.selectedByPercent.toFixed(1)}% owned` : ""}</p></div>;
}

function PlayerCard({ player, forecast, owned, hasSquad }: { player: SquadPlayer; forecast?: Prediction; owned: boolean; hasSquad: boolean }) {
  const cache = useQueryClient();
  const watch = useMutation({ mutationFn: () => api(`/watchlist/${player.playerId}`, { method: "POST" }), onSuccess: () => void cache.invalidateQueries({ queryKey: ["watchlist"] }) });
  const fixtures = forecast?.fixtures.slice(0, 5) ?? [];
  return <article className="card overflow-hidden p-5">
    <div className="flex items-start gap-4"><TeamShirt player={player} compact={false} /><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><h2 className="truncate text-lg font-black">{player.name}</h2>{owned && <span className="rounded-full bg-edge/10 px-2 py-0.5 text-[10px] font-bold text-edge">Owned</span>}{player.selectedByPercent != null && player.selectedByPercent <= differentialOwnershipLimit && <span className="rounded-full bg-violet-400/10 px-2 py-0.5 text-[10px] font-bold text-violet-300">Differential</span>}</div><p className="mt-1 text-xs text-slate-400">{player.clubFullName ?? player.clubName} · {player.position}{player.squadNumber ? ` · #${player.squadNumber}` : ""}</p></div></div>
    <div className="mt-5 grid grid-cols-4 gap-2 text-center"><Stat label="FPL points" value={String(player.totalPoints ?? 0)} /><Stat label="Next 5 xPts" value={forecast ? forecast.nextFiveTotal.toFixed(1) : "—"} /><Stat label="Ownership" value={player.selectedByPercent == null ? "—" : `${player.selectedByPercent.toFixed(1)}%`} /><Stat label="Price" value={`£${(player.currentPrice / 10).toFixed(1)}m`} /></div>
    <div className="mt-5"><div className="mb-2 flex items-center justify-between"><p className="text-[10px] font-bold uppercase tracking-[.16em] text-slate-500">Next five matches</p>{forecast && <span className="text-[10px] text-slate-500">{forecast.coverage}/5 scheduled</span>}</div><div className="grid grid-cols-5 gap-1.5">{Array.from({ length: 5 }, (_, index) => {
      const fixture = fixtures[index];
      return fixture ? <div key={fixture.fixtureId} title={`${fixture.opponent} ${fixture.home ? "home" : "away"} · ${fixture.points.toFixed(1)} expected points`} className={`rounded-lg border p-2 text-center ${difficultyClass(fixture.difficulty)}`}><p className="truncate text-[10px] font-black">{fixture.opponent}</p><p className="mt-0.5 text-[9px] opacity-75">{fixture.home ? "H" : "A"} · GW{fixture.gameweek}</p><p className="mt-1 text-[9px] font-bold">{fixture.points.toFixed(1)}</p></div> : <div key={index} className="grid min-h-[58px] place-items-center rounded-lg border border-white/5 bg-white/[.02] text-[9px] text-slate-600">TBC</div>;
    })}</div></div>
    <div className="mt-5 flex gap-2 border-t border-white/10 pt-4"><button className="button-secondary px-3" disabled={watch.isPending||watch.isSuccess} onClick={()=>watch.mutate()} title="Add to watchlist"><Bookmark size={16}/>{watch.isSuccess?"Watching":"Watch"}</button>{owned ? <Link to="/my-squad" className="inline-flex flex-1 items-center justify-center gap-2 text-sm font-bold text-edge">Manage owned player <ArrowRight size={15} /></Link> : hasSquad ? <Link to={`/transfers?target=${encodeURIComponent(player.playerId)}`} className="button-primary flex-1 gap-2">Target for transfer <ArrowRight size={16} /></Link> : <Link to="/onboarding" className="button-primary flex-1 gap-2">Build a squad <Users size={16} /></Link>}</div>
  </article>;
}

function Stat({ label, value }: { label: string; value: string }) {
  return <div className="rounded-xl bg-white/[.035] px-1 py-2.5"><p className="text-[9px] uppercase text-slate-500">{label}</p><p className="mt-1 truncate text-sm font-black">{value}</p></div>;
}

function difficultyClass(difficulty: number) {
  if (difficulty <= 2) return "border-edge/20 bg-edge/10 text-emerald-100";
  if (difficulty >= 4) return "border-red-400/20 bg-red-400/10 text-red-100";
  return "border-amber-300/15 bg-amber-300/5 text-amber-100";
}
