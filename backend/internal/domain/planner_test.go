package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestPlanTransfersCarriesBankAndFreeTransfers(t *testing.T) {
	clubA, clubB := uuid.New(), uuid.New()
	out := SquadPlayer{PlayerID: uuid.New(), ClubID: clubA, Position: Forward, CurrentPrice: 60, PurchasePrice: 60}
	in := SquadPlayer{PlayerID: uuid.New(), ClubID: clubB, Position: Forward, CurrentPrice: 65, PurchasePrice: 65}
	players := validPlanningSquad(out)
	forecastPlayers := make([]PlayerProjection, 0, len(players)+1)
	for _, player := range players {
		forecastPlayers = append(forecastPlayers, PlayerProjection{SquadPlayer: player, Status: "a", Coverage: 5, Gameweeks: map[int]float64{1: 2, 2: 2, 3: 2}})
	}
	forecastPlayers = append(forecastPlayers, PlayerProjection{SquadPlayer: in, Status: "a", Coverage: 5, Gameweeks: map[int]float64{1: 8, 2: 8, 3: 8}})
	plan, err := PlanTransfers(Squad{Players: players, Bank: 10, FreeTransfers: 1}, Forecast{Gameweek: 1, HorizonEnd: 3, Players: forecastPlayers}, 3, "balanced")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Weeks) != 3 || plan.Weeks[0].Transfer == nil || plan.Weeks[0].BankAfter != 5 || plan.ExpectedGain <= 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func validPlanningSquad(replacement SquadPlayer) []SquadPlayer {
	positions := []Position{Goalkeeper, Goalkeeper, Defender, Defender, Defender, Defender, Defender, Midfielder, Midfielder, Midfielder, Midfielder, Midfielder, Forward, Forward, Forward}
	players := make([]SquadPlayer, len(positions))
	for i, position := range positions {
		players[i] = SquadPlayer{PlayerID: uuid.New(), ClubID: uuid.New(), Position: position, CurrentPrice: 50, PurchasePrice: 50}
	}
	players[len(players)-1] = replacement
	return players
}
