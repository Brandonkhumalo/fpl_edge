import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { useAuth } from "./auth";
import type { SquadPlayer } from "./squad";

export type Prediction = SquadPlayer & {
  status: string; confidence: string; sampleMinutes: number; coverage: number;
  nextFiveTotal: number; horizonTotal: number; gameweeks: Record<string, number>;
  fixtures: { fixtureId: number; gameweek: number; opponent: string; home: boolean; kickoff: string;
    difficulty: number; expectedMinutes: number; points: number; breakdown: Record<string, number> }[];
};
export type Advice = {
  squadRevision: number; gameweek: number; horizonEnd: number; transferHorizonEnd: number; modelVersion: string;
  generatedAt: string; dataUpdatedAt: string; warnings: string[]; players: Prediction[]; owned: string[];
  savedLineupPoints: number;
  lineup: { starters: string[]; bench: string[]; captainId: string; viceCaptainId: string; points: number; benchPoints: number; captainPoints: number };
  transfers: { outId: string; inId: string; netGain: number; nextWeekGain: number; hit: number; bankAfter: number }[];
  chips: { id: number; name: string; decision: string; reason: string; bestGameweek: number; extraPoints: number;
    rebuild?: { moves: { outId: string; inId: string; bankAfter: number }[]; gain: number; nextWeekGain: number } }[];
};

export function useAdvice(revision?: number) {
  const userId = useAuth((state) => state.user?.id);
  return useQuery({ queryKey: ["advice", userId, revision], queryFn: () => api<Advice>("/advice"),
    enabled: revision !== undefined && !!userId, retry: false, staleTime: 60_000, refetchInterval: 300_000 });
}

export const pointLabels: Record<string, string> = {
  appearance: "Minutes played", goals: "Goals", assists: "Assists", cleanSheet: "Clean sheet",
  defence: "Defensive contributions", bonus: "Bonus", saves: "Save points", penaltySaves: "Penalty saves",
  goalsConceded: "Goals conceded", yellowCards: "Yellow cards", redCards: "Red cards", ownGoals: "Own goals", penaltyMisses: "Penalty misses",
};
