package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

type DeadlineStatus struct {
	Gameweek      int        `json:"gameweek"`
	Deadline      time.Time  `json:"deadline"`
	Timezone      string     `json:"timezone"`
	FreeTransfers int        `json:"freeTransfers"`
	ActiveChip    string     `json:"activeChip"`
	Ready         bool       `json:"ready"`
	DataUpdatedAt *time.Time `json:"dataUpdatedAt,omitempty"`
	Checklist     []Check    `json:"checklist"`
	Notifications []Notice   `json:"notifications"`
}

type Check struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Notice struct {
	ID        uuid.UUID `json:"id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) Deadline(ctx context.Context, userID uuid.UUID) (DeadlineStatus, error) {
	squad, err := s.Squad(ctx, userID)
	if err != nil {
		return DeadlineStatus{}, err
	}
	tactics, err := s.Tactics(ctx, userID)
	if err != nil {
		return DeadlineStatus{}, err
	}
	if tactics.Deadline == nil {
		return DeadlineStatus{}, ErrAdviceUnavailable
	}
	result := DeadlineStatus{Gameweek: squad.CurrentGameweek, Deadline: *tactics.Deadline, FreeTransfers: squad.FreeTransfers, ActiveChip: tactics.ActiveChip, Ready: true, Checklist: []Check{}}
	var reminderHours int
	var inApp, email, push bool
	if err = s.pool.QueryRow(ctx, `SELECT timezone,deadline_reminder_hours,in_app_alerts,email_alerts,web_push_alerts FROM user_preferences WHERE user_id=$1`, userID).Scan(&result.Timezone, &reminderHours, &inApp, &email, &push); err != nil {
		return result, err
	}
	add := func(key, label, status, detail string) {
		result.Checklist = append(result.Checklist, Check{key, label, status, detail})
		if status == "action" {
			result.Ready = false
		}
	}
	add("squad", "Squad saved", "ready", fmt.Sprintf("Revision %d is stored in FPL Edge.", squad.Revision))
	if squad.CaptainID == uuid.Nil || squad.ViceCaptainID == uuid.Nil || squad.CaptainID == squad.ViceCaptainID {
		add("captaincy", "Captain and vice-captain", "action", "Choose two different starting players.")
	} else {
		add("captaincy", "Captain and vice-captain", "ready", "Both selections are saved.")
	}
	advice, adviceErr := s.Advice(ctx, userID)
	if adviceErr != nil {
		add("data", "Fresh prediction data", "action", adviceErr.Error())
	} else {
		result.DataUpdatedAt = &advice.DataUpdatedAt
		flagged, blank, doubles := 0, 0, 0
		owned := map[uuid.UUID]bool{}
		for _, id := range advice.Owned {
			owned[id] = true
		}
		for _, player := range advice.Players {
			if !owned[player.PlayerID] {
				continue
			}
			if player.Status != "a" {
				flagged++
			}
			games := 0
			for _, fixture := range player.Fixtures {
				if fixture.Gameweek == advice.Gameweek {
					games++
				}
			}
			if games == 0 {
				blank++
			}
			if games > 1 {
				doubles++
			}
		}
		status := "ready"
		if flagged > 0 {
			status = "action"
		}
		add("availability", "Player availability", status, fmt.Sprintf("%d flagged player(s).", flagged))
		add("fixtures", "Blank and Double exposure", "ready", fmt.Sprintf("%d blank and %d double player(s) in the current squad.", blank, doubles))
		add("recommendations", "Current recommendations", "ready", fmt.Sprintf("Calculated for squad revision %d.", advice.SquadRevision))
	}
	expiring := 0
	for _, chip := range tactics.Chips {
		if !chip.Used && chip.End == squad.CurrentGameweek {
			expiring++
		}
	}
	chipStatus := "ready"
	if expiring > 0 {
		chipStatus = "action"
	}
	add("chips", "Chip expiry", chipStatus, fmt.Sprintf("%d chip set(s) expire after this Gameweek.", expiring))
	add("transfers", "Free transfers", "ready", fmt.Sprintf("%d available.", squad.FreeTransfers))
	if inApp && time.Until(*tactics.Deadline) > 0 && time.Until(*tactics.Deadline) <= time.Duration(reminderHours)*time.Hour {
		var notificationID uuid.UUID
		dedupe := fmt.Sprintf("deadline:%s:%d", squad.ID, squad.CurrentGameweek)
		err = s.pool.QueryRow(ctx, `INSERT INTO user_notifications(user_id,category,title,message,dedupe_key,payload) VALUES($1,'deadline','Gameweek deadline approaching',$2,$3,jsonb_build_object('gameweek',$4,'deadline',$5::text)) ON CONFLICT(user_id,dedupe_key) DO UPDATE SET dedupe_key=EXCLUDED.dedupe_key RETURNING id`, userID, fmt.Sprintf("Review your squad before the Gameweek %d deadline.", squad.CurrentGameweek), dedupe, squad.CurrentGameweek, tactics.Deadline.UTC().Format(time.RFC3339)).Scan(&notificationID)
		if err != nil {
			return result, err
		}
		for channel, enabled := range map[string]bool{"email": email, "web_push": push} {
			if enabled {
				_, err = s.pool.Exec(ctx, `INSERT INTO notification_deliveries(notification_id,channel,status,last_error) VALUES($1,$2,'disabled','Delivery adapter is not configured') ON CONFLICT DO NOTHING`, notificationID, channel)
				if err != nil {
					return result, err
				}
			}
		}
	}
	result.Notifications, err = s.Notifications(ctx, userID)
	return result, err
}

func (s *Store) Notifications(ctx context.Context, userID uuid.UUID) ([]Notice, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,category,title,message,read_at IS NOT NULL,created_at FROM user_notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Notice{}
	for rows.Next() {
		var n Notice
		if err = rows.Scan(&n.ID, &n.Category, &n.Title, &n.Message, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

type WatchedPlayer struct {
	domain.SquadPlayer
	Status            string    `json:"status"`
	SelectedByPercent *float64  `json:"selectedByPercent"`
	TotalPoints       int       `json:"totalPoints"`
	ForecastPoints    *float64  `json:"forecastPoints"`
	WatchedAt         time.Time `json:"watchedAt"`
}

func (s *Store) AddWatch(ctx context.Context, userID, playerID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `INSERT INTO player_watchlists(user_id,player_id) SELECT $1,p.id FROM players p JOIN seasons s ON s.id=p.season_id AND s.is_current WHERE p.id=$2 ON CONFLICT DO NOTHING`, userID, playerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_watchlists WHERE user_id=$1 AND player_id=$2)`, userID, playerID).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}
func (s *Store) RemoveWatch(ctx context.Context, userID, playerID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM player_watchlists WHERE user_id=$1 AND player_id=$2`, userID, playerID)
	return err
}

func (s *Store) Watchlist(ctx context.Context, userID uuid.UUID) ([]WatchedPlayer, error) {
	forecast := map[uuid.UUID]domain.PlayerProjection{}
	if advice, err := s.Advice(ctx, userID); err == nil {
		forecast = advice.ByID()
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.club_id,c.short_name,COALESCE(c.kit_code,0),p.squad_number,p.first_name||' '||p.last_name,p.position,p.price,p.price,p.price,0,COALESCE(p.expected_minutes,0),p.status,p.selected_by_percent,p.total_points,w.created_at FROM player_watchlists w JOIN players p ON p.id=w.player_id JOIN clubs c ON c.id=p.club_id WHERE w.user_id=$1 ORDER BY w.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WatchedPlayer{}
	for rows.Next() {
		var p WatchedPlayer
		if err = rows.Scan(&p.PlayerID, &p.ClubID, &p.ClubName, &p.KitCode, &p.SquadNumber, &p.Name, &p.Position, &p.CurrentPrice, &p.PurchasePrice, &p.SellingPrice, &p.ExpectedPoints, &p.ExpectedMins, &p.Status, &p.SelectedByPercent, &p.TotalPoints, &p.WatchedAt); err != nil {
			return nil, err
		}
		if f, ok := forecast[p.PlayerID]; ok {
			value := f.NextFiveTotal
			p.ForecastPoints = &value
		}
		result = append(result, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for _, player := range result {
		if err = s.recordWatchObservation(ctx, userID, player); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) recordWatchObservation(ctx context.Context, userID uuid.UUID, p WatchedPlayer) error {
	var oldPrice int
	var oldStatus string
	var oldForecast *float64
	err := s.pool.QueryRow(ctx, `SELECT price,status,forecast_points FROM watchlist_observations WHERE user_id=$1 AND player_id=$2`, userID, p.PlayerID).Scan(&oldPrice, &oldStatus, &oldForecast)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		title, message, key := "", "", ""
		switch {
		case oldStatus != p.Status:
			title = "Availability changed"
			message = fmt.Sprintf("%s changed from %s to %s.", p.Name, oldStatus, p.Status)
			key = fmt.Sprintf("watch:%s:status:%s", p.PlayerID, p.Status)
		case oldPrice != p.CurrentPrice:
			title = "Price changed"
			message = fmt.Sprintf("%s is now £%.1fm.", p.Name, float64(p.CurrentPrice)/10)
			key = fmt.Sprintf("watch:%s:price:%d", p.PlayerID, p.CurrentPrice)
		case oldForecast != nil && p.ForecastPoints != nil && (*p.ForecastPoints-*oldForecast >= 1 || *oldForecast-*p.ForecastPoints >= 1):
			title = "Forecast changed"
			message = fmt.Sprintf("%s now projects %.1f points over the next five fixtures.", p.Name, *p.ForecastPoints)
			key = fmt.Sprintf("watch:%s:forecast:%.1f", p.PlayerID, *p.ForecastPoints)
		}
		if key != "" {
			_, err = s.pool.Exec(ctx, `INSERT INTO user_notifications(user_id,category,title,message,dedupe_key,payload) VALUES($1,'watchlist',$2,$3,$4,jsonb_build_object('playerId',$5::text)) ON CONFLICT DO NOTHING`, userID, title, message, key, p.PlayerID.String())
			if err != nil {
				return err
			}
		}
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO watchlist_observations(user_id,player_id,price,status,ownership,forecast_points) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(user_id,player_id) DO UPDATE SET price=EXCLUDED.price,status=EXCLUDED.status,ownership=EXCLUDED.ownership,forecast_points=EXCLUDED.forecast_points,observed_at=now()`, userID, p.PlayerID, p.CurrentPrice, p.Status, p.SelectedByPercent, p.ForecastPoints)
	return err
}

