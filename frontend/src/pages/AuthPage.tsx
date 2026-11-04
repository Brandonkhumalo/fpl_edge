import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowLeft, Check, Eye, EyeOff, LockKeyhole, Mail, ShieldCheck, Sparkles, UserRound } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useNavigate } from "react-router-dom";
import { z } from "zod";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

const schema = z.object({ displayName: z.string().optional(), email: z.email("Enter a valid email address"), password: z.string().min(12, "Use at least 12 characters") });
type Values = z.infer<typeof schema>;
type Session = { user: { id: string; email: string; displayName: string }; accessToken: string };

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const navigate = useNavigate();
  const setSession = useAuth((state) => state.setSession);
  const [serverError, setServerError] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<Values>({ resolver: zodResolver(schema) });

  const submit = async (values: Values) => {
    if (mode === "register" && (!values.displayName || values.displayName.trim().length < 2)) {
      setError("displayName", { message: "Enter at least 2 characters" });
      return;
    }
    setServerError("");
    try {
      const session = await api<Session>(`/auth/${mode}`, { method: "POST", body: JSON.stringify(values) });
      setSession(session);
      navigate("/dashboard", { replace: true });
    } catch (error) {
      setServerError(error instanceof Error ? error.message : "Unable to continue");
    }
  };

  return <main className="grid min-h-screen bg-ink lg:grid-cols-[1.05fr_.95fr]">
    <section className="relative hidden overflow-hidden border-r border-white/[.07] lg:flex lg:flex-col lg:justify-between lg:p-12 xl:p-16">
      <div className="absolute -left-40 top-1/4 h-[500px] w-[500px] rounded-full bg-edge/[.08] blur-3xl" />
      <Link to="/" className="relative flex items-center gap-3 text-xl font-black"><span className="grid h-10 w-10 place-items-center rounded-xl bg-edge text-ink">E</span> FPL Edge</Link>
      <div className="relative max-w-xl"><div className="mb-6 inline-flex items-center gap-2 rounded-full border border-edge/20 bg-edge/5 px-4 py-2 text-sm font-semibold text-edge"><Sparkles size={16} /> Your next Gameweek starts here</div><h1 className="text-5xl font-black leading-[1.06] tracking-tight">Build the team.<br /><span className="text-edge">Find your edge.</span></h1><p className="mt-6 text-lg leading-8 text-slate-400">Your squad, formation, bench and decisions stay together in one focused FPL workspace.</p><div className="mt-9 space-y-4">{["A persistent 15-player squad", "All eight legal FPL formations", "Squad-aware advice and planning"].map((item) => <div key={item} className="flex items-center gap-3 text-sm text-slate-300"><span className="grid h-7 w-7 place-items-center rounded-full bg-edge/10 text-edge"><Check size={15} strokeWidth={3} /></span>{item}</div>)}</div></div>
      <p className="relative text-xs text-slate-600">Independent FPL decision support. No official FPL password required.</p>
    </section>

    <section className="grid place-items-center px-5 py-10 sm:px-8">
      <div className="w-full max-w-md">
        <div className="mb-9 flex items-center justify-between lg:hidden"><Link to="/" className="flex items-center gap-3 font-black"><span className="grid h-9 w-9 place-items-center rounded-xl bg-edge text-ink">E</span> FPL Edge</Link><Link to="/" className="text-sm text-slate-400">Home</Link></div>
        <Link to="/" className="mb-8 hidden items-center gap-2 text-sm text-slate-400 transition hover:text-white lg:inline-flex"><ArrowLeft size={16} /> Back to home</Link>
        <div className="mb-8"><p className="text-xs font-bold uppercase tracking-[.22em] text-edge">{mode === "login" ? "Welcome back" : "Get started"}</p><h1 className="mt-3 text-3xl font-black tracking-tight sm:text-4xl">{mode === "login" ? "Log in to FPL Edge" : "Create your account"}</h1><p className="mt-3 text-sm leading-6 text-slate-400">{mode === "login" ? "Continue to your saved squad and Gameweek dashboard." : "Create your secure account, then build your 15-player squad."}</p></div>

        <form onSubmit={handleSubmit(submit)} className="space-y-5">
          {mode === "register" && <Field label="Display name" error={errors.displayName?.message as string | undefined}><UserRound className="absolute left-3.5 top-3.5 text-slate-500" size={18} /><input className="field pl-11" autoComplete="name" placeholder="How should we address you?" {...register("displayName")} /></Field>}
          <Field label="Email address" error={errors.email?.message as string | undefined}><Mail className="absolute left-3.5 top-3.5 text-slate-500" size={18} /><input className="field pl-11" type="email" autoComplete="email" placeholder="you@example.com" {...register("email")} /></Field>
          <Field label="Password" error={errors.password?.message as string | undefined}><LockKeyhole className="absolute left-3.5 top-3.5 text-slate-500" size={18} /><input className="field px-11" type={showPassword ? "text" : "password"} autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="At least 12 characters" {...register("password")} /><button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute right-3 top-3 rounded-lg p-1 text-slate-500 transition hover:text-white" aria-label={showPassword ? "Hide password" : "Show password"}>{showPassword ? <EyeOff size={18} /> : <Eye size={18} />}</button></Field>
          {serverError && <p role="alert" className="rounded-xl border border-red-400/15 bg-red-500/10 p-3.5 text-sm text-red-200">{serverError}</p>}
          <button className="button-primary w-full gap-2 py-3.5" disabled={isSubmitting}>{isSubmitting ? "Please wait…" : mode === "login" ? "Log in securely" : "Create my account"}</button>
        </form>

        <div className="mt-6 flex items-center justify-center gap-2 text-xs text-slate-500"><ShieldCheck size={14} /> Passwords are protected with Argon2id hashing.</div>
        <p className="mt-8 text-center text-sm text-slate-400">{mode === "login" ? "New to FPL Edge? " : "Already have an account? "}<Link className="font-bold text-edge hover:text-emerald-300" to={mode === "login" ? "/register" : "/login"}>{mode === "login" ? "Create an account" : "Log in"}</Link></p>
      </div>
    </section>
  </main>;
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return <label className="block text-sm font-semibold"><span>{label}</span><span className="relative mt-2 block">{children}</span>{error && <span className="mt-1.5 block text-xs font-normal text-red-300">{error}</span>}</label>;
}
