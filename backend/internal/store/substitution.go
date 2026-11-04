package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

var ErrInvalidSubstitution = errors.New("invalid substitution")

func (s *Store) Substitute(ctx context.Context, userID uuid.UUID, revision int64, starterID, benchID uuid.UUID) (domain.Squad, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Squad{}, err
	}
	defer tx.Rollback(ctx)
	var squad domain.Squad
	err = tx.QueryRow(ctx, `SELECT id,revision,captain_id,vice_captain_id FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&squad.ID, &squad.Revision, &squad.CaptainID, &squad.ViceCaptainID)
	if errors.Is(err, pgx.ErrNoRows) {
		return squad, ErrNotFound
	}
	if err != nil {
		return squad, err
	}
	if squad.Revision != revision {
		return squad, ErrRevisionConflict
	}
	rows, err := tx.Query(ctx, `SELECT p.id,p.position,sp.is_starter,sp.bench_order FROM squad_players sp JOIN players p ON p.id=sp.player_id WHERE sp.squad_id=$1 ORDER BY sp.bench_order NULLS FIRST`, squad.ID)
	if err != nil {
		return squad, err
	}
	benchOrder := 0
	foundStarter, foundBench := false, false
	for rows.Next() {
		var p domain.SquadPlayer
		var starter bool
		var order *int
		if err = rows.Scan(&p.PlayerID, &p.Position, &starter, &order); err != nil {
			rows.Close()
			return squad, err
		}
		squad.Players = append(squad.Players, p)
		if starter {
			if p.PlayerID == starterID {
				foundStarter = true
				squad.Starters = append(squad.Starters, benchID)
			} else {
				squad.Starters = append(squad.Starters, p.PlayerID)
			}
		} else {
			if p.PlayerID == benchID {
				foundBench = true
				benchOrder = *order
				squad.Bench = append(squad.Bench, starterID)
			} else {
				squad.Bench = append(squad.Bench, p.PlayerID)
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return squad, err
	}
	if !foundStarter || !foundBench {
		return squad, fmt.Errorf("%w: select an owned starter and bench player", ErrInvalidSubstitution)
	}
	if squad.CaptainID == starterID {
		squad.CaptainID = benchID
	}
	if squad.ViceCaptainID == starterID {
		squad.ViceCaptainID = benchID
	}
	if err = domain.ValidateLineup(squad); err != nil {
		return squad, fmt.Errorf("%w: %s", ErrInvalidSubstitution, err)
	}
	// Promote first to free the unique bench slot before moving the starter.
	if _, err = tx.Exec(ctx, `UPDATE squad_players SET is_starter=true,bench_order=NULL WHERE squad_id=$1 AND player_id=$2`, squad.ID, benchID); err != nil {
		return squad, err
	}
	if _, err = tx.Exec(ctx, `UPDATE squad_players SET is_starter=false,bench_order=$3 WHERE squad_id=$1 AND player_id=$2`, squad.ID, starterID, benchOrder); err != nil {
		return squad, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET revision=revision+1,updated_at=now(),captain_id=$2,vice_captain_id=$3 WHERE id=$1`, squad.ID, squad.CaptainID, squad.ViceCaptainID); err != nil {
		return squad, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'lineup.substituted',jsonb_build_object('starterId',$4::text,'benchId',$5::text))`, squad.ID, userID, revision+1, starterID.String(), benchID.String()); err != nil {
		return squad, err
	}
	if err = tx.Commit(ctx); err != nil {
		return squad, err
	}
	return s.Squad(ctx, userID)
}
