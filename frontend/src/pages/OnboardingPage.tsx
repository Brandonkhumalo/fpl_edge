import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, ChevronRight, CircleAlert, LoaderCircle, Search, ShieldCheck, Sparkles, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { FootballPitch, SquadPlayerCard } from "../components/FootballPitch";
import { api } from "../lib/api";
import { filterPlayers, type PlayerSort } from "../lib/playerCatalogue";
import { buildLineup, formations, orderBench, squadRequirements, type Formation, type Position, type SquadPlayer } from "../lib/squad";

export function OnboardingPage() {
  const cache = useQueryClient();
  const navigate = useNavigate();
  const [selected, setSelected] = useState<SquadPlayer[]>([]);
  const [search, setSearch] = useState("");
  const [clubId, setClubId] = useState("");
  const [sort, setSort] = useState<PlayerSort>("default");
  const [position, setPosition] = useState<Position | "ALL">("ALL");
  const [formation, setFormation] = useState<Formation>("4-4-2");
  const [selectionMessage, setSelectionMessage] = useState("");

  const season = useQuery({ queryKey: ["current-season"], queryFn: () => api<{ id: string; name: string; currentGameweek: number }>("/seasons/current") });
  const players = useQuery({
    queryKey: ["players", season.data?.id],
    queryFn: () => api<{ data: SquadPlayer[] }>(`/players?seasonId=${season.data!.id}`),
    enabled: Boolean(season.data)
  });

  const total = selected.reduce((sum, player) => sum + player.currentPrice, 0);
  const counts = selected.reduce<Record<Position, number>>((result, player) => ({ ...result, [player.position]: result[player.position] + 1 }), { GKP: 0, DEF: 0, MID: 0, FWD: 0 });
  const clubCounts = selected.reduce<Record<string, number>>((result, player) => { result[player.clubId] = (result[player.clubId] ?? 0) + 1; return result; }, {});
  const starterIds = buildLineup(selected, formation);
  const starters = starterIds.map((id) => selected.find((player) => player.playerId === id)).filter((player): player is SquadPlayer => Boolean(player));
  const benchIds = orderBench(selected, starterIds);
  const bench = benchIds.map((id) => selected.find((player) => player.playerId === id)).filter((player): player is SquadPlayer => Boolean(player));
  const squadComplete = selected.length === 15 && Object.entries(squadRequirements).every(([key, required]) => counts[key as Position] === required);

  const clubs = useMemo(() => Array.from(new Map((players.data?.data ?? []).map((player) =>
    [player.clubId, player.clubFullName ?? player.clubName]
  )).entries()).sort((a, b) => a[1].localeCompare(b[1])), [players.data]);
  const visible = useMemo(() => filterPlayers(players.data?.data ?? [], search, position, clubId, sort),
    [players.data, position, search, clubId, sort]);

  const save = useMutation({
    mutationFn: async () => {
      const captainCandidates = starters.filter((player) => player.position === "MID" || player.position === "FWD");
      const captain = captainCandidates[0] ?? starters[1];
      const viceCaptain = captainCandidates.find((player) => player.playerId !== captain.playerId) ?? starters.find((player) => player.playerId !== captain.playerId)!;
      return api("/squad", {
        method: "POST",
        body: JSON.stringify({
          seasonId: season.data!.id,
          currentGameweek: season.data!.currentGameweek,
          bank: 1000 - total,
          freeTransfers: 1,
          playerIds: selected.map((player) => player.playerId),
          starters: starterIds,
          bench: benchIds,
          captainId: captain.playerId,
          viceCaptainId: viceCaptain.playerId
        })
      });
    },
    onSuccess: async () => {
      await cache.invalidateQueries({ queryKey: ["squad"] });
      navigate("/dashboard");
    }
  });

  const toggle = (player: SquadPlayer) => {
    setSelectionMessage("");
    if (selected.some((item) => item.playerId === player.playerId)) {
      setSelected(selected.filter((item) => item.playerId !== player.playerId));
      return;
    }
    const reason = selectionBlockReason(player, selected.length, counts, clubCounts, total);
    if (reason) {
      setSelectionMessage(reason);
      return;
    }
    setSelected([...selected, player]);
  };

  if (season.isLoading || players.isLoading) return <LoadingState />;
  if (season.isError || players.isError) return <DataUnavailable />;

  return <div className="mx-auto max-w-[1500px]">
    <header className="mb-7 flex flex-wrap items-end justify-between gap-5">
      <div>
        <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-edge/20 bg-edge/5 px-3 py-1.5 text-xs font-semibold text-edge"><Sparkles size={14} /> Your team, your decisions</div>
        <h1 className="text-3xl font-black tracking-tight sm:text-4xl">Build your FPL squad</h1>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-400">Choose 15 players, pick a legal FPL formation, and preview your starting XI and bench before saving.</p>
      </div>
      <div className="min-w-[220px] rounded-2xl border border-white/10 bg-white/[.03] p-4">
        <div className="flex items-center justify-between text-xs text-slate-400"><span>Squad progress</span><strong className="text-white">{selected.length}/15</strong></div>
        <div className="mt-2 h-2 overflow-hidden rounded-full bg-white/5"><div className="h-full rounded-full bg-edge transition-all" style={{ width: `${selected.length / 15 * 100}%` }} /></div>
      </div>
    </header>

    {season.data?.name.startsWith("Development") && <div className="mb-5 flex items-start gap-3 rounded-2xl border border-amber-300/15 bg-amber-300/5 px-4 py-3 text-sm text-amber-100"><CircleAlert className="mt-0.5 shrink-0 text-amber-300" size={18} /><p>Synthetic development players are shown locally. They are clearly separated from production FPL data.</p></div>}

    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_460px]">
      <section className="min-w-0">
        <div className="card mb-4 p-4">
          <div className="flex flex-col gap-3 sm:flex-row">
            <label className="relative min-w-0 flex-1"><Search className="absolute left-3 top-3.5 text-slate-500" size={18} /><input className="field pl-10" aria-label="Search players" placeholder="Search player or club" value={search} onChange={(event) => setSearch(event.target.value)} /></label>
            <div className="flex gap-1.5 overflow-x-auto pb-1 sm:pb-0">{(["ALL", "GKP", "DEF", "MID", "FWD"] as const).map((item) => <button key={item} onClick={() => setPosition(item)} className={`shrink-0 rounded-xl px-3 py-2.5 text-xs font-bold transition ${position === item ? "bg-edge text-ink" : "bg-white/5 text-slate-300 hover:bg-white/10"}`}>{item}</button>)}</div>
          </div>
          {selectionMessage && <p className="mt-3 flex items-center gap-2 text-sm text-amber-200"><CircleAlert size={16} />{selectionMessage}</p>}
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <label className="text-xs font-semibold text-slate-400">Club
              <select className="field mt-1.5" value={clubId} onChange={(event) => setClubId(event.target.value)}>
                <option value="">All clubs</option>
                {clubs.map(([id, name]) => <option key={id} value={id}>{name}</option>)}
              </select>
            </label>
            <label className="text-xs font-semibold text-slate-400">Sort players
              <select className="field mt-1.5" value={sort} onChange={(event) => setSort(event.target.value as PlayerSort)}>
                <option value="default">Total points</option>
                <option value="price-asc">Price: low to high</option>
                <option value="price-desc">Price: high to low</option>
                <option value="most-selected">Most selected</option>
              </select>
            </label>
          </div>
          <p className="mt-3 text-xs text-slate-400" aria-live="polite">{visible.length} players match your filters</p>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {visible.map((player) => {
            const chosen = selected.some((item) => item.playerId === player.playerId);
            const blocked = !chosen && Boolean(selectionBlockReason(player, selected.length, counts, clubCounts, total));
            return <button key={player.playerId} aria-pressed={chosen} onClick={() => toggle(player)} className={`group flex items-center gap-3 rounded-2xl border p-3.5 text-left transition ${chosen ? "border-edge/60 bg-edge/10 shadow-[0_0_24px_rgba(39,226,164,.08)]" : blocked ? "cursor-not-allowed border-white/5 bg-white/[.02] opacity-45" : "border-white/10 bg-panel/70 hover:-translate-y-0.5 hover:border-white/20 hover:bg-panel"}`}>
              <span className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl text-xs font-black ${chosen ? "bg-edge text-ink" : "bg-white/5 text-slate-300 group-hover:bg-edge/10 group-hover:text-edge"}`}>{chosen ? <Check size={18} /> : player.position}</span>
              <span className="min-w-0 flex-1"><span className="block truncate text-sm font-bold">{player.name}</span><span className="mt-0.5 block truncate text-xs text-slate-500">{player.clubName} · £{(player.currentPrice / 10).toFixed(1)}m</span></span>
              {sort === "most-selected" && <span className="shrink-0 text-right text-xs text-slate-400">{player.selectedByPercent == null ? "Unavailable" : `${player.selectedByPercent.toFixed(1)}%`}<span className="block text-[10px]">selected by</span></span>}
              {chosen && <X size={16} className="shrink-0 text-edge" />}
            </button>;
          })}
        </div>
        {visible.length === 0 && <div className="card p-10 text-center text-sm text-slate-400">No players match this search.</div>}
      </section>

      <aside className="h-fit space-y-4 xl:sticky xl:top-6">
        <div className="card overflow-hidden p-4 sm:p-5">
          <div className="mb-4 flex items-center justify-between gap-3"><div><p className="text-xs font-bold uppercase tracking-[.18em] text-edge">Starting XI</p><h2 className="mt-1 text-xl font-black">Formation {formation}</h2></div><span className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-slate-300">{starterIds.length}/11</span></div>
          <div className="mb-4 flex gap-2 overflow-x-auto pb-1">{formations.map((item) => <button key={item} onClick={() => setFormation(item)} className={`shrink-0 rounded-lg px-2.5 py-2 text-xs font-bold transition ${formation === item ? "bg-edge text-ink" : "bg-white/5 text-slate-400 hover:text-white"}`}>{item}</button>)}</div>
          <FootballPitch players={starters} formation={formation} compact />
          <div className="mt-4"><div className="mb-2 flex items-center justify-between"><p className="text-xs font-bold uppercase tracking-[.16em] text-slate-400">Bench</p><span className="text-xs text-slate-500">3 outfield + 1 goalkeeper</span></div><div className="grid grid-cols-4 gap-2">{Array.from({ length: 4 }).map((_, index) => bench[index] ? <SquadPlayerCard key={bench[index].playerId} player={bench[index]} badge={bench[index].position === "GKP" ? "GK" : `${index + 1}`} compact /> : <div key={index} className="grid min-h-[62px] place-items-center rounded-xl border border-dashed border-white/10 text-[10px] text-white/25">Bench</div>)}</div></div>
        </div>

        <div className="card p-5">
          <div className="flex items-center gap-3"><span className="grid h-10 w-10 place-items-center rounded-xl bg-edge/10 text-edge"><ShieldCheck size={20} /></span><div><h2 className="font-bold">FPL squad rules</h2><p className="text-xs text-slate-500">Checked here and on the server</p></div></div>
          <div className="mt-4 grid grid-cols-4 gap-2">{(Object.keys(squadRequirements) as Position[]).map((item) => <div key={item} className={`rounded-xl border p-2.5 text-center ${counts[item] === squadRequirements[item] ? "border-edge/25 bg-edge/5" : "border-white/5 bg-white/[.02]"}`}><p className="text-[10px] text-slate-500">{item}</p><p className={counts[item] === squadRequirements[item] ? "mt-1 font-black text-edge" : "mt-1 font-black"}>{counts[item]}/{squadRequirements[item]}</p></div>)}</div>
          <div className="mt-4 space-y-2 border-t border-white/10 pt-4 text-sm"><div className="flex justify-between"><span className="text-slate-400">Squad value</span><strong>£{(total / 10).toFixed(1)}m</strong></div><div className="flex justify-between"><span className="text-slate-400">Money remaining</span><strong className="text-edge">£{((1000 - total) / 10).toFixed(1)}m</strong></div><div className="flex justify-between"><span className="text-slate-400">Club limit</span><strong>Maximum 3</strong></div></div>
          {save.error && <p role="alert" className="mt-4 rounded-xl bg-red-500/10 p-3 text-sm text-red-300">{save.error.message}</p>}
          <button className="button-primary mt-5 w-full gap-2" disabled={!squadComplete || starterIds.length !== 11 || save.isPending} onClick={() => save.mutate()}>{save.isPending ? <><LoaderCircle className="animate-spin" size={18} /> Saving squad…</> : <>Save my squad <ChevronRight size={18} /></>}</button>
          {!squadComplete && <p className="mt-3 text-center text-xs text-slate-500">Complete all four position requirements to continue.</p>}
        </div>
      </aside>
    </div>
  </div>;
}

function selectionBlockReason(player: SquadPlayer, selectedCount: number, counts: Record<Position, number>, clubCounts: Record<string, number>, total: number): string {
  if (selectedCount >= 15) return "Your 15-player squad is already full.";
  if (counts[player.position] >= squadRequirements[player.position]) return `You already have the required number of ${player.position} players.`;
  if ((clubCounts[player.clubId] ?? 0) >= 3) return `FPL allows a maximum of three players from ${player.clubName}.`;
  if (total + player.currentPrice > 1000) return "This player would take your squad over the £100.0m budget.";
  return "";
}

function LoadingState() {
  return <div className="grid min-h-[65vh] place-items-center"><div className="text-center"><LoaderCircle className="mx-auto animate-spin text-edge" size={32} /><p className="mt-4 text-sm text-slate-400">Loading eligible players…</p></div></div>;
}

function DataUnavailable() {
  return <div className="grid min-h-[65vh] place-items-center"><div className="card max-w-lg p-9 text-center"><CircleAlert className="mx-auto text-amber-300" size={34} /><h1 className="mt-5 text-2xl font-black">Football data unavailable</h1><p className="mt-3 text-sm leading-6 text-slate-400">Squad setup is paused until a current season and eligible players are available. FPL Edge will not fabricate production player data.</p></div></div>;
}
