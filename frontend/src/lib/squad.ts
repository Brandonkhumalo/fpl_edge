export type Position = "GKP" | "DEF" | "MID" | "FWD";

export type SquadPlayer = {
  playerId: string;
  clubId: string;
  clubName: string;
  clubFullName?: string;
  kitCode?: number;
  squadNumber?: number | null;
  selectedByPercent?: number | null;
  totalPoints?: number;
  name: string;
  position: Position;
  currentPrice: number;
  expectedPoints?: number;
};

export type Formation = "3-4-3" | "3-5-2" | "4-3-3" | "4-4-2" | "4-5-1" | "5-2-3" | "5-3-2" | "5-4-1";

export const formations: Formation[] = ["3-4-3", "3-5-2", "4-3-3", "4-4-2", "4-5-1", "5-2-3", "5-3-2", "5-4-1"];

export const squadRequirements: Record<Position, number> = { GKP: 2, DEF: 5, MID: 5, FWD: 3 };

export function formationCounts(formation: Formation): Record<Position, number> {
  const [defenders, midfielders, forwards] = formation.split("-").map(Number);
  return { GKP: 1, DEF: defenders, MID: midfielders, FWD: forwards };
}

export function buildLineup(players: SquadPlayer[], formation: Formation): string[] {
  const required = formationCounts(formation);
  return (["GKP", "DEF", "MID", "FWD"] as Position[]).flatMap((position) =>
    players.filter((player) => player.position === position).slice(0, required[position]).map((player) => player.playerId)
  );
}

export function orderBench(players: SquadPlayer[], starterIds: string[]): string[] {
  const substitutes = players.filter((player) => !starterIds.includes(player.playerId));
  return [
    ...substitutes.filter((player) => player.position !== "GKP"),
    ...substitutes.filter((player) => player.position === "GKP")
  ].map((player) => player.playerId);
}

export function inferFormation(players: SquadPlayer[], starterIds: string[]): Formation {
  const starters = players.filter((player) => starterIds.includes(player.playerId));
  const value = (["DEF", "MID", "FWD"] as Position[]).map((position) => starters.filter((player) => player.position === position).length).join("-") as Formation;
  return formations.includes(value) ? value : "4-4-2";
}

