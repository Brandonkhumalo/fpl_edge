import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
type Preferences={displayName:string;strategy:string;planningHorizon:number};
export function SettingsPage() {
 const query=useQuery({queryKey:["settings"],queryFn:()=>api<Preferences>("/settings")});
 return <div className="mx-auto max-w-2xl"><h1 className="mb-5 text-3xl font-black">Settings</h1>{query.data ? <SettingsForm initial={query.data} /> : <p>{query.isError?"Settings could not be loaded.":"Loading settings…"}</p>}</div>;
}
function SettingsForm({initial}:{initial:Preferences}) {
 const [form,setForm]=useState(initial);const cache=useQueryClient();
 const user=useAuth(s=>s.user);
 const save=useMutation({mutationFn:()=>api<Preferences>("/settings",{method:"PUT",body:JSON.stringify(form)}),onSuccess:data=>{
  cache.setQueryData(["settings"],data);
  const auth=useAuth.getState();if(auth.user && auth.accessToken)auth.setSession({user:{...auth.user,displayName:data.displayName},accessToken:auth.accessToken});
 }});
 return <form className="card space-y-5 p-6" onSubmit={e=>{e.preventDefault();save.mutate();}}>
  <p className="text-sm text-slate-400">Account: {user?.email}</p>
  <label className="block">Display name<input required minLength={2} maxLength={80} className="field mt-2" value={form.displayName} onChange={e=>setForm({...form,displayName:e.target.value})} /></label>
  <label className="block">Preferred strategy<select className="field mt-2" value={form.strategy} onChange={e=>setForm({...form,strategy:e.target.value})}>{["conservative","balanced","aggressive"].map(s=><option key={s}>{s}</option>)}</select></label>
  <label className="block">Planning horizon<select className="field mt-2" value={form.planningHorizon} onChange={e=>setForm({...form,planningHorizon:Number(e.target.value)})}>{Array.from({length:8},(_,i)=><option key={i+1} value={i+1}>{i+1} gameweeks</option>)}</select></label>
  <p className="text-xs text-slate-400">Strategy and horizon are saved preferences. Automated advice is not available yet.</p>
  {save.error && <p role="alert" className="text-red-300">{save.error.message}</p>}{save.isSuccess && <p role="status" className="text-edge">Settings saved.</p>}
  <button className="button-primary" disabled={save.isPending}>{save.isPending?"Saving…":"Save settings"}</button>
 </form>;
}
