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

var ErrAdviceUnavailable = errors.New("predictions unavailable: fresh fixture and player statistics are required")

type ChipAdvice struct {
	Rebuild      *domain.RebuildAdvice `json:"rebuild,omitempty"`
	ID           int                   `json:"id"`
	Name         string                `json:"name"`
	Decision     string                `json:"decision"`
	Reason       string                `json:"reason"`
	BestGameweek int                   `json:"bestGameweek"`
	ExtraPoints  float64               `json:"extraPoints"`
}
type Advice struct {
	domain.Forecast
	TransferHorizonEnd int                     `json:"transferHorizonEnd"`
	SquadRevision      int64                   `json:"squadRevision"`
	GeneratedAt        time.Time               `json:"generatedAt"`
	DataUpdatedAt      time.Time               `json:"dataUpdatedAt"`
	Lineup             domain.LineupAdvice     `json:"lineup"`
	SavedLineupPoints  float64                 `json:"savedLineupPoints"`
	Transfers          []domain.TransferAdvice `json:"transfers"`
	Chips              []ChipAdvice            `json:"chips"`
	Owned              []uuid.UUID             `json:"owned"`
}

func (s *Store) Advice(ctx context.Context, userID uuid.UUID) (Advice, error) {
	var result Advice
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
	// Source and identity map are read from one database snapshot. Source writes
	// commit atomically with the player catalogue, never half a provider refresh.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT payload,fetched_at FROM football_forecast_sources WHERE season_id=$1`, squad.SeasonID).Scan(&raw, &result.DataUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrAdviceUnavailable
	}
	if err != nil {
		return result, err
	}
	if time.Since(result.DataUpdatedAt) > 24*time.Hour {
		return result, ErrAdviceUnavailable
	}
	var source domain.FootballData
	if err = json.Unmarshal(raw, &source); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT p.provider_id,p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 ORDER BY p.provider_id`, squad.SeasonID)
	if err != nil {
		return result, err
	}
	ids := map[int]domain.SquadPlayer{}
	for rows.Next() {
		var id int
		var p domain.SquadPlayer
		if err = rows.Scan(&id, &p.PlayerID, &p.ClubID, &p.ClubName, &p.Name, &p.Position, &p.CurrentPrice); err != nil {
			rows.Close()
			return result, err
		}
		ids[id] = p
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	var transferCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM user_transfer_ledger WHERE squad_id=$1 AND gameweek=$2`, squad.ID, squad.CurrentGameweek).Scan(&transferCount); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	result.GeneratedAt = time.Now().UTC()
	result.SquadRevision = squad.Revision
	result.Forecast, err = domain.ForecastPlayers(source, ids, squad.CurrentGameweek, result.GeneratedAt)
	if err != nil {
		return result, ErrAdviceUnavailable
	}
	predictions := result.Forecast.ByID()
	for i, p := range squad.Players {
		predicted, ok := predictions[p.PlayerID]
		if !ok {
			return result, ErrAdviceUnavailable
		}
		squad.Players[i].CurrentPrice = predicted.CurrentPrice
		result.Owned = append(result.Owned, p.PlayerID)
		result.SavedLineupPoints += predicted.Gameweeks[squad.CurrentGameweek] * float64(tactics.ScoringMultipliers[p.PlayerID])
	}
	result.Lineup, err = domain.RecommendLineup(squad.Players, result.Forecast, squad.CurrentGameweek)
	if err != nil {
		return result, err
	}
	result.TransferHorizonEnd = result.HorizonEnd
	if tactics.ActiveChip == "freehit" {
		result.TransferHorizonEnd = result.Gameweek
	}
	result.Transfers, err = domain.RankTransfers(squad, result.Forecast, tactics.ActiveChip)
	if err != nil {
		return result, err
	}
	result.Chips, err = recommendChips(squad, tactics, result.Forecast, transferCount)
	if err != nil {
		return result, err
	}
	// Mutations use revisions: reject mixed reads rather than advising against a
	// squad or chip inventory changed while this computation was running.
	latest, err := s.Squad(ctx, userID)
	if err != nil {
		return result, err
	}
	if latest.ID != squad.ID || latest.Revision != squad.Revision {
		return result, ErrRevisionConflict
	}
	return result, nil
}

