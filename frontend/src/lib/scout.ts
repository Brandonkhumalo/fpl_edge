import type { Position, SquadPlayer } from "./squad";

export type ScoutView = "top" | "differentials" | "all";
export type ScoutSort = "points-desc" | "forecast-desc" | "price-desc" | "price-asc" | "ownership-asc" | "ownership-desc";
export type ScoutPlayer = SquadPlayer & { nextFiveTotal?: number };

type ScoutFilters = {
  search: string;
  position: Position | "ALL";
  clubId: string;
  view: ScoutView;
  sort: ScoutSort;
};

export const differentialOwnershipLimit = 10;

export function rankScoutPlayers(players: ScoutPlayer[], filters: ScoutFilters): ScoutPlayer[] {
  const query = filters.search.trim().toLowerCase();
  const result = players.filter((player) => {
    const matchesDifferential = filters.view !== "differentials" ||
      (player.selectedByPercent != null && player.selectedByPercent <= differentialOwnershipLimit && (player.totalPoints ?? 0) > 0);
    const matchesTopTalent = filters.view !== "top" || (player.totalPoints ?? 0) > 0;
    return matchesDifferential && matchesTopTalent &&
      (filters.position === "ALL" || player.position === filters.position) &&
      (!filters.clubId || player.clubId === filters.clubId) &&
      `${player.name} ${player.clubName} ${player.clubFullName ?? ""}`.toLowerCase().includes(query);
  });

  return result.sort((first, second) => {
    let difference = 0;
    switch (filters.sort) {
      case "price-desc": difference = second.currentPrice - first.currentPrice; break;
      case "price-asc": difference = first.currentPrice - second.currentPrice; break;
      case "forecast-desc": difference = (second.nextFiveTotal ?? -1) - (first.nextFiveTotal ?? -1); break;
      case "ownership-asc": difference = (first.selectedByPercent ?? Number.POSITIVE_INFINITY) - (second.selectedByPercent ?? Number.POSITIVE_INFINITY); break;
      case "ownership-desc": difference = (second.selectedByPercent ?? -1) - (first.selectedByPercent ?? -1); break;
      default: difference = (second.totalPoints ?? 0) - (first.totalPoints ?? 0);
    }
    // Differential ties favor the less-owned player before applying stable names.
    if (!difference && filters.view === "differentials") {
      difference = (first.selectedByPercent ?? Number.POSITIVE_INFINITY) - (second.selectedByPercent ?? Number.POSITIVE_INFINITY);
    }
    return difference || first.name.localeCompare(second.name) || first.playerId.localeCompare(second.playerId);
  });
}
