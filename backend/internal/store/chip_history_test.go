package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"github.com/tishanyq/fpl-edge/backend/migrations"
)

func TestPriorChipHistory(t *testing.T) {
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
	seasonID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO seasons(id,name,starts_on,ends_on,rules) VALUES($1,$2,'2026-08-01','2027-06-01','{}')`, seasonID, "History "+seasonID.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO gameweeks(season_id,number,deadline_at) SELECT $1,n,now()+(n-30)*interval '1 day'+interval '12 hours' FROM generate_series(1,38) n`, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO season_chips(season_id,provider_id,name,start_gameweek,end_gameweek) VALUES
	($1,1,'wildcard',2,19),($1,2,'wildcard',20,38),($1,3,'freehit',2,19),($1,6,'freehit',20,38),
	($1,4,'bboost',1,19),($1,7,'bboost',20,38),($1,5,'3xc',1,19),($1,8,'3xc',20,38)`, seasonID)
	if err != nil {
		t.Fatal(err)
	}
	var players []domain.SquadPlayer
	var clubIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		id := uuid.New()
		clubIDs = append(clubIDs, id)
		if _, err = pool.Exec(ctx, `INSERT INTO clubs(id,season_id,provider_id,name,short_name) VALUES($1,$2,$3,$4,$4)`, id, seasonID, i, fmt.Sprintf("T%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for _, pos := range []domain.Position{domain.Goalkeeper, domain.Defender, domain.Midfielder, domain.Forward} {
		for i := 0; i < domain.DefaultRules().PositionAllocation[pos]; i++ {
			index := len(players)
			p := domain.SquadPlayer{PlayerID: uuid.New(), ClubID: clubIDs[index%5], Position: pos, PurchasePrice: 50, CurrentPrice: 50, SellingPrice: 50}
			_, err = pool.Exec(ctx, `INSERT INTO players(id,season_id,club_id,provider_id,first_name,last_name,position,price) VALUES($1,$2,$3,$4,'History','Player',$5,50)`, p.PlayerID, seasonID, p.ClubID, index, pos)
			if err != nil {
				t.Fatal(err)
			}
			players = append(players, p)
		}
	}
	var starters, bench []uuid.UUID
	remaining := map[domain.Position]int{domain.Goalkeeper: 1, domain.Defender: 4, domain.Midfielder: 4, domain.Forward: 2}
	for _, p := range players {
		if remaining[p.Position] > 0 {
			starters = append(starters, p.PlayerID)
			remaining[p.Position]--
		} else {
			bench = append(bench, p.PlayerID)
		}
	}
	captain, vice := starters[1], starters[2]
	newUser := func() uuid.UUID {
		t.Helper()
		user, err := s.CreateUser(ctx, uuid.NewString()+"@example.test", "History", "unused")
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.CreateSquad(ctx, user.ID, CreateSquadInput{SeasonID: seasonID, CurrentGameweek: 30, Players: players, Starters: starters, Bench: bench, CaptainID: captain, ViceCaptainID: vice, Bank: 250, FreeTransfers: 2})
		if err != nil {
			t.Fatal(err)
		}
		return user.ID
	}
	userID, otherID := newUser(), newUser()
	week := func(n int) *int { return &n }
	prior := []PriorChipUsage{{ChipID: 4, Gameweek: week(6)}, {ChipID: 7, Gameweek: week(25)}, {ChipID: 3}}
	before, err := s.Squad(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetChipHistory(ctx, userID, before.Revision, prior); err != nil {
		t.Fatal(err)
	}
	tactics, err := s.Tactics(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !tactics.HistoryReviewed || tactics.ActiveChip != "" {
		t.Fatal("history did not stay separate from activation")
	}
	used := 0
	for _, chip := range tactics.Chips {
		if chip.Used {
			used++
			if chip.UsageSource != "prior" {
				t.Fatal("wrong provenance")
			}
		}
	}
	if used != 3 {
		t.Fatal("did not record both Bench Boost sets and one Free Hit")
	}
	after, err := s.Squad(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	after.Revision = before.Revision
	if !reflect.DeepEqual(before, after) {
		t.Fatal("history changed squad, captaincy or finances")
	}
	other, err := s.Tactics(ctx, otherID)
	if err != nil {
		t.Fatal(err)
	}
	if other.HistoryReviewed || other.Revision != 1 {
		t.Fatal("history leaked across users")
	}
	if err = s.SetChipHistory(ctx, uuid.New(), 1, prior); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing user could change history")
	}
	if err = s.SetChipHistory(ctx, userID, 1, prior); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("stale edit accepted")
	}
	if err = s.SetChip(ctx, userID, tactics.Revision, 7, false); !errors.Is(err, ErrTactics) {
		t.Fatal("consumed chip could be activated")
	}
	for name, invalid := range map[string][]PriorChipUsage{
		"unknown chip":          {{ChipID: 999}},
		"duplicate chip":        {{ChipID: 4}, {ChipID: 4}},
		"wrong half":            {{ChipID: 4, Gameweek: week(25)}},
		"future week":           {{ChipID: 7, Gameweek: week(31)}},
		"planning week":         {{ChipID: 7, Gameweek: week(30)}},
		"same gameweek":         {{ChipID: 4, Gameweek: week(6)}, {ChipID: 5, Gameweek: week(6)}},
		"consecutive Free Hits": {{ChipID: 3, Gameweek: week(19)}, {ChipID: 6, Gameweek: week(20)}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := s.SetChipHistory(ctx, userID, tactics.Revision, invalid); !errors.Is(err, ErrTactics) {
				t.Fatalf("accepted invalid history: %v", err)
			}
			state, _ := s.Tactics(ctx, userID)
			if state.Revision != tactics.Revision {
				t.Fatal("failed validation mutated history")
			}
		})
	}
	// Correcting an imported record leaves the other half independent.
	if err = s.SetChipHistory(ctx, userID, tactics.Revision, []PriorChipUsage{{ChipID: 4, Gameweek: week(6)}}); err != nil {
		t.Fatal(err)
	}
	tactics, _ = s.Tactics(ctx, userID)
	if err = s.SetChip(ctx, userID, tactics.Revision, 7, false); err != nil {
		t.Fatal(err)
	}
	tactics, _ = s.Tactics(ctx, userID)
	if err = s.SetChipHistory(ctx, userID, tactics.Revision, []PriorChipUsage{{ChipID: 7, Gameweek: week(25)}}); !errors.Is(err, ErrTactics) {
		t.Fatal("in-app activation could be overwritten")
	}
	if err = s.SetChipHistory(ctx, userID, tactics.Revision, []PriorChipUsage{}); err != nil {
		t.Fatal(err)
	}
	tactics, _ = s.Tactics(ctx, userID)
	if tactics.ActiveChip != "bboost" {
		t.Fatal("clearing imported history cancelled an in-app chip")
	}
	// Across the reset, unknown Free Hit dates block only the potentially
	// consecutive opportunity until an exact date is supplied.
	boundaryUser := newUser()
	if _, err = pool.Exec(ctx, `UPDATE user_squads SET current_gameweek=20 WHERE user_id=$1`, boundaryUser); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE gameweeks SET deadline_at=now()+interval '1 day' WHERE season_id=$1 AND number=20`, seasonID); err != nil {
		t.Fatal(err)
	}
	if err = s.SetChipHistory(ctx, boundaryUser, 1, []PriorChipUsage{{ChipID: 7}}); !errors.Is(err, ErrTactics) {
		t.Fatal("future half marked used")
	}
	if err = s.SetChipHistory(ctx, boundaryUser, 1, []PriorChipUsage{{ChipID: 3}}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetChip(ctx, boundaryUser, 2, 6, false); !errors.Is(err, ErrTactics) {
		t.Fatal("unknown prior Free Hit bypassed consecutive guard")
	}
	if err = s.SetChipHistory(ctx, boundaryUser, 2, []PriorChipUsage{{ChipID: 3, Gameweek: week(18)}}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetChip(ctx, boundaryUser, 3, 6, false); err != nil {
		t.Fatal(err)
	}
}
