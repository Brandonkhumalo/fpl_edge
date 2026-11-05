export const chipNames: Record<string,string> = { bboost: "Bench Boost", "3xc": "Triple Captain", freehit: "Free Hit", wildcard: "Wildcard" };
export type Chip = {
  id: number; name: string; startGameweek: number; endGameweek: number;
  usedGameweek: number | null; used: boolean; usageSource: "" | "prior" | "fpl_edge";
};
export type Tactics = {
  revision: number; gameweek: number; deadline: string | null;
  activeChip: string; chips: Chip[]; scoringMultipliers: Record<string,number>;
  historyReviewed: boolean;
};
