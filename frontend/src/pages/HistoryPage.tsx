import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
const names:Record<string,string>={"squad.created":"Squad created","transfer.confirmed":"Transfer confirmed","lineup.substituted":"Substitution saved","captaincy.updated":"Captain & vice-captain changed","chip.updated":"Chip selection changed","gameweek.advanced":"Squad advanced to the next gameweek"};
export function HistoryPage() {
 const history=useQuery({queryKey:["squad-history"],queryFn:()=>api<{type:string;revision:number;at:string}[]>("/squad/history")});
 return <div className="mx-auto max-w-3xl"><h1 className="text-3xl font-black">Team history</h1><p className="my-4 text-slate-400">Your latest 100 saved squad changes.</p>
 {history.isLoading?<p>Loading history…</p>:history.isError?<p role="alert">History could not be loaded.</p>:!history.data?.length?<p className="card p-5">No squad changes yet.</p>:<ol className="space-y-3">{history.data.map((event,i)=><li key={i} className="card p-4"><p className="font-bold">{names[event.type] ?? "Squad updated"}</p><p className="mt-2 text-sm text-slate-400">{new Date(event.at).toLocaleString()} · Revision {event.revision}</p></li>)}</ol>}
 </div>;
}
