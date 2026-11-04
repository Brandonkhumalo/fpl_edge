import { ArrowRight, BarChart3, BrainCircuit, Check, ChevronRight, CircleDollarSign, LineChart, Menu, ShieldCheck, Sparkles, Target, Trophy, Users, X, Zap } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";

const features = [
  { icon: Target, title: "Build your real squad", text: "Select all 15 players within the official position, club and £100m budget constraints." },
  { icon: Trophy, title: "Choose your best XI", text: "See your eleven starters on the pitch, four-player bench and every legal FPL formation." },
  { icon: BrainCircuit, title: "Make informed decisions", text: "Use explainable captaincy, lineup and transfer recommendations grounded in your saved team." },
  { icon: LineChart, title: "Plan beyond one week", text: "Compare fixture runs and transfer ideas across your preferred planning horizon." },
  { icon: CircleDollarSign, title: "Respect every £0.1m", text: "Track purchase price, selling value and money in the bank without unsafe floating-point calculations." },
  { icon: ShieldCheck, title: "Private by design", text: "Your squad is protected by your account. FPL Edge never asks for your official FPL password." }
];

const steps = [
  ["01", "Create your account", "Sign up securely with your email and password."],
  ["02", "Pick your 15 players", "Choose two goalkeepers, five defenders, five midfielders and three forwards."],
  ["03", "Set your formation", "Select any legal FPL formation and review your starting XI and bench."],
  ["04", "Use your edge", "Open your dashboard for squad-aware insights and recommendations."]
];

