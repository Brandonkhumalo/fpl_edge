import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";

export const chipNames: Record<string,string> = { bboost: "Bench Boost", "3xc": "Triple Captain", freehit: "Free Hit", wildcard: "Wildcard" };
const descriptions: Record<string,string> = {
 bboost: "Include all four bench players alongside your starting XI in this gameweek's scoring selection.",
 "3xc": "Set your captain's scoring multiplier to three for this gameweek.",
 freehit: "Make unlimited transfers without hits. Your original players, bank and captaincy return when the next gameweek opens.",
 wildcard: "Make unlimited transfers without hits. Your new squad stays in place after this gameweek."
};
type Chip = { id: number; name: string; startGameweek: number; endGameweek: number; usedGameweek: number | null };
export type Tactics = { revision: number; gameweek: number; deadline: string | null; activeChip: string; chips: Chip[]; scoringMultipliers: Record<string,number> };

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
  {data.activeChip && <div className="card mb-5 border-edge/30 p-5"><strong className="text-edge">{chipNames[data.activeChip]} is active</strong><p className="mt-2 text-sm">{descriptions[data.activeChip]}</p><Link className="mt-3 inline-block text-edge" to="/my-squad">Manage this squad →</Link></div>}
  {notice && <p role="status" className="my-3 text-edge">{notice}</p>}
  {save.error && <p role="alert" className="my-3 text-red-300">{save.error.message}</p>}
  {!data.chips.length && <p className="card p-5">Chip availability is waiting for the current season's football data.</p>}
  <div className="grid gap-4 sm:grid-cols-2">{data.chips.map(chip => {
   const active=chip.usedGameweek===data.gameweek;
   const available=chip.usedGameweek===null && chip.startGameweek<=data.gameweek && chip.endGameweek>=data.gameweek && !data.activeChip && !closed;
   return <section key={chip.id} className="card p-5"><p className="text-xs text-edge">Gameweeks {chip.startGameweek}–{chip.endGameweek}</p><h2 className="my-2 text-xl font-bold">{chipNames[chip.name]}</h2><p className="min-h-20 text-sm leading-6 text-slate-400">{descriptions[chip.name]}</p>
    <p className="my-3 text-sm">{active ? "Active this gameweek" : chip.usedGameweek ? `Used in Gameweek ${chip.usedGameweek}` : chip.endGameweek<data.gameweek ? "Expired" : chip.startGameweek>data.gameweek ? "Available later this season" : "Unused"}</p>
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
