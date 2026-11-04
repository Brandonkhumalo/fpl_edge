import { Construction } from "lucide-react";

export function PlaceholderPage({title,description}:{title:string;description:string}){return <div className="grid min-h-[60vh] place-items-center"><div className="card max-w-xl p-9 text-center"><Construction className="mx-auto text-edge" size={36}/><p className="mt-5 text-xs uppercase tracking-[.2em] text-edge">In active development</p><h1 className="mt-2 text-3xl font-black">{title}</h1><p className="mt-4 leading-7 text-slate-400">{description}</p><p className="mt-5 text-sm text-slate-500">This surface will only expose results once its backend workflow and validation are complete.</p></div></div>}