export function LandingPage() {
  const [mobileOpen, setMobileOpen] = useState(false);
  return <div className="min-h-screen overflow-hidden bg-ink">
    <header className="fixed inset-x-0 top-0 z-50 border-b border-white/[.07] bg-ink/85 backdrop-blur-xl">
      <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-5 sm:px-6">
        <a href="#top" className="flex items-center gap-3 text-xl font-black"><Logo />FPL Edge</a>
        <nav className="hidden items-center gap-8 text-sm font-medium text-slate-300 md:flex"><a href="#features" className="transition hover:text-white">Features</a><a href="#how-it-works" className="transition hover:text-white">How it works</a><a href="#squad-rules" className="transition hover:text-white">Squad rules</a></nav>
        <div className="hidden items-center gap-3 sm:flex"><Link to="/login" className="rounded-xl px-4 py-2.5 text-sm font-semibold text-slate-200 transition hover:bg-white/5">Log in</Link><Link to="/register" className="button-primary py-2.5 text-sm">Create account</Link></div>
        <button className="rounded-xl border border-white/10 p-2.5 sm:hidden" onClick={() => setMobileOpen(!mobileOpen)} aria-label="Toggle menu">{mobileOpen ? <X /> : <Menu />}</button>
      </div>
      {mobileOpen && <div className="border-t border-white/10 bg-ink px-5 py-5 sm:hidden"><nav className="space-y-2">{[["Features", "#features"], ["How it works", "#how-it-works"], ["Squad rules", "#squad-rules"]].map(([label, href]) => <a key={href} href={href} onClick={() => setMobileOpen(false)} className="block rounded-xl px-3 py-3 text-sm text-slate-300 hover:bg-white/5">{label}</a>)}</nav><div className="mt-4 grid grid-cols-2 gap-3"><Link to="/login" className="rounded-xl border border-white/10 px-4 py-3 text-center text-sm font-semibold">Log in</Link><Link to="/register" className="button-primary py-3 text-sm">Sign up</Link></div></div>}
    </header>

    <main id="top">
      <section className="relative mx-auto grid min-h-[820px] max-w-7xl items-center gap-16 px-5 pb-24 pt-36 sm:px-6 lg:grid-cols-[1.08fr_.92fr] lg:pt-40">
        <div className="pointer-events-none absolute -right-40 top-12 h-[520px] w-[520px] rounded-full bg-edge/[.07] blur-3xl" />
        <div className="relative">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-edge/20 bg-edge/[.06] px-4 py-2 text-sm font-semibold text-edge"><Sparkles size={16} /> Built for thoughtful FPL managers</div>
          <h1 className="max-w-3xl text-5xl font-black leading-[1.02] tracking-[-.045em] text-white sm:text-6xl lg:text-7xl">Your FPL team.<br /><span className="bg-gradient-to-r from-edge to-emerald-300 bg-clip-text text-transparent">Sharper decisions.</span></h1>
          <p className="mt-7 max-w-2xl text-lg leading-8 text-slate-400 sm:text-xl">Build your 15-player squad, choose a legal formation, and turn fixtures, form and availability into clear lineup, captaincy and transfer decisions.</p>
          <div className="mt-9 flex flex-col gap-3 sm:flex-row"><Link to="/register" className="button-primary gap-2 px-6 py-3.5">Build my squad <ArrowRight size={18} /></Link><a href="#how-it-works" className="inline-flex items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/[.03] px-6 py-3.5 font-semibold text-white transition hover:bg-white/[.07]">See how it works <ChevronRight size={18} /></a></div>
          <div className="mt-8 flex flex-wrap gap-x-6 gap-y-3 text-sm text-slate-400">{["Free to get started", "No FPL password required", "Your squad stays private"].map((item) => <span key={item} className="flex items-center gap-2"><span className="grid h-5 w-5 place-items-center rounded-full bg-edge/10 text-edge"><Check size={12} strokeWidth={3} /></span>{item}</span>)}</div>
        </div>
        <HeroPreview />
      </section>

      <section className="border-y border-white/[.07] bg-white/[.02]"><div className="mx-auto grid max-w-7xl grid-cols-2 gap-px px-5 py-7 sm:grid-cols-4 sm:px-6">{[["15", "Players in your squad"], ["8", "Legal FPL formations"], ["£100m", "Budget-aware selection"], ["1", "Squad-aware dashboard"]].map(([value, label]) => <div key={label} className="px-3 py-4 text-center"><p className="text-2xl font-black text-white sm:text-3xl">{value}</p><p className="mt-1 text-xs text-slate-500 sm:text-sm">{label}</p></div>)}</div></section>

      <section id="features" className="scroll-mt-24 px-5 py-24 sm:px-6 lg:py-32"><div className="mx-auto max-w-7xl"><SectionHeading eyebrow="Everything in one place" title="Manage your team with confidence" text="FPL Edge keeps the experience simple while respecting the rules and financial details that make every decision matter." /><div className="mt-14 grid gap-4 md:grid-cols-2 lg:grid-cols-3">{features.map(({ icon: Icon, title, text }) => <article key={title} className="group rounded-3xl border border-white/[.08] bg-gradient-to-b from-white/[.045] to-transparent p-6 transition hover:-translate-y-1 hover:border-edge/20"><span className="grid h-12 w-12 place-items-center rounded-2xl bg-edge/10 text-edge transition group-hover:bg-edge group-hover:text-ink"><Icon size={22} /></span><h3 className="mt-6 text-xl font-black">{title}</h3><p className="mt-3 text-sm leading-6 text-slate-400">{text}</p></article>)}</div></div></section>

      <section id="how-it-works" className="scroll-mt-24 border-y border-white/[.07] bg-[#091626] px-5 py-24 sm:px-6 lg:py-32"><div className="mx-auto max-w-7xl"><SectionHeading eyebrow="A clear path to your dashboard" title="From sign-up to starting XI in minutes" text="No official FPL credentials, browser extensions or complicated imports are needed to start." /><div className="relative mt-14 grid gap-4 lg:grid-cols-4">{steps.map(([number, title, text], index) => <article key={number} className="relative rounded-3xl border border-white/[.08] bg-ink/60 p-6"><div className="flex items-center justify-between"><span className="text-3xl font-black text-edge/35">{number}</span>{index < steps.length - 1 && <ArrowRight className="hidden text-white/15 lg:block" />}</div><h3 className="mt-8 text-lg font-black">{title}</h3><p className="mt-2 text-sm leading-6 text-slate-400">{text}</p></article>)}</div></div></section>

      <section id="squad-rules" className="scroll-mt-24 px-5 py-24 sm:px-6 lg:py-32"><div className="mx-auto grid max-w-7xl items-center gap-14 lg:grid-cols-2"><div><p className="text-xs font-bold uppercase tracking-[.24em] text-edge">Rules built in</p><h2 className="mt-4 text-4xl font-black tracking-tight sm:text-5xl">A legal squad before it ever reaches your dashboard.</h2><p className="mt-6 max-w-xl text-base leading-8 text-slate-400">The builder guides every pick and the API validates the finished team again. You always know what is missing and why a selection cannot be made.</p><div className="mt-8 grid gap-3 sm:grid-cols-2">{["2 goalkeepers", "5 defenders", "5 midfielders", "3 forwards", "Maximum 3 per club", "£100.0m starting budget"].map((rule) => <div key={rule} className="flex items-center gap-3 rounded-xl border border-white/[.07] bg-white/[.025] px-4 py-3 text-sm"><span className="grid h-6 w-6 place-items-center rounded-full bg-edge/10 text-edge"><Check size={13} strokeWidth={3} /></span>{rule}</div>)}</div></div><FormationCard /></div></section>

      <section className="px-5 pb-24 sm:px-6 lg:pb-32"><div className="mx-auto max-w-7xl overflow-hidden rounded-[2rem] border border-edge/20 bg-gradient-to-br from-emerald-400/15 via-panel to-panel p-8 sm:p-12 lg:flex lg:items-center lg:justify-between"><div><p className="text-sm font-bold text-edge">Ready to build your edge?</p><h2 className="mt-3 max-w-2xl text-3xl font-black tracking-tight sm:text-4xl">Create your account and put your team on the pitch.</h2><p className="mt-4 text-slate-400">Your saved squad will be waiting whenever you log back in.</p></div><div className="mt-8 flex shrink-0 flex-col gap-3 sm:flex-row lg:mt-0"><Link to="/login" className="rounded-xl border border-white/10 px-5 py-3 text-center font-semibold transition hover:bg-white/5">Log in</Link><Link to="/register" className="button-primary gap-2 px-6">Create free account <ArrowRight size={18} /></Link></div></div></section>
    </main>

    <footer className="border-t border-white/[.07] px-5 py-8 sm:px-6"><div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-5 text-center sm:flex-row sm:text-left"><div className="flex items-center gap-3 font-black"><Logo small />FPL Edge</div><p className="text-xs text-slate-500">Independent FPL decision support. We never claim to execute official FPL changes.</p><div className="flex gap-5 text-xs text-slate-400"><Link to="/privacy">Privacy</Link><Link to="/terms">Terms</Link></div></div></footer>
  </div>;
}

