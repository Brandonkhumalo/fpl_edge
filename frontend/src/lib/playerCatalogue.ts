import type { Position, SquadPlayer } from "./squad";

export type PlayerSort = "default" | "price-asc" | "price-desc" | "most-selected";

export function filterPlayers(players: SquadPlayer[], search: string, position: Position | "ALL", clubId: string, sort: PlayerSort): SquadPlayer[] {
  const query = search.trim().toLowerCase();
  const result = players.filter((player) =>
    (position === "ALL" || player.position === position) &&
    (!clubId || player.clubId === clubId) &&
    `${player.name} ${player.clubName} ${player.clubFullName ?? ""}`.toLowerCase().includes(query)
  );
  if (sort === "default") return result;
  return result.sort((a, b) => {
    const difference = sort === "price-asc" ? a.currentPrice - b.currentPrice
      : sort === "price-desc" ? b.currentPrice - a.currentPrice
      : (b.selectedByPercent ?? -1) - (a.selectedByPercent ?? -1);
    return difference || a.name.localeCompare(b.name) || a.playerId.localeCompare(b.playerId);
  });
}
