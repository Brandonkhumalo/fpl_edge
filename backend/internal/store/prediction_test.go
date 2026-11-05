package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

func TestSavedLineupPointsUsesSelectionCaptainAndChipMultipliers(t *testing.T) {
	captainID := uuid.New()
	starterID := uuid.New()
	benchID := uuid.New()
	players := []domain.SquadPlayer{{PlayerID: captainID}, {PlayerID: starterID}, {PlayerID: benchID}}
	forecast := domain.Forecast{Players: []domain.PlayerProjection{
		{SquadPlayer: players[0], Gameweeks: map[int]float64{10: 6}},
		{SquadPlayer: players[1], Gameweeks: map[int]float64{10: 4}},
		{SquadPlayer: players[2], Gameweeks: map[int]float64{10: 3}},
	}}

	points, err := savedLineupPoints(players, forecast, 10, map[uuid.UUID]int{captainID: 2, starterID: 1, benchID: 0})
	if err != nil || points != 16 {
		t.Fatalf("standard captain scoring = %.1f, %v; want 16", points, err)
	}
	points, err = savedLineupPoints(players, forecast, 10, map[uuid.UUID]int{captainID: 3, starterID: 1, benchID: 0})
	if err != nil || points != 22 {
		t.Fatalf("triple captain scoring = %.1f, %v; want 22", points, err)
	}
	points, err = savedLineupPoints(players, forecast, 10, map[uuid.UUID]int{captainID: 2, starterID: 1, benchID: 1})
	if err != nil || points != 19 {
		t.Fatalf("bench boost scoring = %.1f, %v; want 19", points, err)
	}
}
