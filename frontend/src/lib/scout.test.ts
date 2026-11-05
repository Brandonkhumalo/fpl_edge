import { describe, expect, it } from "vitest";
import { rankScoutPlayers, type ScoutPlayer } from "./scout";

const players: ScoutPlayer[] = [
  { playerId: "a", clubId: "ars", clubName: "ARS", name: "Popular", position: "MID", currentPrice: 100, selectedByPercent: 42, totalPoints: 90, nextFiveTotal: 24 },
  { playerId: "b", clubId: "bri", clubName: "BHA", name: "Differential", position: "MID", currentPrice: 65, selectedByPercent: 4.5, totalPoints: 70, nextFiveTotal: 27 },
  { playerId: "c", clubId: "eve", clubName: "EVE", name: "Budget", position: "DEF", currentPrice: 45, selectedByPercent: 8, totalPoints: 55, nextFiveTotal: 18 },
  { playerId: "d", clubId: "liv", clubName: "LIV", name: "Unknown ownership", position: "FWD", currentPrice: 75, selectedByPercent: null, totalPoints: 75 },
];

const base = { search: "", position: "ALL" as const, clubId: "", view: "all" as const, sort: "points-desc" as const };

describe("scout rankings", () => {
  it("shows productive low-owned differentials and excludes unknown ownership", () => {
    expect(rankScoutPlayers(players, { ...base, view: "differentials" }).map(player => player.playerId)).toEqual(["b", "c"]);
  });

  it("supports high-to-low and low-to-high price ordering", () => {
    expect(rankScoutPlayers(players, { ...base, sort: "price-desc" }).map(player => player.playerId)).toEqual(["a", "d", "b", "c"]);
    expect(rankScoutPlayers(players, { ...base, sort: "price-asc" }).map(player => player.playerId)).toEqual(["c", "b", "d", "a"]);
  });

  it("combines position filtering with next-five forecast ranking", () => {
    expect(rankScoutPlayers(players, { ...base, position: "MID", sort: "forecast-desc" }).map(player => player.playerId)).toEqual(["b", "a"]);
  });
});
