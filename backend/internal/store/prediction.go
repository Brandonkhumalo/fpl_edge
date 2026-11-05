package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

type SquadPrediction struct {
	SquadRevision int64     `json:"squadRevision"`
	Gameweek      int       `json:"gameweek"`
	Points        float64   `json:"points"`
	ModelVersion  string    `json:"modelVersion"`
	ActiveChip    string    `json:"activeChip"`
	GeneratedAt   time.Time `json:"generatedAt"`
	DataUpdatedAt time.Time `json:"dataUpdatedAt"`
}

// Prediction calculates only the saved scoring selection. It deliberately does
// not depend on transfer ranking, chip recommendations or lineup optimization.
func (s *Store) Prediction(ctx context.Context, userID uuid.UUID) (SquadPrediction, error) {
	var result SquadPrediction
	squad, err := s.Squad(ctx, userID)
	if err != nil {
		return result, err
	}
	tactics, err := s.Tactics(ctx, userID)
	if err != nil {
		return result, err
	}
	if tactics.Revision != squad.Revision {
		return result, ErrRevisionConflict
	}
	if tactics.Deadline == nil || !tactics.Deadline.After(time.Now()) {
		return result, ErrAdviceUnavailable
	}
	forecast, dataUpdatedAt, generatedAt, err := s.forecastForSquad(ctx, squad)
	if err != nil {
		return result, err
	}
	result.Points, err = savedLineupPoints(squad.Players, forecast, squad.CurrentGameweek, tactics.ScoringMultipliers)
	if err != nil {
		return result, err
	}
	latest, err := s.Squad(ctx, userID)
	if err != nil {
		return result, err
	}
	if latest.ID != squad.ID || latest.Revision != squad.Revision {
		return result, ErrRevisionConflict
	}
	result.SquadRevision = squad.Revision
	result.Gameweek = squad.CurrentGameweek
	result.ModelVersion = forecast.ModelVersion
	result.ActiveChip = tactics.ActiveChip
	result.GeneratedAt = generatedAt
	result.DataUpdatedAt = dataUpdatedAt
	return result, nil
}

func savedLineupPoints(players []domain.SquadPlayer, forecast domain.Forecast, gameweek int, multipliers map[uuid.UUID]int) (float64, error) {
	predictions := forecast.ByID()
	points := 0.0
	for _, player := range players {
		prediction, ok := predictions[player.PlayerID]
		if !ok {
			return 0, ErrAdviceUnavailable
		}
		points += prediction.Gameweeks[gameweek] * float64(multipliers[player.PlayerID])
	}
	return points, nil
}

func (s *Store) forecastForSquad(ctx context.Context, squad domain.Squad) (domain.Forecast, time.Time, time.Time, error) {
	var empty domain.Forecast
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var dataUpdatedAt time.Time
	err = tx.QueryRow(ctx, `SELECT payload,fetched_at FROM football_forecast_sources WHERE season_id=$1`, squad.SeasonID).Scan(&raw, &dataUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, time.Time{}, time.Time{}, ErrAdviceUnavailable
	}
	if err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	if time.Since(dataUpdatedAt) > 24*time.Hour {
		return empty, time.Time{}, time.Time{}, ErrAdviceUnavailable
	}
	var source domain.FootballData
	if err = json.Unmarshal(raw, &source); err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	rows, err := tx.Query(ctx, `SELECT p.provider_id,p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 ORDER BY p.provider_id`, squad.SeasonID)
	if err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	identities := map[int]domain.SquadPlayer{}
	for rows.Next() {
		var providerID int
		var player domain.SquadPlayer
		if err = rows.Scan(&providerID, &player.PlayerID, &player.ClubID, &player.ClubName, &player.Name, &player.Position, &player.CurrentPrice); err != nil {
			rows.Close()
			return empty, time.Time{}, time.Time{}, err
		}
		identities[providerID] = player
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, time.Time{}, time.Time{}, err
	}
	generatedAt := time.Now().UTC()
	forecast, err := domain.ForecastPlayers(source, identities, squad.CurrentGameweek, generatedAt)
	if err != nil {
		return empty, time.Time{}, time.Time{}, ErrAdviceUnavailable
	}
	return forecast, dataUpdatedAt, generatedAt, nil
}
