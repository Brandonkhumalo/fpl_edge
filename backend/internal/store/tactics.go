package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrTactics = errors.New("team change rejected")

func tacticsError(message string) error { return fmt.Errorf("%w: %s", ErrTactics, message) }

type ChipOption struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Start        int    `json:"startGameweek"`
	End          int    `json:"endGameweek"`
	UsedGameweek *int   `json:"usedGameweek"`
}
type Tactics struct {
	ScoringMultipliers map[uuid.UUID]int `json:"scoringMultipliers"`
	Revision           int64             `json:"revision"`
	Gameweek           int               `json:"gameweek"`
	Deadline           *time.Time        `json:"deadline"`
	ActiveChip         string            `json:"activeChip"`
	Chips              []ChipOption      `json:"chips"`
}

func (s *Store) Tactics(ctx context.Context, userID uuid.UUID) (Tactics, error) {
	var result Tactics
	var squadID, seasonID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id,season_id,revision,current_gameweek FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1`, userID).Scan(&squadID, &seasonID, &result.Revision, &result.Gameweek)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	err = s.pool.QueryRow(ctx, `SELECT (SELECT deadline_at FROM gameweeks WHERE season_id=$1 AND number=$2),COALESCE((SELECT name FROM squad_chips WHERE squad_id=$3 AND gameweek=$2),'')`, seasonID, result.Gameweek, squadID).Scan(&result.Deadline, &result.ActiveChip)
	if err != nil {
		return result, err
	}
	result.ScoringMultipliers = map[uuid.UUID]int{}
	scoringRows, err := s.pool.Query(ctx, `SELECT sp.player_id,sp.is_starter,sp.player_id=u.captain_id FROM squad_players sp JOIN user_squads u ON u.id=sp.squad_id WHERE sp.squad_id=$1`, squadID)
	if err != nil {
		return result, err
	}
	for scoringRows.Next() {
		var id uuid.UUID
		var starter, captain bool
		if err = scoringRows.Scan(&id, &starter, &captain); err != nil {
			scoringRows.Close()
			return result, err
		}
		weight := 0
		if starter || result.ActiveChip == "bboost" {
			weight = 1
		}
		if captain {
			weight = 2
			if result.ActiveChip == "3xc" {
				weight = 3
			}
		}
		result.ScoringMultipliers[id] = weight
	}
	err = scoringRows.Err()
	scoringRows.Close()
	if err != nil {
		return result, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.provider_id,c.name,c.start_gameweek,c.end_gameweek,u.gameweek FROM season_chips c LEFT JOIN squad_chips u ON u.squad_id=$2 AND u.provider_id=c.provider_id WHERE c.season_id=$1 ORDER BY c.start_gameweek,c.provider_id`, seasonID, squadID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Chips = []ChipOption{}
	for rows.Next() {
		var chip ChipOption
		if err = rows.Scan(&chip.ID, &chip.Name, &chip.Start, &chip.End, &chip.UsedGameweek); err != nil {
			return result, err
		}
		result.Chips = append(result.Chips, chip)
	}
	return result, rows.Err()
}

// PrepareSquad advances the internal planning week and restores a Free Hit
// under the same squad lock used by mutations. Repeated calls are idempotent.
func (s *Store) PrepareSquad(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, seasonID uuid.UUID
	var gw int
	err = tx.QueryRow(ctx, `SELECT id,season_id,current_gameweek FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&id, &seasonID, &gw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var next *int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(min(number) FILTER (WHERE deadline_at>now()),max(number)+1) FROM gameweeks WHERE season_id=$1`, seasonID).Scan(&next); err != nil {
		return err
	}
	if next == nil || *next <= gw {
		return nil
	}
	var backup []byte
	err = tx.QueryRow(ctx, `SELECT backup FROM squad_chips WHERE squad_id=$1 AND name='freehit' AND gameweek=$2`, id, gw).Scan(&backup)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(backup) > 0 {
		var saved squadBackup
		if err = json.Unmarshal(backup, &saved); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM squad_players WHERE squad_id=$1`, id); err != nil {
			return err
		}
		for _, p := range saved.Players {
			if _, err = tx.Exec(ctx, `INSERT INTO squad_players(squad_id,player_id,purchase_price,selling_price,is_starter,bench_order) VALUES($1,$2,$3,$4,$5,$6)`, id, p.ID, p.Purchase, p.Selling, p.Starter, p.Order); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE user_squads SET bank=$2,team_value=$3,free_transfers=$4,captain_id=$5,vice_captain_id=$6 WHERE id=$1`, id, saved.Bank, saved.Value, saved.Free, saved.Captain, saved.Vice); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET current_gameweek=$2,free_transfers=LEAST(5,free_transfers+$3),revision=revision+1,updated_at=now() WHERE id=$1`, id, *next, *next-gw); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) SELECT id,user_id,revision,'gameweek.advanced',jsonb_build_object('gameweek',current_gameweek,'freeHitRestored',$2::boolean) FROM user_squads WHERE id=$1`, id, len(backup) > 0); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type squadBackup struct {
	Bank    int       `json:"bank"`
	Value   int       `json:"value"`
	Free    int       `json:"free"`
	Captain uuid.UUID `json:"captain"`
	Vice    uuid.UUID `json:"vice"`
	Players []struct {
		ID       uuid.UUID `json:"id"`
		Purchase int       `json:"purchase"`
		Selling  int       `json:"selling"`
		Starter  bool      `json:"starter"`
		Order    *int      `json:"order"`
	} `json:"players"`
}

