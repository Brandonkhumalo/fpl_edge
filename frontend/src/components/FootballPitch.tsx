import { Shirt } from "lucide-react";
import { formationCounts, type Formation, type Position, type SquadPlayer } from "../lib/squad";

type Props = {
  players: SquadPlayer[];
  formation: Formation;
  captainId?: string;
  viceCaptainId?: string;
  compact?: boolean;
};

const positions: Position[] = ["FWD", "MID", "DEF", "GKP"];

export function FootballPitch({ players, formation, captainId, viceCaptainId, compact = false }: Props) {
  const required = formationCounts(formation);
  return <div className="pitch relative overflow-hidden rounded-[1.75rem] border border-white/10 p-4 sm:p-6">
    <div className="pointer-events-none absolute inset-x-[8%] top-1/2 border-t border-white/15" />
    <div className="pointer-events-none absolute left-1/2 top-1/2 h-20 w-20 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/15" />
    <div className="pointer-events-none absolute inset-x-[28%] bottom-0 h-[16%] border-x border-t border-white/15" />
    <div className={`relative flex flex-col justify-between ${compact ? "min-h-[330px] gap-3" : "min-h-[500px] gap-5"}`}>
      {positions.map((position) => {
        const positionPlayers = players.filter((player) => player.position === position).slice(0, required[position]);
        return <div key={position} className="flex min-h-[68px] items-center justify-center gap-1.5 sm:gap-3">
          {Array.from({ length: required[position] }).map((_, index) => {
            const player = positionPlayers[index];
            return player
              ? <PitchPlayer key={player.playerId} player={player} badge={player.playerId === captainId ? "C" : player.playerId === viceCaptainId ? "V" : undefined} compact={compact} />
              : <EmptySlot key={`${position}-${index}`} position={position} compact={compact} />;
          })}
        </div>;
      })}
    </div>
  </div>;
}

export function SquadPlayerCard({ player, badge, compact = false }: { player: SquadPlayer; badge?: string; compact?: boolean }) {
  return <div className={`relative min-w-0 rounded-xl border border-white/10 bg-[#07111f]/95 text-center shadow-lg ${compact ? "px-2 py-2" : "px-3 py-3"}`}>
    <div className={`mx-auto grid place-items-center rounded-lg bg-gradient-to-br from-edge to-emerald-600 text-ink ${compact ? "h-7 w-7" : "h-9 w-9"}`}><Shirt size={compact ? 14 : 17} strokeWidth={2.5} /></div>
    <p className={`mt-1.5 truncate font-bold ${compact ? "max-w-[72px] text-[10px] sm:max-w-[92px] sm:text-xs" : "text-xs sm:text-sm"}`}>{player.name}</p>
    <p className="mt-0.5 truncate text-[10px] font-medium text-slate-400">{player.clubName || player.position}</p>
    {badge && <span className="absolute -right-1.5 -top-1.5 grid h-6 w-6 place-items-center rounded-full bg-amber-300 text-[10px] font-black text-ink shadow-lg">{badge}</span>}
  </div>;
}

function PitchPlayer({ player, badge, compact }: { player: SquadPlayer; badge?: string; compact: boolean }) {
  return <SquadPlayerCard player={player} badge={badge} compact={compact} />;
}

function EmptySlot({ position, compact }: { position: Position; compact: boolean }) {
  return <div className={`grid place-items-center rounded-xl border border-dashed border-white/20 bg-black/10 text-center text-white/35 ${compact ? "h-[60px] w-[62px] text-[9px] sm:w-[78px]" : "h-[76px] w-[78px] text-[10px] sm:w-[98px]"}`}>
    <div><Shirt className="mx-auto mb-1" size={compact ? 16 : 20} /><span>{position}</span></div>
  </div>;
}

