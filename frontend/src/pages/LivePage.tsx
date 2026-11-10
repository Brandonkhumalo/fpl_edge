import { useQuery } from "@tanstack/react-query";
import { Activity, Clock3, ShieldCheck, Sparkles, Trophy } from "lucide-react";
import { api } from "../lib/api";

type LivePlayer = {
  playerId: string; name: string; club: string; position: string;
  isStarter: boolean; isCaptain: boolean; isViceCaptain: boolean;
  multiplier: number; points: number; countedPoints: number; minutes: number;
  bonus: number; bps: number; defensiveContribution: number; defensiveTarget: number;
  remaining: boolean; estimatedAutoSub: boolean;
};

type LiveGameweek = {
  gameweek: number; status: "scheduled" | "provisional" | "final";
  fetchedAt: string; stale: boolean; squadRevision: number; activeChip: string;
  points: number; captainPoints: number; captainExtra: number; benchPoints: number;
  estimatedBonus: number; remainingPlayers: number; remainingFixtures: number;
  estimatedAutoSubs: string[]; players: LivePlayer[]; miniLeagueMovement?: number;
  miniLeagueMessage: string; warnings: string[];
};

export function LivePage() {
  const query = useQuery({
    queryKey: ["live-gameweek"], queryFn: () => api<LiveGameweek>("/live"),
    refetchInterval: 60_000, retry: false
  });

  if (query.isLoading) return <div className="card p-8">Loading the latest Gameweek snapshot…</div>;
  if (query.isError) return <div className="card mx-auto max-w-xl p-8">
    <p className="text-sm font-bold text-edge">Live Gameweek</p>
    <h1 className="mt-2 text-2xl font-black">Live data is not ready</h1>
    <p className="my-4 text-slate-400">{query.error.message}</p>
    <p className="text-sm text-slate-500">The page fails closed when no synchronized provider snapshot exists; it never substitutes predicted points for live points.</p>
    <button className="button-primary mt-5" onClick={() => void query.refetch()}>Retry</button>
  </div>;

  const data = query.data!;
  const sorted = [...data.players].sort((first, second) => second.multiplier - first.multiplier || second.countedPoints - first.countedPoints);

  return <div className="mx-auto max-w-6xl space-y-6">
    <header className="rounded-3xl border border-edge/20 bg-gradient-to-br from-edge/10 to-panel p-6 sm:p-9">
      <div className="flex flex-wrap items-center justify-between gap-4"><div>
        <p className="text-xs font-bold uppercase tracking-[.2em] text-edge">GW {data.gameweek} · {data.status}</p>
        <h1 className="mt-3 text-3xl font-black">Live Gameweek</h1>
        <p className="mt-2 text-slate-400">Provider snapshot {new Date(data.fetchedAt).toLocaleTimeString()} · squad revision {data.squadRevision}{data.activeChip ? ` · ${data.activeChip} active` : ""}</p>
      </div><span className={`rounded-full px-3 py-1 text-xs font-bold ${data.status === "final" ? "bg-edge/10 text-edge" : "bg-amber-300/10 text-amber-200"}`}>{data.stale ? "Stale snapshot" : data.status === "final" ? "Final provider state" : "Provisional"}</span></div>
    </header>

    <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <Score icon={<Trophy />} label="Squad points" value={String(data.points)} />
      <Score icon={<Sparkles />} label="Captain total" value={String(data.captainPoints)} detail={`+${data.captainExtra} multiplier`} />
      <Score icon={<ShieldCheck />} label="Estimated bonus" value={String(data.estimatedBonus)} />
      <Score icon={<Clock3 />} label="Players remaining" value={String(data.remainingPlayers)} />
      <Score icon={<Activity />} label="Fixtures remaining" value={String(data.remainingFixtures)} />
    </section>

    {data.estimatedAutoSubs.length > 0 && <p className="rounded-xl border border-violet-300/20 bg-violet-300/5 p-4 text-sm text-violet-100">Estimated automatic substitutions: {data.estimatedAutoSubs.length}. These remain provisional until official processing.</p>}

    <section className="card overflow-hidden p-5">
      <div className="mb-4"><h2 className="text-xl font-black">Your live selection</h2><p className="mt-1 text-sm text-slate-400">Bench points: {data.benchPoints}. Bonus and defensive contributions can change during provider corrections.</p></div>
      <div className="overflow-x-auto"><table className="w-full whitespace-nowrap text-left text-sm">
        <thead className="text-xs text-slate-500"><tr><th className="p-3">Player</th><th className="p-3">Minutes</th><th className="p-3">Raw</th><th className="p-3">Multiplier</th><th className="p-3">Counted</th><th className="p-3">Bonus</th><th className="p-3">Defence</th><th className="p-3">State</th></tr></thead>
        <tbody>{sorted.map(player => <tr key={player.playerId} className="border-t border-white/5">
          <td className="p-3"><strong>{player.name}</strong><span className="ml-2 text-xs text-slate-500">{player.club} · {player.position}{player.isCaptain ? " · C" : player.isViceCaptain ? " · VC" : ""}</span></td>
          <td className="p-3">{player.minutes}</td><td className="p-3">{player.points}</td><td className="p-3">×{player.multiplier}</td><td className="p-3 font-black text-edge">{player.countedPoints}</td>
          <td className="p-3">{player.bonus} <span className="text-xs text-slate-500">({player.bps} BPS)</span></td><td className="p-3">{player.defensiveTarget ? `${player.defensiveContribution}/${player.defensiveTarget}` : "—"}</td>
          <td className="p-3 text-xs">{player.estimatedAutoSub ? <span className="text-violet-300">auto-sub in</span> : player.remaining ? <span className="text-amber-200">remaining</span> : data.status === "final" ? "final" : "played"}</td>
        </tr>)}</tbody>
      </table></div>
    </section>

    <section className="card p-5"><h2 className="font-black">Mini-league movement</h2><p className="mt-2 text-sm text-slate-400">{data.miniLeagueMovement == null ? data.miniLeagueMessage : `${data.miniLeagueMovement > 0 ? "+" : ""}${data.miniLeagueMovement} places`}</p></section>
    <footer className="rounded-xl border border-amber-300/20 p-4 text-xs leading-6 text-slate-400">{data.warnings.map(warning => <p key={warning}>{warning}</p>)}</footer>
  </div>;
}

function Score({ icon, label, value, detail }: { icon: React.ReactNode; label: string; value: string; detail?: string }) {
  return <article className="card p-5"><span className="text-edge">{icon}</span><p className="mt-3 text-xs uppercase tracking-wider text-slate-500">{label}</p><p className="mt-1 text-3xl font-black">{value}</p>{detail && <p className="text-xs text-slate-500">{detail}</p>}</article>;
}
