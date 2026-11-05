import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../lib/api";
import { chipNames, type Tactics } from "../lib/chips";

export function ChipHistoryForm({ data, onSaved }: { data: Tactics; onSaved: () => void }) {
  const cache = useQueryClient();
  const [draft, setDraft] = useState<Record<number, string>>(() =>
    Object.fromEntries(data.chips.filter(c => c.usageSource === "prior").map(c => [c.id, c.usedGameweek === null ? "" : String(c.usedGameweek)])));
  const save = useMutation({
    mutationFn: () => api<Tactics>("/squad/chip-history", {
      method: "PUT",
      body: JSON.stringify({
        expectedRevision: data.revision,
        usages: Object.entries(draft).map(([id, gameweek]) => ({ chipId: Number(id), gameweek: gameweek === "" ? null : Number(gameweek) }))
      })
    }),
    onSuccess: result => {
      cache.setQueryData(["tactics"], result);
      void cache.invalidateQueries({ queryKey: ["squad"] });
      void cache.invalidateQueries({ queryKey: ["squad-history"] });
      onSaved();
    }
  });
  const names = [...new Set(data.chips.map(c => c.name))];
  return <section className="card my-6 p-5" id="chip-history">
    <h2 className="text-xl font-bold">Played FPL before joining?</h2>
    <p className="mt-2 text-sm leading-6 text-slate-400">Record chips already used on your official FPL team. Tick each set separately—using a first-half chip does not use the second-half copy. You can correct these entries later.</p>
    <p className="mt-2 text-xs text-slate-400">Recording history will not activate chips, change players, or restore an old Free Hit team. If none were used, leave every box unchecked and save.</p>
    <form className="mt-5 space-y-4" onSubmit={event => { event.preventDefault(); save.mutate(); }}>
      {names.map(name => {
        const copies = data.chips.filter(c => c.name === name);
        const count = copies.filter(c => c.usageSource === "fpl_edge" || Object.hasOwn(draft, c.id)).length;
        return <fieldset className="rounded-xl border border-white/10 p-4" key={name} disabled={save.isPending}>
          <legend className="px-2 font-bold">{chipNames[name]} <span className="text-sm font-normal text-slate-400">· {count} of {copies.length} used</span></legend>
          <div className="grid gap-4 sm:grid-cols-2">{copies.map((chip, index) => {
            const local = chip.usageSource === "fpl_edge";
            const checked = local || Object.hasOwn(draft, chip.id);
            const future = chip.startGameweek >= data.gameweek;
            const lastPastWeek = Math.min(chip.endGameweek, data.gameweek - 1);
            return <div key={chip.id}>
              <label className="flex items-start gap-3 text-sm">
                <input type="checkbox" className="mt-1 accent-emerald-400" checked={checked} disabled={local || future}
                  onChange={event => setDraft(current => {
                    const next = { ...current };
                    if (event.target.checked) next[chip.id] = ""; else delete next[chip.id];
                    return next;
                  })} />
                <span>Set {index + 1} · GW {chip.startGameweek}–{chip.endGameweek}
                  <span className="mt-1 block text-xs text-slate-400">{local ? "Recorded in FPL Edge; cannot edit as prior history" : future ? "Future set—cannot already be used" : "Already used before joining"}</span>
                </span>
              </label>
              {checked && !local && <label className="mt-3 block text-xs text-slate-400">Gameweek used (optional)
                <select className="field mt-1" value={draft[chip.id]} aria-label={`${chipNames[name]} set ${index + 1} gameweek used`}
                  onChange={event => setDraft(current => ({ ...current, [chip.id]: event.target.value }))}>
                  <option value="">I don't remember</option>
                  {Array.from({ length: Math.max(0, lastPastWeek - chip.startGameweek + 1) }, (_, i) => chip.startGameweek + i).map(gw =>
                    <option key={gw} value={gw}>Gameweek {gw}</option>)}
                </select>
              </label>}
            </div>;
          })}</div>
        </fieldset>;
      })}
      <p className="text-xs leading-5 text-slate-400">Free Hit cannot be used in consecutive gameweeks. If the exact week is unknown near a reset, enter it before activating your next Free Hit.</p>
      {save.error && <p role="alert" className="text-sm text-red-300">{save.error.message} <button type="button" className="underline" onClick={() => void cache.invalidateQueries({ queryKey: ["tactics"] })}>Reload chip history</button></p>}
      <button className="button-primary" disabled={save.isPending}>{save.isPending ? "Saving…" : "Save prior chip usage"}</button>
    </form>
  </section>;
}
