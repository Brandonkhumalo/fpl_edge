package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"github.com/tishanyq/fpl-edge/backend/migrations"
	"os"
	"testing"
)

func TestManageSquadPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires isolated TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := New(pool)
	if err = s.SeedDevelopmentData(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(ctx, uuid.NewString()+"@example.test", "Test", "unused")
	if err != nil {
		t.Fatal(err)
	}
	season, err := s.CurrentSeason(ctx)
	if err != nil {
		t.Fatal(err)
	}
	catalogue, err := s.ListPlayers(ctx, season.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogue) == 0 || catalogue[0].TotalPoints <= 0 {
		t.Fatal("catalogue did not expose current FPL points")
	}
	var players []domain.SquadPlayer
	counts := map[domain.Position]int{}
	clubs := map[uuid.UUID]int{}
	for _, p := range catalogue {
		if counts[p.Position] >= domain.DefaultRules().PositionAllocation[p.Position] || clubs[p.ClubID] >= 3 {
			continue
		}
		players = append(players, p.SquadPlayer)
		counts[p.Position]++
		clubs[p.ClubID]++
	}
	starters, bench, captain, vice, err := domain.OptimizeLineup(players)
	if err != nil {
		t.Fatal(err)
	}
	squad, err := s.CreateSquad(ctx, user.ID, CreateSquadInput{SeasonID: season.ID, CurrentGameweek: 1, Players: players, Starters: starters, Bench: bench, CaptainID: captain, ViceCaptainID: vice, FreeTransfers: 1})
	if err != nil {
		t.Fatal(err)
	}
	var starterGK, benchGK uuid.UUID
	for _, p := range players {
		if p.Position == domain.Goalkeeper {
			for _, id := range starters {
				if id == p.PlayerID {
					starterGK = id
				}
			}
			for _, id := range bench {
				if id == p.PlayerID {
					benchGK = id
				}
			}
		}
	}
	if _, err = s.Substitute(ctx, uuid.New(), 1, starterGK, benchGK); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user change: %v", err)
	}
	var outfield uuid.UUID
	for _, p := range players {
		if p.Position != domain.Goalkeeper {
			for _, id := range starters {
				if id == p.PlayerID {
					outfield = id
				}
			}
		}
	}
	if _, err = s.Substitute(ctx, user.ID, 1, outfield, benchGK); err == nil {
		t.Fatal("accepted illegal keeper swap")
	}
	updated, err := s.Substitute(ctx, user.ID, squad.Revision, starterGK, benchGK)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Bank != squad.Bank || updated.FreeTransfers != 1 {
		t.Fatal("incorrect substitution finances/revision")
	}
	if err = domain.ValidateLineup(updated); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Substitute(ctx, user.ID, 1, benchGK, starterGK); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	saved, err := s.Squad(ctx, user.ID)
	if err != nil || saved.Revision != 2 {
		t.Fatal("substitution did not persist")
	}
	var replacement uuid.UUID
	for _, p := range catalogue {
		if p.Position != domain.Goalkeeper || p.CurrentPrice > playersPrice(players, starterGK) {
			continue
		}
		owned := false
		for _, o := range players {
			if o.PlayerID == p.PlayerID {
				owned = true
			}
		}
		if !owned && clubs[p.ClubID] < 3 {
			replacement = p.PlayerID
			break
		}
	}
	if replacement == uuid.Nil {
		t.Fatal("no transfer fixture")
	}
	updated, err = s.ConfirmTransfer(ctx, user.ID, ConfirmTransferInput{ExpectedRevision: 2, PlayerOutID: starterGK, PlayerInID: replacement, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 3 || len(updated.Players) != 15 || updated.FreeTransfers != 0 {
		t.Fatal("incorrect transfer result")
	}
	if err = domain.ValidateLineup(updated); err != nil {
		t.Fatal(err)
	}
	testTactics(t, s, ctx, user.ID, season.ID, updated, starterGK, replacement)
}

func testTactics(t *testing.T, s *Store, ctx context.Context, userID, seasonID uuid.UUID, team domain.Squad, oldGK, newGK uuid.UUID) {
	t.Helper()
	if err := s.SetCaptaincy(ctx, uuid.New(), team.Revision, team.CaptainID, team.ViceCaptainID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user captaincy: %v", err)
	}
	if err := s.SetCaptaincy(ctx, userID, team.Revision, team.CaptainID, team.CaptainID); !errors.Is(err, ErrTactics) {
		t.Fatal("accepted identical captain and VC")
	}
	if err := s.SetCaptaincy(ctx, userID, team.Revision, team.Bench[0], team.ViceCaptainID); !errors.Is(err, ErrTactics) {
		t.Fatal("accepted bench captain")
	}
	if err := s.SetCaptaincy(ctx, userID, team.Revision, team.ViceCaptainID, team.CaptainID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCaptaincy(ctx, userID, team.Revision, team.CaptainID, team.ViceCaptainID); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("accepted stale revision")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO gameweeks(season_id,number,deadline_at) VALUES($1,2,now()+interval '1 day'),($1,3,now()+interval '8 days'),($1,4,now()+interval '15 days') ON CONFLICT(season_id,number) DO UPDATE SET deadline_at=EXCLUDED.deadline_at`, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO season_chips(season_id,provider_id,name,start_gameweek,end_gameweek) VALUES($1,101,'freehit',2,19),($1,102,'wildcard',2,19),($1,103,'bboost',2,19),($1,104,'3xc',2,19),($1,105,'freehit',2,19) ON CONFLICT DO NOTHING`, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareSquad(ctx, userID); err != nil {
		t.Fatal(err)
	}
	before, err := s.Squad(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetChip(ctx, uuid.New(), before.Revision, 101, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user chip: %v", err)
	}
	if err = s.SetChip(ctx, userID, before.Revision, 101, false); err != nil {
		t.Fatal(err)
	}
	active, err := s.Tactics(ctx, userID)
	if err != nil || active.ActiveChip != "freehit" {
		t.Fatalf("missing active chip: %v", err)
	}
	if err = s.SetChip(ctx, userID, active.Revision, 104, false); !errors.Is(err, ErrTactics) {
		t.Fatal("accepted two chips")
	}
	if err = s.SetChip(ctx, userID, active.Revision, 101, true); !errors.Is(err, ErrTactics) {
		t.Fatal("cancelled free hit")
	}
	changed, err := s.ConfirmTransfer(ctx, userID, ConfirmTransferInput{ExpectedRevision: active.Revision, PlayerOutID: newGK, PlayerInID: oldGK, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if changed.FreeTransfers != before.FreeTransfers {
		t.Fatal("free hit consumed free transfer")
	}
	var hit int
	if err = s.pool.QueryRow(ctx, `SELECT points_hit FROM user_transfer_ledger WHERE squad_id=$1 ORDER BY squad_revision DESC LIMIT 1`, before.ID).Scan(&hit); err != nil || hit != 0 {
		t.Fatal("free hit charged points")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE gameweeks SET deadline_at=now()-interval '1 minute' WHERE season_id=$1 AND number=2`, seasonID); err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareSquad(ctx, userID); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Squad(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Bank != before.Bank || restored.CaptainID != before.CaptainID || restored.ViceCaptainID != before.ViceCaptainID || restored.CurrentGameweek != 3 {
		t.Fatal("free hit failed to restore squad state")
	}
	for _, p := range before.Players {
		found := false
		for _, q := range restored.Players {
			if p.PlayerID == q.PlayerID && p.PurchasePrice == q.PurchasePrice {
				found = true
			}
		}
		if !found {
			t.Fatal("free hit failed to restore owned player")
		}
	}
	if err = domain.ValidateLineup(restored); err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareSquad(ctx, userID); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Squad(ctx, userID)
	if again.Revision != restored.Revision {
		t.Fatal("rollover was not idempotent")
	}
	if err = s.SetChip(ctx, userID, restored.Revision, 101, false); !errors.Is(err, ErrTactics) {
		t.Fatal("reused chip")
	}
	if err = s.SetChip(ctx, userID, restored.Revision, 105, false); !errors.Is(err, ErrTactics) {
		t.Fatal("consecutive free hits accepted")
	}
	if err = s.SetChip(ctx, userID, restored.Revision, 102, false); err != nil {
		t.Fatal(err)
	}
	active, _ = s.Tactics(ctx, userID)
	changed, err = s.ConfirmTransfer(ctx, userID, ConfirmTransferInput{ExpectedRevision: active.Revision, PlayerOutID: newGK, PlayerInID: oldGK, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if changed.FreeTransfers != restored.FreeTransfers {
		t.Fatal("wildcard consumed transfer")
	}
	if _, err = s.pool.Exec(ctx, `UPDATE gameweeks SET deadline_at=now()-interval '1 minute' WHERE season_id=$1 AND number=3`, seasonID); err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareSquad(ctx, userID); err != nil {
		t.Fatal(err)
	}
	permanent, _ := s.Squad(ctx, userID)
	found := false
	for _, p := range permanent.Players {
		if p.PlayerID == oldGK {
			found = true
		}
	}
	if !found {
		t.Fatal("wildcard did not preserve transfer")
	}
	if err = s.SetChip(ctx, userID, permanent.Revision, 103, false); err != nil {
		t.Fatal(err)
	}
	active, _ = s.Tactics(ctx, userID)
	if active.ActiveChip != "bboost" {
		t.Fatal("bench boost not active")
	}
	for _, id := range permanent.Bench {
		if active.ScoringMultipliers[id] != 1 {
			t.Fatal("bench boost omitted bench player")
		}
	}
	if err = s.SetChip(ctx, userID, active.Revision, 103, true); err != nil {
		t.Fatal(err)
	}
	active, _ = s.Tactics(ctx, userID)
	if err = s.SetChip(ctx, userID, active.Revision, 104, false); err != nil {
		t.Fatal(err)
	}
	active, _ = s.Tactics(ctx, userID)
	if active.ActiveChip != "3xc" {
		t.Fatal("triple captain not active")
	}
	if active.ScoringMultipliers[permanent.CaptainID] != 3 {
		t.Fatal("triple captain multiplier missing")
	}
	for _, id := range permanent.Bench {
		if active.ScoringMultipliers[id] != 0 {
			t.Fatal("bench remained boosted after cancellation")
		}
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			results <- s.SetCaptaincy(ctx, userID, active.Revision, permanent.ViceCaptainID, permanent.CaptainID)
		}()
	}
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("concurrent captaincy edits were not serialized")
	}
}

func playersPrice(players []domain.SquadPlayer, id uuid.UUID) int {
	for _, p := range players {
		if p.PlayerID == id {
			return p.CurrentPrice
		}
	}
	return 0
}
