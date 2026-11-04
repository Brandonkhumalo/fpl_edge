import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../lib/api";
import type { SquadPlayer } from "../lib/squad";

type Team = { revision: number; captainId: string; viceCaptainId: string; starters: string[]; players: SquadPlayer[] };

export function CaptaincyControls({ team }: { team: Team }) {
 const cache = useQueryClient();
 const [captain, setCaptain] = useState(team.captainId);
 const [vice, setVice] = useState(team.viceCaptainId);
 const save = useMutation({
  mutationFn: () => api<Team>("/squad/captaincy", { method: "POST", body: JSON.stringify({ expectedRevision: team.revision, captainId: captain, viceCaptainId: vice }) }),
  onSuccess: data => { cache.setQueryData(["squad"],data); void cache.invalidateQueries({ queryKey: ["tactics"] }); },
  onError: () => { void cache.invalidateQueries({ queryKey: ["squad"] }); }
 });
 const starters=team.players.filter(p => team.starters.includes(p.playerId));
 return <section className="card my-6 p-5"><h2 className="text-xl font-bold">Captain & vice-captain</h2>
  <p className="my-3 text-sm text-slate-400">Choose two different starters. Your vice-captain takes over if your captain does not play.</p>
  <div className="grid gap-4 sm:grid-cols-2">
   <label className="text-sm">Captain<select className="field mt-2" value={captain} disabled={save.isPending} onChange={e => setCaptain(e.target.value)}>{starters.map(p => <option key={p.playerId} value={p.playerId}>{p.name}</option>)}</select></label>
   <label className="text-sm">Vice-captain<select className="field mt-2" value={vice} disabled={save.isPending} onChange={e => setVice(e.target.value)}>{starters.map(p => <option key={p.playerId} value={p.playerId}>{p.name}</option>)}</select></label>
  </div>
  {captain===vice && <p className="mt-3 text-amber-200">Choose different players for captain and vice-captain.</p>}
  {save.error && <p role="alert" className="mt-3 text-red-300">{save.error.message}</p>}
  <button className="button-primary mt-4" disabled={captain===vice || save.isPending || (captain===team.captainId && vice===team.viceCaptainId)} onClick={() => save.mutate()}>{save.isPending ? "Saving…" : "Save captain & VC"}</button>
 </section>;
}
