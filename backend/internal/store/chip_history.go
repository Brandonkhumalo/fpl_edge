package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PriorChipUsage struct {
	ChipID   int  `json:"chipId"`
	Gameweek *int `json:"gameweek"`
}

// SetChipHistory replaces only self-reported history. In-app activations and
// squad contents remain untouched; the squad lock serializes both workflows.
func (s *Store) SetChipHistory(ctx context.Context, userID uuid.UUID, revision int64, usages []PriorChipUsage) error {
	if usages == nil || len(usages) > 60 {
		return tacticsError("provide a chip history list, including an empty list if none were used")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var squadID, seasonID uuid.UUID
	var current int64
	var planningWeek int
	err = tx.QueryRow(ctx, `SELECT id,season_id,revision,current_gameweek FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&squadID, &seasonID, &current, &planningWeek)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current != revision {
		return ErrRevisionConflict
	}

	type window struct {
		name       string
		start, end int
	}
	windows := map[int]window{}
	rows, err := tx.Query(ctx, `SELECT provider_id,name,start_gameweek,end_gameweek FROM season_chips WHERE season_id=$1`, seasonID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		var w window
		if err = rows.Scan(&id, &w.name, &w.start, &w.end); err != nil {
			rows.Close()
			return err
		}
		windows[id] = w
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		return tacticsError("season chip rules are not available yet")
	}

	elapsed := map[int]bool{}
	rows, err = tx.Query(ctx, `SELECT number FROM gameweeks WHERE season_id=$1 AND deadline_at<=now() AND number<$2`, seasonID, planningWeek)
	if err != nil {
		return err
	}
	for rows.Next() {
		var gw int
		if err = rows.Scan(&gw); err != nil {
			rows.Close()
			return err
		}
		elapsed[gw] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	local := map[int]bool{}
	occupied := map[int]bool{}
	var freeHits []int
	rows, err = tx.Query(ctx, `SELECT provider_id,name,gameweek FROM squad_chips WHERE squad_id=$1`, squadID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, gw int
		var name string
		if err = rows.Scan(&id, &name, &gw); err != nil {
			rows.Close()
			return err
		}
		local[id] = true
		occupied[gw] = true
		if name == "freehit" {
			freeHits = append(freeHits, gw)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	seen := map[int]bool{}
	var unknownFreeHits []window
	for _, use := range usages {
		w, ok := windows[use.ChipID]
		if !ok {
			return tacticsError("chip does not belong to your squad's season")
		}
		if seen[use.ChipID] {
			return tacticsError("record each chip window only once")
		}
		if local[use.ChipID] {
			return tacticsError("chips used in FPL Edge cannot be overwritten by prior history")
		}
		seen[use.ChipID] = true
		hasElapsed := false
		for gw := range elapsed {
			if gw >= w.start && gw <= w.end {
				hasElapsed = true
				break
			}
		}
		if !hasElapsed {
			return tacticsError("cannot record a chip from a future availability window")
		}
		if use.Gameweek != nil {
			gw := *use.Gameweek
			if gw < w.start || gw > w.end || !elapsed[gw] {
				return tacticsError("choose a past gameweek within this chip's availability window")
			}
			if occupied[gw] {
				return tacticsError("only one chip can be recorded for a gameweek")
			}
			occupied[gw] = true
			if w.name == "freehit" {
				freeHits = append(freeHits, gw)
			}
		} else if w.name == "freehit" {
			unknownFreeHits = append(unknownFreeHits, w)
		}
	}
	for i, gw := range freeHits {
		for _, other := range freeHits[i+1:] {
			if gw == other+1 || gw == other-1 {
				return tacticsError("Free Hit cannot be used in consecutive gameweeks")
			}
		}
		for _, w := range unknownFreeHits {
			if (elapsed[gw-1] && gw-1 >= w.start && gw-1 <= w.end) || (elapsed[gw+1] && gw+1 >= w.start && gw+1 <= w.end) {
				return tacticsError("record the exact Free Hit gameweek to check consecutive usage")
			}
		}
	}
	var previous []PriorChipUsage
	rows, err = tx.Query(ctx, `SELECT provider_id,gameweek FROM prior_chip_usage WHERE squad_id=$1 ORDER BY provider_id`, squadID)
	if err != nil {
		return err
	}
	previous = []PriorChipUsage{}
	for rows.Next() {
		var use PriorChipUsage
		if err = rows.Scan(&use.ChipID, &use.Gameweek); err != nil {
			rows.Close()
			return err
		}
		previous = append(previous, use)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM prior_chip_usage WHERE squad_id=$1`, squadID); err != nil {
		return err
	}
	for _, use := range usages {
		if _, err = tx.Exec(ctx, `INSERT INTO prior_chip_usage(squad_id,provider_id,gameweek) VALUES($1,$2,$3)`, squadID, use.ChipID, use.Gameweek); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET chip_history_reviewed=true,revision=revision+1,updated_at=now() WHERE id=$1`, squadID); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"before": previous, "after": usages})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'chip_history.updated',$4)`, squadID, userID, revision+1, payload); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
