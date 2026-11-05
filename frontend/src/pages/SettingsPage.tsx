import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
type Preferences={displayName:string;strategy:string;planningHorizon:number;timezone:string;deadlineReminderHours:number;inAppAlerts:boolean;emailAlerts:boolean;webPushAlerts:boolean};
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
  <label className="block">IANA timezone<input className="field mt-2" value={form.timezone} onChange={e=>setForm({...form,timezone:e.target.value})} placeholder="Africa/Johannesburg" /></label>
  <label className="block">Deadline reminder<select className="field mt-2" value={form.deadlineReminderHours} onChange={e=>setForm({...form,deadlineReminderHours:Number(e.target.value)})}>{[6,12,24,48,72].map(h=><option key={h} value={h}>{h} hours before</option>)}</select></label>
  <fieldset className="space-y-3"><legend className="font-bold">Alerts</legend><Toggle label="In-app alerts" checked={form.inAppAlerts} onChange={checked=>setForm({...form,inAppAlerts:checked})}/><Toggle label="Email delivery (queued; requires server adapter)" checked={form.emailAlerts} onChange={checked=>setForm({...form,emailAlerts:checked})}/><Toggle label="Web push (queued; requires server adapter)" checked={form.webPushAlerts} onChange={checked=>setForm({...form,webPushAlerts:checked})}/></fieldset>
  <p className="text-xs text-slate-400">Saved planner scenarios use this horizon when selected. Email and web-push jobs are retained safely but remain disabled until delivery adapters are configured.</p>
  {save.error && <p role="alert" className="text-red-300">{save.error.message}</p>}{save.isSuccess && <p role="status" className="text-edge">Settings saved.</p>}
  <button className="button-primary" disabled={save.isPending}>{save.isPending?"Saving…":"Save settings"}</button>
 </form>;
}
function Toggle({label,checked,onChange}:{label:string;checked:boolean;onChange:(checked:boolean)=>void}){return <label className="flex items-center gap-3 text-sm text-slate-300"><input type="checkbox" checked={checked} onChange={e=>onChange(e.target.checked)}/>{label}</label>}