function Logo({ small = false }: { small?: boolean }) {
  return <span className={`grid place-items-center rounded-xl bg-gradient-to-br from-edge to-emerald-400 font-black text-ink shadow-[0_0_24px_rgba(39,226,164,.2)] ${small ? "h-8 w-8 text-sm" : "h-10 w-10"}`}>E</span>;
}

function SectionHeading({ eyebrow, title, text }: { eyebrow: string; title: string; text: string }) {
  return <div className="mx-auto max-w-3xl text-center"><p className="text-xs font-bold uppercase tracking-[.24em] text-edge">{eyebrow}</p><h2 className="mt-4 text-4xl font-black tracking-tight sm:text-5xl">{title}</h2><p className="mx-auto mt-5 max-w-2xl text-base leading-7 text-slate-400">{text}</p></div>;
}

function HeroPreview() {
  const rows = [["FWD", 2], ["MID", 4], ["DEF", 4], ["GK", 1]] as const;
  return <div className="relative mx-auto w-full max-w-[540px]"><div className="absolute -inset-10 rounded-full bg-edge/[.06] blur-3xl" /><div className="relative rounded-[2rem] border border-white/10 bg-panel/95 p-4 shadow-2xl sm:p-5"><div className="flex items-center justify-between border-b border-white/[.07] pb-4"><div><p className="text-[10px] font-bold uppercase tracking-[.2em] text-edge">Squad preview</p><p className="mt-1 font-black">My Starting XI</p></div><span className="rounded-full bg-edge/10 px-3 py-1 text-xs font-bold text-edge">4-4-2</span></div><div className="pitch relative mt-4 overflow-hidden rounded-3xl border border-white/10 px-3 py-5"><div className="absolute inset-x-[8%] top-1/2 border-t border-white/15" /><div className="absolute left-1/2 top-1/2 h-16 w-16 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/15" /><div className="relative space-y-4">{rows.map(([label, count]) => <div key={label} className="flex justify-center gap-2">{Array.from({ length: count }).map((_, index) => <div key={index} className="w-[58px] rounded-lg bg-ink/90 p-1.5 text-center shadow-lg sm:w-[72px]"><div className="mx-auto grid h-6 w-6 place-items-center rounded-md bg-edge text-[8px] font-black text-ink">{label}</div><div className="mx-auto mt-1 h-1.5 w-10 rounded-full bg-white/15" /></div>)}</div>)}</div></div><div className="mt-3 grid grid-cols-4 gap-2">{[1, 2, 3, 4].map((number) => <div key={number} className="rounded-lg border border-white/[.07] bg-white/[.03] px-2 py-2 text-center text-[9px] text-slate-500">Bench {number}</div>)}</div></div><div className="absolute -bottom-5 -left-5 hidden rounded-2xl border border-white/10 bg-ink/95 p-4 shadow-xl sm:block"><div className="flex items-center gap-3"><span className="grid h-9 w-9 place-items-center rounded-xl bg-edge/10 text-edge"><BarChart3 size={18} /></span><div><p className="text-[10px] text-slate-500">Squad status</p><p className="text-sm font-black">15 / 15 ready</p></div></div></div></div>;
}

function FormationCard() {
  return <div className="rounded-[2rem] border border-white/10 bg-panel p-5 shadow-2xl sm:p-7"><div className="flex items-center justify-between"><div><p className="text-xs font-bold uppercase tracking-[.2em] text-edge">Formation selector</p><h3 className="mt-1 text-xl font-black">Every legal FPL shape</h3></div><Zap className="text-edge" /></div><div className="mt-6 grid grid-cols-4 gap-2">{["3-4-3", "3-5-2", "4-3-3", "4-4-2", "4-5-1", "5-2-3", "5-3-2", "5-4-1"].map((formation, index) => <div key={formation} className={`rounded-xl border px-2 py-3 text-center text-xs font-black ${index === 3 ? "border-edge bg-edge text-ink" : "border-white/[.07] bg-white/[.025] text-slate-300"}`}>{formation}</div>)}</div><div className="mt-6 rounded-2xl border border-edge/15 bg-edge/[.04] p-4"><div className="flex gap-3"><Users className="shrink-0 text-edge" size={20} /><p className="text-sm leading-6 text-slate-300">Switch formation without changing your 15-player squad. The correct players move between the starting XI and bench.</p></div></div></div>;
}
