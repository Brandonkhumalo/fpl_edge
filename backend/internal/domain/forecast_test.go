package domain

import (
	"github.com/google/uuid"
	"math"
	"testing"
	"time"
)

func testForecastData() (FootballData, map[int]SquadPlayer, time.Time) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	minutes := 900
	data := FootballData{Clubs: []FootballClub{{ProviderID: 1, ShortName: "AAA"}, {ProviderID: 2, ShortName: "BBB"}}, Players: []FootballPlayer{{ProviderID: 1, ClubID: 1, Position: Defender, Price: 50, Status: "a", FootballStats: FootballStats{Minutes: &minutes, Starts: 10, XG: "1.0", XA: "2.0", DefensiveActions: 100, Saves: 20, Bonus: 6}}}}
	for w := 1; w <= 9; w++ {
		data.Gameweeks = append(data.Gameweeks, FootballGameweek{Number: w})
	}
	// Two fixtures in GW2, a blank GW3, then three fixtures GW4..6.
	for i, w := range []int{2, 2, 4, 5, 6} {
		week := w
		kick := now.Add(time.Duration(i+1) * 24 * time.Hour)
		data.Fixtures = append(data.Fixtures, FootballFixture{ID: i + 1, Home: 1, Away: 2, Gameweek: &week, Kickoff: &kick, HomeDifficulty: 2, AwayDifficulty: 4})
	}
	ids := map[int]SquadPlayer{1: {PlayerID: uuid.New(), ClubID: uuid.New(), Position: Defender}}
	return data, ids, now
}

