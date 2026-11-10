package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

func TestLiveMultipliersApplyAutosubsAndCaptainFallback(t *testing.T) {
	players := []domain.SquadPlayer{}
	add := func(position domain.Position) uuid.UUID {
		id := uuid.New()
		players = append(players, domain.SquadPlayer{PlayerID: id, Position: position})
		return id
	}
	gk := add(domain.Goalkeeper)
	benchGK := add(domain.Goalkeeper)
	defenders := []uuid.UUID{add(domain.Defender), add(domain.Defender), add(domain.Defender), add(domain.Defender), add(domain.Defender)}
	mids := []uuid.UUID{add(domain.Midfielder), add(domain.Midfielder), add(domain.Midfielder), add(domain.Midfielder), add(domain.Midfielder)}
	forwards := []uuid.UUID{add(domain.Forward), add(domain.Forward), add(domain.Forward)}
	starters := []uuid.UUID{gk, defenders[0], defenders[1], defenders[2], defenders[3], mids[0], mids[1], mids[2], mids[3], forwards[0], forwards[1]}
	bench := []uuid.UUID{mids[4], defenders[4], forwards[2], benchGK}
	squad := domain.Squad{Players: players, Starters: starters, Bench: bench, CaptainID: mids[0], ViceCaptainID: forwards[0]}
	stats := map[uuid.UUID]domain.LivePlayer{}
	for _, p := range players {
		stats[p.PlayerID] = domain.LivePlayer{Minutes: 90}
	}
	stats[mids[0]] = domain.LivePlayer{}
	stats[defenders[0]] = domain.LivePlayer{}
	multipliers, subs := liveMultipliers(squad, stats, map[uuid.UUID]bool{mids[0]: true, defenders[0]: true}, "")
	if multipliers[mids[0]] != 0 || multipliers[forwards[0]] != 2 || multipliers[mids[4]] != 1 || multipliers[defenders[0]] != 0 || multipliers[defenders[4]] != 1 || len(subs) != 2 {
		t.Fatalf("unexpected live selection: multipliers=%v substitutions=%v", multipliers, subs)
	}
}

func TestLiveMultipliersBenchBoostDoesNotAutosub(t *testing.T) {
	starter, bench, captain := uuid.New(), uuid.New(), uuid.New()
	squad := domain.Squad{Players: []domain.SquadPlayer{{PlayerID: starter}, {PlayerID: bench}, {PlayerID: captain}}, Starters: []uuid.UUID{starter, captain}, Bench: []uuid.UUID{bench}, CaptainID: captain}
	stats := map[uuid.UUID]domain.LivePlayer{starter: {}, bench: {Minutes: 90}, captain: {Minutes: 90}}
	multipliers, subs := liveMultipliers(squad, stats, map[uuid.UUID]bool{starter: true}, "bboost")
	if multipliers[bench] != 1 || multipliers[captain] != 2 || len(subs) != 0 {
		t.Fatalf("unexpected bench boost selection: %v %v", multipliers, subs)
	}
}
