import { ArrowRightLeft, Bell, Bookmark, Crown, ShieldCheck, Search, Settings, History, LayoutDashboard, LogOut, Menu, Route, Users, X } from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

const nav = [
  ["Dashboard", "/dashboard", LayoutDashboard], ["Weekly advisor", "/advice", Crown], ["My squad", "/my-squad", Users],
  ["Captain & VC", "/captaincy", Crown], ["Transfers", "/transfers", ArrowRightLeft], ["Chip centre", "/chips", ShieldCheck],
  ["Player scout", "/scout", Search], ["Watchlist", "/watchlist", Bookmark], ["Deadline centre", "/deadline", Bell],
  ["Transfer planner", "/planner", Route], ["Outcomes", "/recommendations", History], ["Team history", "/my-squad/history", History], ["Settings", "/settings", Settings]
] as const;

export function AppShell() {
  const [open, setOpen] = useState(false); const navigate = useNavigate(); const clear = useAuth((s) => s.clear);
  const logout = async () => { try { await api("/auth/logout", { method: "POST" }); } finally { clear(); navigate("/login"); } };
  return <div className="min-h-screen lg:grid lg:grid-cols-[260px_1fr]">
    <button className="fixed left-4 top-4 z-50 rounded-xl border border-white/10 bg-panel p-2 lg:hidden" onClick={() => setOpen(!open)} aria-label="Toggle navigation">{open ? <X/> : <Menu/>}</button>
    <aside className={`${open ? "translate-x-0" : "-translate-x-full"} fixed inset-y-0 z-40 w-[260px] overflow-y-auto border-r border-white/10 bg-ink p-5 transition lg:static lg:translate-x-0`}>
      <NavLink to="/dashboard" className="mb-9 flex items-center gap-3 text-xl font-black"><span className="grid h-10 w-10 place-items-center rounded-xl bg-edge text-ink">E</span> FPL Edge</NavLink>
      <nav className="space-y-1">{nav.map(([label, to, Icon]) => <NavLink key={to} to={to} onClick={() => setOpen(false)} className={({isActive}) => `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm ${isActive ? "bg-edge/10 text-edge" : "text-slate-400 hover:bg-white/5 hover:text-white"}`}><Icon size={18}/>{label}</NavLink>)}</nav>
      <button onClick={logout} className="mt-8 flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm text-slate-400 hover:bg-red-500/10 hover:text-red-300"><LogOut size={18}/>Log out</button>
    </aside>
    <main className="min-w-0 px-5 pb-12 pt-20 sm:px-8 lg:px-12 lg:pt-10"><Outlet/></main>
  </div>;
}

