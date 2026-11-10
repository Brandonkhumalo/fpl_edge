import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, CheckCircle2, CircleAlert, Download, LoaderCircle, RefreshCw, ShieldCheck } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../lib/api";

type ImportPlayer = {
  playerId: string; providerPlayerId: number; name: string; clubName: string;
  position: string; currentPrice: number; lineupPosition: number; captain: boolean; viceCaptain: boolean;
};
type Difference = { kind: string; label: string; internal?: string; official: string };
type GameweekHistory = { gameweek: number; points: number; totalPoints: number; overallRank: number; bank: number; teamValue: number; transfers: number; transferCost: number; benchPoints: number };
type Preview = {
  id: string; expectedSquadRevision: number; purchasePricesUnavailable: boolean; expiresAt: string;
  players: ImportPlayer[]; differences: Difference[];
  official: {
    managerId: number; managerName: string; teamName: string; gameweek: number; bank: number; teamValue: number;
    freeTransfers: number; freeTransfersEstimated: boolean; activeChip: string; fetchedAt: string;
    gameweekHistory: GameweekHistory[]; chips: { name: string; gameweek: number }[];
  };
};

export function ImportSquadPage() {
  const [managerId, setManagerId] = useState("");
  const cache = useQueryClient();
  const navigate = useNavigate();
  const connected = useQuery({ queryKey: ["official-squad"], queryFn: () => api<{ managerId: number; lastReconciledAt: string }>("/official-squad"), retry: false });
  useEffect(() => { if (connected.data) setManagerId((current) => current || String(connected.data!.managerId)); }, [connected.data]);
  const preview = useMutation({
    mutationFn: () => api<Preview>("/official-squad/imports", { method: "POST", body: JSON.stringify({ managerId: Number(managerId) }) })
  });
  const apply = useMutation({
    mutationFn: () => api(`/official-squad/imports/${preview.data!.id}/apply`, { method: "POST", body: JSON.stringify({ expectedRevision: preview.data!.expectedSquadRevision }) }),
    onSuccess: async () => {
      await Promise.all([
        cache.invalidateQueries({ queryKey: ["squad"] }), cache.invalidateQueries({ queryKey: ["tactics"] }),
        cache.invalidateQueries({ queryKey: ["prediction"] }), cache.invalidateQueries({ queryKey: ["advice"] }),
        cache.invalidateQueries({ queryKey: ["official-squad"] })
      ]);
      navigate("/my-squad", { replace: true });
    }
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (/^[1-9][0-9]{0,9}$/.test(managerId)) preview.mutate();
  };

  return <div className="mx-auto max-w-6xl">
    <header className="mb-7">
      <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-edge/20 bg-edge/5 px-3 py-1.5 text-xs font-semibold text-edge"><Download size={14} /> Read-only official import</div>
      <h1 className="text-3xl font-black tracking-tight sm:text-4xl">Import and reconcile your FPL squad</h1>
      <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-400">Enter the numeric ID from your public FPL team URL. FPL Edge reads public squad data only—it never asks for your FPL password or session cookie and cannot submit official changes.</p>
      {connected.data && <p className="mt-3 text-sm text-edge">Connected to team ID {connected.data.managerId}. Last reconciled {new Date(connected.data.lastReconciledAt).toLocaleString()}.</p>}
    </header>

    <form className="card p-5 sm:flex sm:items-end sm:gap-4" onSubmit={submit}>
      <label className="block flex-1 text-sm font-semibold text-slate-300">FPL manager or team ID
        <input className="field mt-2" inputMode="numeric" pattern="[1-9][0-9]{0,9}" required placeholder="For example: 1234567" value={managerId} onChange={(event) => { setManagerId(event.target.value.trim()); preview.reset(); apply.reset(); }} />
      </label>
      <button className="button-primary mt-4 min-w-48 gap-2 sm:mt-0" disabled={preview.isPending || !/^[1-9][0-9]{0,9}$/.test(managerId)}>{preview.isPending ? <><LoaderCircle className="animate-spin" size={18} /> Fetching…</> : <><RefreshCw size={18} /> Compare squad</>}</button>
    </form>
    {preview.error && <p role="alert" className="mt-4 rounded-xl border border-red-400/20 bg-red-500/10 p-4 text-sm text-red-200">{preview.error.message}</p>}

    {preview.data && <PreviewPanel data={preview.data} applying={apply.isPending} applyError={apply.error?.message} onApply={() => apply.mutate()} />}
  </div>;
}

function PreviewPanel({ data, applying, applyError, onApply }: { data: Preview; applying: boolean; applyError?: string; onApply: () => void }) {
  const starters = data.players.filter((player) => player.lineupPosition <= 11);
  const bench = data.players.filter((player) => player.lineupPosition > 11);
  return <div className="mt-6 space-y-6">
    <section className="card p-5">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div><p className="text-xs font-bold uppercase tracking-[.16em] text-edge">Official gameweek {data.official.gameweek}</p><h2 className="mt-1 text-2xl font-black">{data.official.teamName || `Team ${data.official.managerId}`}</h2><p className="mt-1 text-sm text-slate-400">{data.official.managerName} · ID {data.official.managerId}</p></div>
        <div className="grid grid-cols-3 gap-2 text-center text-sm">
          <Metric label="Bank" value={`£${(data.official.bank / 10).toFixed(1)}m`} />
          <Metric label="Team value" value={`£${(data.official.teamValue / 10).toFixed(1)}m`} />
          <Metric label="Free transfers*" value={String(data.official.freeTransfers)} />
        </div>
      </div>
      <div className="mt-5 flex items-start gap-3 rounded-xl border border-amber-300/15 bg-amber-300/5 p-4 text-sm leading-6 text-amber-100"><CircleAlert className="mt-0.5 shrink-0 text-amber-300" size={18} /><p>*The public API does not expose the exact free-transfer balance or purchase prices. Free transfers are derived from public transfer and chip history. Player purchase/selling prices will use current prices, so review future transfer proceeds carefully.</p></div>
    </section>

    <section className="card p-5"><h2 className="text-xl font-bold">Differences to review</h2><p className="mt-1 text-sm text-slate-400">Nothing is changed until you apply this preview.</p><div className="mt-4 divide-y divide-white/10">{data.differences.map((difference, index) => <div key={`${difference.kind}-${index}`} className="grid gap-2 py-3 text-sm sm:grid-cols-[180px_1fr_24px_1fr] sm:items-center"><strong>{difference.label}</strong><span className="text-slate-400">{difference.internal || "—"}</span><ArrowRight className="hidden text-slate-600 sm:block" size={16} /><span className="font-semibold text-edge">{difference.official}</span></div>)}</div></section>

    <section className="grid gap-5 lg:grid-cols-[1fr_320px]">
      <div className="card p-5"><h2 className="text-xl font-bold">Official selection</h2><PlayerGroup title="Starting XI" players={starters} /><PlayerGroup title="Bench order" players={bench} /></div>
      <div className="space-y-5">
        <div className="card p-5"><h2 className="font-bold">Chip history</h2>{data.official.chips.length ? <ul className="mt-3 space-y-2 text-sm">{data.official.chips.map((chip) => <li key={`${chip.name}-${chip.gameweek}`} className="flex justify-between"><span className="capitalize text-slate-300">{chip.name}</span><strong>GW{chip.gameweek}</strong></li>)}</ul> : <p className="mt-3 text-sm text-slate-400">No used chips reported.</p>}</div>
        <div className="card p-5"><h2 className="font-bold">Recent history</h2><div className="mt-3 space-y-2">{data.official.gameweekHistory.slice(-5).reverse().map((week) => <div key={week.gameweek} className="flex items-center justify-between rounded-lg bg-white/[.03] px-3 py-2 text-sm"><span className="text-slate-400">GW{week.gameweek}</span><strong>{week.points} pts</strong><span className="text-slate-400">OR {week.overallRank.toLocaleString()}</span></div>)}</div></div>
      </div>
    </section>

    <section className="card flex flex-col gap-4 border-edge/20 p-5 sm:flex-row sm:items-center sm:justify-between"><div className="flex items-start gap-3"><ShieldCheck className="mt-0.5 shrink-0 text-edge" size={22} /><div><h2 className="font-bold">Apply inside FPL Edge</h2><p className="mt-1 text-sm text-slate-400">This replaces the internal squad shown above. It does not make changes on the official FPL website.</p></div></div><button className="button-primary min-w-48 gap-2" disabled={applying} onClick={onApply}>{applying ? <><LoaderCircle className="animate-spin" size={18} /> Applying…</> : <><CheckCircle2 size={18} /> Apply imported squad</>}</button></section>
    {applyError && <p role="alert" className="rounded-xl border border-red-400/20 bg-red-500/10 p-4 text-sm text-red-200">{applyError}</p>}
  </div>;
}

function PlayerGroup({ title, players }: { title: string; players: ImportPlayer[] }) {
  return <div className="mt-5"><h3 className="mb-2 text-xs font-bold uppercase tracking-[.16em] text-slate-500">{title}</h3><div className="grid gap-2 sm:grid-cols-2">{players.map((player) => <div key={player.playerId} className="flex items-center gap-3 rounded-xl border border-white/5 bg-white/[.025] p-3"><span className="grid h-9 w-9 place-items-center rounded-lg bg-white/5 text-[10px] font-black text-slate-300">{player.position}</span><div className="min-w-0 flex-1"><p className="truncate text-sm font-bold">{player.name} {player.captain ? "(C)" : player.viceCaptain ? "(VC)" : ""}</p><p className="text-xs text-slate-500">{player.clubName} · £{(player.currentPrice / 10).toFixed(1)}m</p></div></div>)}</div></div>;
}

function Metric({ label, value }: { label: string; value: string }) { return <div className="min-w-24 rounded-xl bg-white/[.04] p-3"><p className="text-[10px] text-slate-500">{label}</p><p className="mt-1 font-black">{value}</p></div>; }