func TestForecastFiveFixturesAndBlankDoubleWeeks(t *testing.T) {
	data, ids, now := testForecastData()
	f, err := ForecastPlayers(data, ids, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	p := f.Players[0]
	if p.Coverage != 5 || len(p.Fixtures) != 5 || p.Gameweeks[3] != 0 || p.Gameweeks[2] <= 0 {
		t.Fatalf("wrong fixture coverage: %+v", p)
	}
	if math.Abs(p.Gameweeks[2]-p.Fixtures[0].Points-p.Fixtures[1].Points) > 1e-8 {
		t.Fatal("double gameweek not summed")
	}
	if math.Abs(p.NextFiveTotal-p.HorizonTotal) > 1e-8 {
		t.Fatal("wrong horizon total")
	}
	for _, fixture := range p.Fixtures {
		sum := 0.0
		for _, n := range fixture.Breakdown {
			sum += n
		}
		if math.Abs(sum-fixture.Points) > 1e-8 {
			t.Fatal("breakdown disagrees with total")
		}
	}
	data.Fixtures[4].Gameweek = nil
	f, err = ForecastPlayers(data, ids, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Players[0].Coverage != 4 || len(f.Warnings) < 3 {
		t.Fatal("unassigned fixture fabricated or not disclosed")
	}
}

func TestForecastUnavailableAndLowMinutes(t *testing.T) {
	data, ids, now := testForecastData()
	zero := 0
	data.Players[0].Chance = &zero
	f, err := ForecastPlayers(data, ids, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Players[0].HorizonTotal != 0 {
		t.Fatal("unavailable player scored points")
	}
	data.Players[0].Chance = nil
	data.Players[0].Minutes = &zero
	data.Players[0].Starts = 0
	f, err = ForecastPlayers(data, ids, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Players[0].HorizonTotal != 0 || f.Players[0].Confidence != "low" {
		t.Fatal("zero evidence overstated")
	}
	data.Players[0].Minutes = nil
	if _, err = ForecastPlayers(data, ids, 2, now); err == nil {
		t.Fatal("missing minutes accepted")
	}
}

func TestScoringPositionRulesAndThresholds(t *testing.T) {
	coefficients := DefaultProjectionCoefficients()
	for position, goals := range map[Position]float64{Goalkeeper: 10, Defender: 6, Midfielder: 5, Forward: 4} {
		if coefficients.GoalPoints[position] != goals {
			t.Fatal("incorrect goal points")
		}
	}
	if poissonGroups(3, 3) >= 1 || poissonGroups(3, 3) <= 0 {
		t.Fatal("save groups must use expectation of floor, not floor of expectation")
	}
	if poissonTail(10, 10) <= poissonTail(10, 12) {
		t.Fatal("DEF/MID defensive thresholds not distinguished")
	}
	data, _, _ := testForecastData()
	p := data.Players[0]
	fixture := data.Fixtures[0]
	def := projectFixture(p, fixture, 10, 14, map[int]string{})
	p.Position = Midfielder
	mid := projectFixture(p, fixture, 10, 14, map[int]string{})
	p.Position = Forward
	fwd := projectFixture(p, fixture, 10, 14, map[int]string{})
	p.Position = Goalkeeper
	p.PenaltySaves = 1
	p.PenaltyMisses = 1
	p.Yellow = 1
	p.Red = 1
	p.OwnGoals = 1
	gk := projectFixture(p, fixture, 10, 14, map[int]string{})
	if def.Breakdown["cleanSheet"] != 4*mid.Breakdown["cleanSheet"] || fwd.Breakdown["cleanSheet"] != 0 {
		t.Fatal("incorrect clean sheet points")
	}
	if def.Breakdown["defence"] <= mid.Breakdown["defence"] || gk.Breakdown["defence"] != 0 {
		t.Fatal("incorrect defensive contributions")
	}
	if gk.Breakdown["saves"] <= 0 || gk.Breakdown["penaltySaves"] <= 0 || gk.Breakdown["goalsConceded"] >= 0 || gk.Breakdown["penaltyMisses"] >= 0 || gk.Breakdown["yellowCards"] >= 0 || gk.Breakdown["redCards"] >= 0 || gk.Breakdown["ownGoals"] >= 0 {
		t.Fatal("missing scoring component")
	}
	fixture.HomeDifficulty = 5
	hard := projectFixture(data.Players[0], fixture, 10, 14, map[int]string{})
	if hard.Points >= def.Points {
		t.Fatal("fixture difficulty has no effect")
	}
}

func TestZeroProjectionOptimizerIsLegal(t *testing.T) {
	players := validPlayers()
	s, b, c, v, err := OptimizeLineup(players)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateLineup(Squad{Players: players, Starters: s, Bench: b, CaptainID: c, ViceCaptainID: v}); err != nil {
		t.Fatal(err)
	}
}

func TestTransferAdviceBudgetClubHitAndHorizon(t *testing.T) {
	players := validPlayers()
	forecast := Forecast{Gameweek: 2, HorizonEnd: 6}
	for _, p := range players {
		forecast.Players = append(forecast.Players, PlayerProjection{SquadPlayer: p, Status: "a", Coverage: 5, HorizonTotal: 10, NextFiveTotal: 10, Gameweeks: map[int]float64{2: 2, 3: 2, 4: 2, 5: 2, 6: 2}})
	}
	candidate := PlayerProjection{SquadPlayer: SquadPlayer{PlayerID: uuid.New(), ClubID: uuid.New(), Position: Defender, CurrentPrice: 61}, Status: "a", Coverage: 5, HorizonTotal: 30, NextFiveTotal: 30, Gameweeks: map[int]float64{2: 2, 3: 7, 4: 7, 5: 7, 6: 7}}
	forecast.Players = append(forecast.Players, candidate)
	squad := Squad{Players: players, Bank: 1, FreeTransfers: 1}
	free, err := RankTransfers(squad, forecast, "")
	if err != nil || len(free) == 0 {
		t.Fatalf("missing five-game improvement: %v", err)
	}
	squad.FreeTransfers = 0
	paid, err := RankTransfers(squad, forecast, "")
	if err != nil || len(paid) == 0 {
		t.Fatal(err)
	}
	if math.Abs(free[0].NetGain-paid[0].NetGain-4) > 1e-8 || paid[0].Hit != 4 {
		t.Fatal("transfer hit ignored")
	}
	squad.Bank = 0
	none, _ := RankTransfers(squad, forecast, "")
	if len(none) != 0 {
		t.Fatal("over-budget transfer")
	}
	squad.Bank = 1
	forecast.Players[len(forecast.Players)-1].Coverage = 4
	none, _ = RankTransfers(squad, forecast, "")
	if len(none) != 0 {
		t.Fatal("incomplete five-fixture coverage recommended")
	}
	forecast.Players[len(forecast.Players)-1].Coverage = 5
	forecast.Players[len(forecast.Players)-1].ClubID = players[0].ClubID
	moves, _ := RankTransfers(squad, forecast, "")
	for _, move := range moves {
		for _, p := range players {
			if p.PlayerID == move.OutID && p.ClubID != players[0].ClubID {
				t.Fatal("fourth club player recommended")
			}
		}
	}
}

func TestRebuildUsesLegalMovesAndScoresTheRequestedWindow(t *testing.T) {
	players := validPlayers()
	f := Forecast{Gameweek: 2, HorizonEnd: 6}
	for _, p := range players {
		f.Players = append(f.Players, PlayerProjection{SquadPlayer: p, Status: "a", Coverage: 5, HorizonTotal: 10, NextFiveTotal: 10, Gameweeks: map[int]float64{2: 2, 3: 2, 4: 2, 5: 2, 6: 2}})
	}
	for _, pos := range []Position{Defender, Midfielder, Forward} {
		p := SquadPlayer{PlayerID: uuid.New(), ClubID: uuid.New(), Position: pos, CurrentPrice: 61}
		f.Players = append(f.Players, PlayerProjection{SquadPlayer: p, Status: "a", Coverage: 5, HorizonTotal: 25, Gameweeks: map[int]float64{2: 1, 3: 6, 4: 6, 5: 6, 6: 6}})
	}
	squad := Squad{Players: players, Bank: 3, FreeTransfers: 1}
	fh, err := PlanRebuild(squad, f, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fh.Moves) != 0 {
		t.Fatal("Free Hit recommended for future-week gains")
	}
	wc, err := PlanRebuild(squad, f, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(wc.Moves) != 3 || wc.Gain <= 0 {
		t.Fatal("Wildcard missed persistent upgrades")
	}
	fhTransfers, err := RankTransfers(squad, f, "freehit")
	if err != nil || len(fhTransfers) != 0 {
		t.Fatal("active Free Hit incorrectly valued persistent transfers")
	}
	for _, move := range wc.Moves {
		if move.BankAfter < 0 {
			t.Fatal("rebuild overspent")
		}
	}
	if wc.Moves[2].BankAfter != 0 {
		t.Fatal("rebuild bank not carried across moves")
	}
}