type RecommendationRecord struct {
	ID             uuid.UUID       `json:"id"`
	Gameweek       int             `json:"gameweek"`
	SquadRevision  int64           `json:"squadRevision"`
	ModelVersion   string          `json:"modelVersion"`
	DataUpdatedAt  time.Time       `json:"dataUpdatedAt"`
	ExpectedPoints float64         `json:"expectedPoints"`
	Confidence     string          `json:"confidence"`
	SelectedAction json.RawMessage `json:"selectedAction,omitempty"`
	ActualPoints   *int            `json:"actualPoints"`
	OutcomeSource  *string         `json:"outcomeSource"`
	CreatedAt      time.Time       `json:"createdAt"`
}

func (s *Store) RecommendationHistory(ctx context.Context, userID uuid.UUID) ([]RecommendationRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id,r.gameweek,r.squad_revision,r.model_version,r.data_updated_at,r.expected_points,r.confidence,r.selected_action,o.actual_points,o.source,r.created_at FROM recommendation_snapshots r LEFT JOIN recommendation_outcomes o ON o.snapshot_id=r.id WHERE r.user_id=$1 ORDER BY r.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RecommendationRecord{}
	for rows.Next() {
		var r RecommendationRecord
		if err = rows.Scan(&r.ID, &r.Gameweek, &r.SquadRevision, &r.ModelVersion, &r.DataUpdatedAt, &r.ExpectedPoints, &r.Confidence, &r.SelectedAction, &r.ActualPoints, &r.OutcomeSource, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s *Store) SelectRecommendation(ctx context.Context, userID, id uuid.UUID, action json.RawMessage) error {
	tag, err := s.pool.Exec(ctx, `UPDATE recommendation_snapshots SET selected_action=$3,selected_at=now() WHERE id=$2 AND user_id=$1`, userID, id, action)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) SetRecommendationOutcome(ctx context.Context, userID, id uuid.UUID, points int, notes string) error {
	if points < 0 {
		return errors.New("actual points cannot be negative")
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO recommendation_outcomes(snapshot_id,actual_points,notes) SELECT id,$3,$4 FROM recommendation_snapshots WHERE id=$2 AND user_id=$1 ON CONFLICT(snapshot_id) DO UPDATE SET actual_points=EXCLUDED.actual_points,notes=EXCLUDED.notes,settled_at=now()`, userID, id, points, notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type SavedTransferPlan struct {
	ID               uuid.UUID           `json:"id"`
	Name             string              `json:"name"`
	Strategy         string              `json:"strategy"`
	RequestedHorizon int                 `json:"requestedHorizon"`
	SquadRevision    int64               `json:"squadRevision"`
	Stale            bool                `json:"stale"`
	Assumptions      json.RawMessage     `json:"assumptions"`
	Plan             domain.TransferPlan `json:"plan"`
	CreatedAt        time.Time           `json:"createdAt"`
}

func (s *Store) GenerateTransferPlan(ctx context.Context, userID uuid.UUID, name, strategy string, horizon int) (SavedTransferPlan, error) {
	squad, err := s.Squad(ctx, userID)
	if err != nil {
		return SavedTransferPlan{}, err
	}
	advice, err := s.Advice(ctx, userID)
	if err != nil {
		return SavedTransferPlan{}, err
	}
	plan, err := domain.PlanTransfers(squad, advice.Forecast, horizon, strategy)
	if err != nil {
		return SavedTransferPlan{}, err
	}
	assumptions, _ := json.Marshal(map[string]any{"modelVersion": advice.ModelVersion, "dataUpdatedAt": advice.DataUpdatedAt, "boundedSearch": true, "priceChangesSimulated": false})
	payload, _ := json.Marshal(plan)
	result := SavedTransferPlan{Name: name, Strategy: strategy, RequestedHorizon: horizon, SquadRevision: squad.Revision, Assumptions: assumptions, Plan: plan}
	err = s.pool.QueryRow(ctx, `INSERT INTO transfer_plans(user_id,squad_id,name,strategy,requested_horizon,effective_horizon,squad_revision,assumptions,plan,expected_gain) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,created_at`, userID, squad.ID, name, strategy, horizon, plan.EffectiveHorizon, squad.Revision, assumptions, payload, plan.ExpectedGain).Scan(&result.ID, &result.CreatedAt)
	return result, err
}
func (s *Store) TransferPlans(ctx context.Context, userID uuid.UUID) ([]SavedTransferPlan, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.name,p.strategy,p.requested_horizon,p.squad_revision,p.squad_revision<>s.revision,p.assumptions,p.plan,p.created_at FROM transfer_plans p JOIN user_squads s ON s.id=p.squad_id WHERE p.user_id=$1 ORDER BY p.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SavedTransferPlan{}
	for rows.Next() {
		var p SavedTransferPlan
		var raw []byte
		if err = rows.Scan(&p.ID, &p.Name, &p.Strategy, &p.RequestedHorizon, &p.SquadRevision, &p.Stale, &p.Assumptions, &raw, &p.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &p.Plan); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
