import { describe, expect, it } from "vitest";
import { filterPlayers } from "./playerCatalogue";
import type { SquadPlayer } from "./squad";

const players: SquadPlayer[] = [
  { playerId: "a", clubId: "ars", clubName: "ARS", clubFullName: "Arsenal", name: "Alpha", position: "DEF", currentPrice: 60, selectedByPercent: 9.5 },
  { playerId: "b", clubId: "che", clubName: "CHE", clubFullName: "Chelsea", name: "Beta", position: "MID", currentPrice: 45, selectedByPercent: 40 },
  { playerId: "c", clubId: "ars", clubName: "ARS", clubFullName: "Arsenal", name: "Gamma", position: "MID", currentPrice: 50, selectedByPercent: null },
];

describe("player catalogue", () => {
  it("sorts numeric prices without changing the source catalogue", () => {
    expect(filterPlayers(players, "", "ALL", "", "price-asc").map(p => p.playerId)).toEqual(["b", "c", "a"]);
    expect(filterPlayers(players, "", "ALL", "", "price-desc").map(p => p.playerId)).toEqual(["a", "c", "b"]);
    expect(players.map(p => p.playerId)).toEqual(["a", "b", "c"]);
  });
  it("ranks numeric ownership and places unavailable values last", () => {
    expect(filterPlayers(players, "", "ALL", "", "most-selected").map(p => p.playerId)).toEqual(["b", "a", "c"]);
  });
  it("combines full club name search, club selection and position", () => {
    expect(filterPlayers(players, " ARSENAL ", "MID", "ars", "price-asc").map(p => p.playerId)).toEqual(["c"]);
    expect(filterPlayers(players, "Arsenal", "ALL", "che", "default")).toEqual([]);
  });
});