func (s *Store) SetCaptaincy(ctx context.Context, userID uuid.UUID, revision int64, captain, vice uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	var current int64
	err = tx.QueryRow(ctx, `SELECT id,revision FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&id, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current != revision {
		return ErrRevisionConflict
	}
	if captain == vice {
		return tacticsError("captain and vice-captain must be different")
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM squad_players WHERE squad_id=$1 AND is_starter AND player_id=ANY($2)`, id, []uuid.UUID{captain, vice}).Scan(&count); err != nil {
		return err
	}
	if count != 2 {
		return tacticsError("choose captain and vice-captain from your starting XI")
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET captain_id=$2,vice_captain_id=$3,revision=revision+1,updated_at=now() WHERE id=$1`, id, captain, vice); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'captaincy.updated',jsonb_build_object('captainId',$4::text,'viceCaptainId',$5::text))`, id, userID, revision+1, captain.String(), vice.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetChip(ctx context.Context, userID uuid.UUID, revision int64, chipID int, cancel bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, season uuid.UUID
	var current int64
	var gw int
	err = tx.QueryRow(ctx, `SELECT id,season_id,revision,current_gameweek FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&id, &season, &current, &gw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if revision != current {
		return ErrRevisionConflict
	}
	var deadline *time.Time
	if err = tx.QueryRow(ctx, `SELECT (SELECT deadline_at FROM gameweeks WHERE season_id=$1 AND number=$2)`, season, gw).Scan(&deadline); err != nil {
		return err
	}
	if deadline == nil || !deadline.After(time.Now()) {
		return tacticsError("a future gameweek deadline is required")
	}
	var name string
	var start, end int
	err = tx.QueryRow(ctx, `SELECT name,start_gameweek,end_gameweek FROM season_chips WHERE season_id=$1 AND provider_id=$2`, season, chipID).Scan(&name, &start, &end)
	if errors.Is(err, pgx.ErrNoRows) {
		return tacticsError("chip is not available for this season")
	}
	if err != nil {
		return err
	}
	if gw < start || gw > end {
		return tacticsError("chip is outside its availability window")
	}
	if cancel {
		if name == "wildcard" || name == "freehit" {
			return tacticsError("Wildcard and Free Hit cannot be cancelled after activation")
		}
		tag, err := tx.Exec(ctx, `DELETE FROM squad_chips WHERE squad_id=$1 AND provider_id=$2 AND gameweek=$3`, id, chipID, gw)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return tacticsError("chip is not active this gameweek")
		}
	} else {
		var used bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM squad_chips WHERE squad_id=$1 AND (provider_id=$2 OR gameweek=$3 OR (name='freehit' AND $4='freehit' AND gameweek=$3-1)))`, id, chipID, gw, name).Scan(&used); err != nil {
			return err
		}
		if used {
			return tacticsError("chip already used, another chip is active, or Free Hit was used last gameweek")
		}
		var backup []byte
		if name == "wildcard" || name == "freehit" {
			var transfers bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_transfer_ledger WHERE squad_id=$1 AND gameweek=$2)`, id, gw).Scan(&transfers); err != nil {
				return err
			}
			if transfers {
				return tacticsError("activate transfer chips before making transfers in this FPL Edge gameweek")
			}
			if name == "freehit" {
				if err = tx.QueryRow(ctx, `SELECT jsonb_build_object('bank',bank,'value',team_value,'free',free_transfers,'captain',captain_id,'vice',vice_captain_id,'players',(SELECT jsonb_agg(jsonb_build_object('id',player_id,'purchase',purchase_price,'selling',selling_price,'starter',is_starter,'order',bench_order)) FROM squad_players WHERE squad_id=$1)) FROM user_squads WHERE id=$1`, id).Scan(&backup); err != nil {
					return err
				}
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO squad_chips(squad_id,provider_id,name,gameweek,backup) VALUES($1,$2,$3,$4,$5)`, id, chipID, name, gw, backup); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET revision=revision+1,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'chip.updated',jsonb_build_object('chip',$4::text,'cancelled',$5::boolean,'gameweek',$6::integer))`, id, userID, revision+1, name, cancel, gw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
