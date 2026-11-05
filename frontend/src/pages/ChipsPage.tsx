import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";

import { ChipHistoryForm } from "../components/ChipHistoryForm";
import { chipNames, type Chip, type Tactics } from "../lib/chips";
export { chipNames, type Tactics } from "../lib/chips";
const descriptions: Record<string,string> = {
 bboost: "Include all four bench players alongside your starting XI in this gameweek's scoring selection.",
 "3xc": "Set your captain's scoring multiplier to three for this gameweek.",
 freehit: "Make unlimited transfers without hits. Your original players, bank and captaincy return when the next gameweek opens.",
 wildcard: "Make unlimited transfers without hits. Your new squad stays in place after this gameweek."
};

export function ChipsPage() {
 const cache=useQueryClient();
 const state=useQuery({ queryKey: ["tactics"], queryFn: () => api<Tactics>("/squad/tactics"), retry:false });
 const [confirm,setConfirm]=useState<Chip | null>(null);
 const dialog=useRef<HTMLDialogElement>(null);
 useEffect(() => { if(confirm) dialog.current?.showModal(); },[confirm]);
 const [notice,setNotice]=useState("");
 const save=useMutation({
  mutationFn: ({chip,cancel}:{chip:Chip;cancel:boolean}) => api<Tactics>("/squad/chip",{method:"POST",body:JSON.stringify({expectedRevision:state.data!.revision,chipId:chip.id,cancel})}),
  onSuccess: data => { cache.setQueryData(["tactics"],data); void cache.invalidateQueries({queryKey:["squad"]}); setConfirm(null); setNotice("Chip selection saved."); },
  onError: () => { void cache.invalidateQueries({queryKey:["tactics"]}); }
 });
 if(state.isLoading) return <p>Loading your chips…</p>;
 if(!state.data) return <div className="card p-6"><h1 className="text-2xl font-bold">Chip centre</h1><p className="my-4">{state.error?.message === "resource not found" ? "Create a squad to manage your chips." : "Unable to load chip availability."}</p><Link to="/my-squad" className="text-edge">Go to my squad</Link><button className="ml-4" onClick={() => void state.refetch()}>Retry</button></div>;
 const data=state.data;
 const closed=!data.deadline || Date.parse(data.deadline)<=Date.now();
 return <div className="mx-auto max-w-5xl"><h1 className="text-3xl font-black">Chip centre</h1>
  <p className="mt-3 text-slate-400">Planning for Gameweek {data.gameweek}{data.deadline && ` · Deadline ${new Date(data.deadline).toLocaleString()}`}</p>
  <p className="my-4 text-sm text-slate-400">One chip per gameweek. Changes affect your FPL Edge team only; activate the same chip separately on the official FPL site. Live points scoring is not available yet.</p>
  <p className="my-4 text-sm leading-6 text-slate-400">For 2026/27, all four chips have a first-half and second-half set. The second set starts in Gameweek 20; unused first-set chips expire after the Gameweek 19 deadline. Availability below follows your season's data. <a className="text-edge underline" href="https://www.premierleague.com/en/news/4679879/whats-happening-with-fpl-chips-in-202627" target="_blank" rel="noreferrer">Official FPL chip rules</a></p>
  {data.chips.length > 0 && <details className="mb-5" open={!data.historyReviewed || undefined}>
    <summary className="cursor-pointer font-semibold text-edge">{data.historyReviewed ? "Edit chips used before joining" : "Complete your chip history"}</summary>
    <ChipHistoryForm key={data.revision} data={data} onSaved={() => setNotice("Prior chip usage saved. Remaining chips have been updated.")} />
  </details>}
  {data.activeChip && <div className="card mb-5 border-edge/30 p-5"><strong className="text-edge">{chipNames[data.activeChip]} is active</strong><p className="mt-2 text-sm">{descriptions[data.activeChip]}</p><Link className="mt-3 inline-block text-edge" to="/my-squad">Manage this squad →</Link></div>}
  {notice && <p role="status" className="my-3 text-edge">{notice}</p>}
  {save.error && <p role="alert" className="my-3 text-red-300">{save.error.message}</p>}
  {!data.chips.length && <p className="card p-5">Chip availability is waiting for the current season's football data.</p>}
  <div className="grid gap-4 sm:grid-cols-2">{data.chips.map(chip => {
   const active=chip.usageSource==="fpl_edge" && chip.usedGameweek===data.gameweek;
   const available=!chip.used && chip.startGameweek<=data.gameweek && chip.endGameweek>=data.gameweek && !data.activeChip && !closed;
   return <section key={chip.id} className="card p-5"><p className="text-xs text-edge">Gameweeks {chip.startGameweek}–{chip.endGameweek}</p><h2 className="my-2 text-xl font-bold">{chipNames[chip.name]}</h2><p className="min-h-20 text-sm leading-6 text-slate-400">{descriptions[chip.name]}</p>
    <p className="my-3 text-sm">{active ? "Active this gameweek" : chip.used ? (chip.usedGameweek ? `Used in Gameweek ${chip.usedGameweek}` : "Used · gameweek not recorded") : chip.endGameweek<data.gameweek ? "Expired unused" : chip.startGameweek>data.gameweek ? "Available later this season" : "Unused"}{chip.usageSource === "prior" && <span className="block text-xs text-slate-400">Recorded from before joining</span>}</p>
    <button className="button-primary" disabled={!available || save.isPending} onClick={() => {setConfirm(chip);save.reset();}}>Use {chipNames[chip.name]}</button>
    {active && ["bboost","3xc"].includes(chip.name) && <button className="mt-3 block text-sm text-slate-300" disabled={closed || save.isPending} onClick={() => save.mutate({chip,cancel:true})}>Cancel chip</button>}
   </section>;
  })}</div>
  {confirm && <dialog ref={dialog} aria-label="Confirm chip" onCancel={e => { if(save.isPending) e.preventDefault(); else setConfirm(null); }} className="card m-auto w-[calc(100%-2rem)] max-w-lg p-6 text-slate-100 backdrop:bg-black/75"><h2 className="text-xl font-bold">Use {chipNames[confirm.name]} in Gameweek {data.gameweek}?</h2><p className="my-4 text-sm text-slate-300">{descriptions[confirm.name]}</p>
   {["wildcard","freehit"].includes(confirm.name) && <p className="mb-4 text-sm text-amber-200">Activate before making transfers this gameweek. Once activated, this chip cannot be cancelled.</p>}
   {save.error && <p role="alert" className="mb-4 text-red-300">{save.error.message}</p>}
   <button className="button-primary" disabled={save.isPending} onClick={() => save.mutate({chip:confirm,cancel:false})}>{save.isPending ? "Saving…" : "Confirm activation"}</button><button className="ml-4" disabled={save.isPending} onClick={() => setConfirm(null)}>Go back</button>
  </dialog>}
 </div>;
}
