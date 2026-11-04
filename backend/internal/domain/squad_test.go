package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestSellingPrice(t *testing.T) {
	tests := []struct{ purchase, current, want int }{
		{70, 66, 66}, {70, 70, 70}, {70, 71, 70}, {70, 72, 71}, {70, 75, 72},
	}
	for _, test := range tests {
		if got := SellingPrice(test.purchase, test.current); got != test.want {
			t.Fatalf("SellingPrice(%d, %d) = %d, want %d", test.purchase, test.current, got, test.want)
		}
	}
}

func TestTransferHit(t *testing.T) {
	if got := TransferHit(2, 4, DefaultRules()); got != 8 {
		t.Fatalf("got %d, want 8", got)
	}
	if got := TransferHit(2, 1, DefaultRules()); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}

func TestValidateSquadRejectsFourthPlayerFromClub(t *testing.T) {
	players := validPlayers()
	players[3].ClubID = players[0].ClubID
	err := ValidateSquad(players, 0, DefaultRules())
	if err == nil {
		t.Fatal("expected club-limit error")
	}
}

func TestOptimizeLineupProducesLegalLineup(t *testing.T) {
	players := validPlayers()
	for i := range players {
		players[i].ExpectedPoints = float64(i + 1)
		players[i].ExpectedMins = 90
	}
	starters, bench, captain, vice, err := OptimizeLineup(players)
	if err != nil {
		t.Fatal(err)
	}
	squad := Squad{Players: players, Starters: starters, Bench: bench, CaptainID: captain, ViceCaptainID: vice}
	if err := ValidateLineup(squad); err != nil {
		t.Fatalf("optimized lineup is illegal: %v", err)
	}
}

func validPlayers() []SquadPlayer {
	positions := []Position{Goalkeeper, Goalkeeper, Defender, Defender, Defender, Defender, Defender, Midfielder, Midfielder, Midfielder, Midfielder, Midfielder, Forward, Forward, Forward}
	players := make([]SquadPlayer, len(positions))
	clubs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for i, position := range positions {
		players[i] = SquadPlayer{PlayerID: uuid.New(), ClubID: clubs[i%len(clubs)], Position: position, PurchasePrice: 60, CurrentPrice: 60, SellingPrice: 60}
	}
	return players
}
