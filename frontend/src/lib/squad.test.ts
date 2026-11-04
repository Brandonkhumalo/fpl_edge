import { describe, expect, it } from "vitest";
import { buildLineup, formations, inferFormation, orderBench, type Position, type SquadPlayer } from "./squad";

const positions: Position[] = ["GKP", "GKP", "DEF", "DEF", "DEF", "DEF", "DEF", "MID", "MID", "MID", "MID", "MID", "FWD", "FWD", "FWD"];
const players: SquadPlayer[] = positions.map((position, index) => ({
  playerId: `player-${index}`,
  clubId: `club-${index % 5}`,
  clubName: `C${index % 5}`,
  name: `Player ${index}`,
  position,
  currentPrice: 50
}));

describe("FPL formations", () => {
  it.each(formations)("builds a legal %s with eleven starters and four substitutes", (formation) => {
    const starters = buildLineup(players, formation);
    const bench = orderBench(players, starters);
    expect(starters).toHaveLength(11);
    expect(new Set(starters).size).toBe(11);
    expect(bench).toHaveLength(4);
    expect(inferFormation(players, starters)).toBe(formation);
    expect(players.find((player) => player.playerId === bench[3])?.position).toBe("GKP");
  });
});
