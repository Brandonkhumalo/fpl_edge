import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowRightLeft, Crown, LoaderCircle, Sparkles, Users, WalletCards } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { FootballPitch, SquadPlayerCard } from "../components/FootballPitch";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { inferFormation, type SquadPlayer } from "../lib/squad";

type Squad = {
  revision: number;
  bank: number;
  freeTransfers: number;
  currentGameweek: number;
  players: SquadPlayer[];
  starters: string[];
  bench: string[];
  captainId: string;
  viceCaptainId: string;
};

export function DashboardPage() {
  const user = useAuth((state) => state.user);
  const squad = useQuery({ queryKey: ["squad"], queryFn: () => api<Squad>("/squad"), retry: false });
  if (squad.isLoading) return <State icon={<LoaderCircle className="animate-spin" />} title="Loading your command centre" text="Fetching the latest saved squad revision." />;
  if (squad.isError) return <State icon={<Users />} title="Build your first squad" text="Select 15 players, choose an FPL formation, and save your starting XI and bench." action={<Link className="button-primary" to="/onboarding">Build my squad</Link>} />;

  const data = squad.data!;
  const starters = data.starters.map((id) => data.players.find((player) => player.playerId === id)).filter((player): player is SquadPlayer => Boolean(player));
  const bench = data.bench.map((id) => data.players.find((player) => player.playerId === id)).filter((player): player is SquadPlayer => Boolean(player));
  const captain = data.players.find((player) => player.playerId === data.captainId);
  const formation = inferFormation(data.players, data.starters);
  const projectedPoints = starters.reduce((sum, player) => sum + (player.expectedPoints ?? 0), 0);

  return <div className="mx-auto max-w-[1450px]">
    <header className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div><p className="text-sm font-semibold text-edge">Gameweek {data.currentGameweek} command centre</p><h1 className="mt-1 text-3xl font-black tracking-tight sm:text-4xl">Welcome back, {user?.displayName}</h1><p className="mt-2 text-sm text-slate-400">Your saved {formation} squad · Revision {data.revision}</p></div>
      <div className="flex gap-3"><Link to="/onboarding" className="button-primary gap-2"><ArrowRightLeft size={17} /> Manage team</Link></div>
    </header>

    <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Metric label="Formation" value={formation} icon={<Users />} />
      <Metric label="Projected points" value={projectedPoints > 0 ? projectedPoints.toFixed(1) : "Pending"} icon={<Sparkles />} />
      <Metric label="Money in bank" value={`£${(data.bank / 10).toFixed(1)}m`} icon={<WalletCards />} />
      <Metric label="Captain" value={captain?.name ?? "Not set"} icon={<Crown />} />
    </section>

    <section className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,1.45fr)_380px]">
      <div className="card overflow-hidden p-5 sm:p-7">
        <div className="mb-5 flex items-center justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[.2em] text-edge">Your team</p><h2 className="mt-1 text-2xl font-black">Starting XI · {formation}</h2></div><span className="rounded-full border border-edge/20 bg-edge/5 px-3 py-1.5 text-xs font-semibold text-edge">11 starters</span></div>
        <FootballPitch players={starters} formation={formation} captainId={data.captainId} viceCaptainId={data.viceCaptainId} />
        <div className="mt-5"><div className="mb-3 flex items-center justify-between"><div><p className="text-xs font-bold uppercase tracking-[.18em] text-slate-400">Substitutes</p><h3 className="mt-1 font-bold">Your bench</h3></div><span className="text-xs text-slate-500">Ordered left to right</span></div><div className="grid grid-cols-2 gap-3 sm:grid-cols-4">{bench.map((player, index) => <SquadPlayerCard key={player.playerId} player={player} badge={player.position === "GKP" ? "GK" : `${index + 1}`} />)}</div></div>
      </div>

      <div className="space-y-4">
        <div className="card p-5"><div className="flex items-center justify-between"><div><p className="text-xs font-bold uppercase tracking-[.18em] text-edge">Squad status</p><h3 className="mt-2 text-xl font-black">Ready for Gameweek {data.currentGameweek}</h3></div><span className="grid h-11 w-11 place-items-center rounded-xl bg-edge/10 text-edge"><Sparkles /></span></div><div className="mt-5 space-y-3 border-t border-white/10 pt-4 text-sm"><Detail label="Players" value="15 / 15" /><Detail label="Starting XI" value="11 players" /><Detail label="Bench" value="4 players" /><Detail label="Free transfers" value={String(data.freeTransfers)} /></div></div>
        <Recommendation title="Captaincy" value={captain?.name ?? "Awaiting selection"} text="Captain and vice-captain are selected only from your saved starting XI." />
        <Recommendation title="Transfer watch" value="No forced transfer" text="FPL Edge will recommend activity only when a legal move improves your expected outcome." />
        <div className="card border-amber-400/20 p-5"><div className="flex gap-3"><AlertTriangle className="shrink-0 text-amber-300" /><div><h3 className="font-bold">Live data boundary</h3><p className="mt-1 text-sm leading-6 text-slate-400">Availability and fixture advice is withheld when provider data is stale or unavailable. Statistics are never invented.</p></div></div></div>
      </div>
    </section>
  </div>;
}

function Metric({ label, value, icon }: { label: string; value: string; icon: ReactNode }) {
  return <div className="card p-5"><div className="flex items-center justify-between text-slate-500"><p className="text-xs font-semibold uppercase tracking-wider">{label}</p><span className="text-edge">{icon}</span></div><p className="mt-4 truncate text-2xl font-black">{value}</p></div>;
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="flex justify-between"><span className="text-slate-400">{label}</span><strong>{value}</strong></div>;
}

function Recommendation({ title, value, text }: { title: string; value: string; text: string }) {
  return <div className="card p-5"><p className="text-xs font-bold uppercase tracking-[.18em] text-edge">{title}</p><h3 className="mt-2 text-xl font-black">{value}</h3><p className="mt-2 text-sm leading-6 text-slate-400">{text}</p></div>;
}

function State({ icon, title, text, action }: { icon: ReactNode; title: string; text: string; action?: ReactNode }) {
  return <div className="grid min-h-[65vh] place-items-center"><div className="card max-w-md p-9 text-center"><div className="mx-auto mb-5 grid h-12 w-12 place-items-center rounded-xl bg-edge/10 text-edge">{icon}</div><h1 className="text-2xl font-black">{title}</h1><p className="mb-6 mt-3 text-sm leading-6 text-slate-400">{text}</p>{action}</div></div>;
}