func recommendChips(squad domain.Squad, tactics Tactics, forecast domain.Forecast, transfers int) ([]ChipAdvice, error) {
	result := []ChipAdvice{}
	for _, chip := range tactics.Chips {
		advice := ChipAdvice{ID: chip.ID, Name: chip.Name, Decision: "hold", Reason: "Keep this chip for a stronger opportunity."}
		switch {
		case chip.Used:
			advice.Decision = "unavailable"
			advice.Reason = "This chip set has already been used."
		case squad.CurrentGameweek < chip.Start || squad.CurrentGameweek > chip.End:
			advice.Decision = "unavailable"
			advice.Reason = "Outside this chip set's gameweek window."
		case !tactics.HistoryReviewed:
			advice.Decision = "unavailable"
			advice.Reason = "Confirm your prior chip usage first."
		case tactics.ActiveChip != "":
			advice.Decision = "unavailable"
			advice.Reason = "A chip is already active this gameweek."
		default:
			switch chip.Name {
			case "bboost", "3xc":
				nowGain := 0.0
				bestGain := -1.0
				for gw := forecast.Gameweek; gw <= forecast.HorizonEnd && gw <= chip.End; gw++ {
					lineup, err := domain.RecommendLineup(squad.Players, forecast, gw)
					if err != nil {
						return nil, err
					}
					gain := lineup.BenchPoints
					if chip.Name == "3xc" {
						gain = lineup.CaptainPoints
					}
					if gw == forecast.Gameweek {
						nowGain = gain
					}
					if gain > bestGain {
						bestGain = gain
						advice.BestGameweek = gw
					}
				}
				advice.ExtraPoints = nowGain
				threshold := 12.0
				if chip.Name == "3xc" {
					threshold = 7
				}
				if nowGain >= threshold && advice.BestGameweek == forecast.Gameweek {
					advice.Decision = "consider"
					advice.Reason = "This is the highest estimated extra return within the forecast window. Compare other chips before activating."
				} else {
					advice.Reason = "Hold: a later week is stronger, or the estimated extra points do not meet the conservative threshold."
				}
				if chip.End == forecast.Gameweek && nowGain > 0 {
					advice.Decision = "consider"
					advice.Reason = "This set expires after this gameweek; unused chips do not carry over."
				}
			case "wildcard", "freehit":
				if transfers > 0 {
					advice.Decision = "unavailable"
					advice.Reason = "FPL Edge requires activating this chip before confirming transfers this week."
					break
				}
				if chip.Name == "freehit" {
					for _, previous := range tactics.Chips {
						if previous.Name == "freehit" && previous.Used && (previous.UsedGameweek == nil || *previous.UsedGameweek == forecast.Gameweek-1) {
							advice.Decision = "unavailable"
							advice.Reason = "Consecutive Free Hits are prohibited; a previous use is adjacent or its date is unknown."
						}
					}
				}
				if advice.Decision == "unavailable" {
					break
				}
				plan, err := domain.PlanRebuild(squad, forecast, chip.Name == "freehit")
				if err != nil {
					return nil, err
				}
				advice.Rebuild = &plan
				advice.ExtraPoints = plan.NextWeekGain
				advice.Reason = "Hold: the constrained rebuild does not clear the gain and transfer-count thresholds. This bounded search is not a global optimum."
				if chip.Name == "freehit" && plan.Gain >= 12 && len(plan.Moves) > squad.FreeTransfers+1 {
					advice.Decision = "consider"
					advice.Reason = "A legal temporary rebuild projects at least 12 extra points this week and needs multiple moves. Your original squad returns afterwards; review its next-five outlook too."
				}
				if chip.Name == "wildcard" && plan.Gain >= 20 && len(plan.Moves) > squad.FreeTransfers+1 {
					advice.Decision = "consider"
					advice.Reason = "A legal persistent rebuild projects at least 20 extra points over the fixture window and needs more moves than your free transfers cover. Search is greedy, not globally optimal."
				}
			}
		}
		result = append(result, advice)
	}
	// One chip per gameweek: at most one numerical recommendation is actionable.
	best := -1
	for i, c := range result {
		if c.Decision == "consider" && (best < 0 || c.ExtraPoints > result[best].ExtraPoints) {
			best = i
		}
	}
	for i := range result {
		if i != best && result[i].Decision == "consider" {
			result[i].Decision = "hold"
			result[i].Reason = "Another available chip has a higher estimated immediate return; only one can be played."
		}
	}
	return result, nil
}
