package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/advisor"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

type AdvisorGrounding struct {
	SquadID    uuid.UUID
	SnapshotID uuid.UUID
	Evidence   []advisor.Evidence
}

type AdvisorAnswer struct {
	ID          uuid.UUID          `json:"id"`
	Answer      string             `json:"answer"`
	Kind        string             `json:"kind"`
	Evidence    []advisor.Evidence `json:"evidence"`
	Provider    string             `json:"provider"`
	Model       string             `json:"model"`
	GeneratedAt time.Time          `json:"generatedAt"`
}

func (s *Store) AdvisorRequestAllowed(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM grounded_advisor_messages WHERE user_id=$1 AND created_at>now()-interval '1 hour'`, userID).Scan(&count)
	return count < 20, err
}

func (s *Store) AdvisorGrounding(ctx context.Context, userID uuid.UUID, question string) (AdvisorGrounding, error) {
	adviceResult, err := s.Advice(ctx, userID)
	if err != nil {
		return AdvisorGrounding{}, err
	}
	squad, err := s.Squad(ctx, userID)
	if err != nil {
		return AdvisorGrounding{}, err
	}
	var snapshotID uuid.UUID
	err = s.pool.QueryRow(ctx, `SELECT id FROM recommendation_snapshots WHERE user_id=$1 AND squad_id=$2 AND squad_revision=$3 AND model_version=$4 ORDER BY created_at DESC LIMIT 1`, userID, squad.ID, adviceResult.SquadRevision, adviceResult.ModelVersion).Scan(&snapshotID)
	if err != nil {
		return AdvisorGrounding{}, err
	}
	byID := adviceResult.ByID()
	name := func(id uuid.UUID) string {
		if p, ok := byID[id]; ok {
			return p.Name
		}
		return "Unknown player"
	}
	source := fmt.Sprintf("recommendation_snapshot:%s", snapshotID)
	evidence := []advisor.Evidence{{ID: "provenance", Category: "provenance", Text: fmt.Sprintf("The evidence uses model %s, squad revision %d, and source data updated %s.", adviceResult.ModelVersion, adviceResult.SquadRevision, adviceResult.DataUpdatedAt.UTC().Format(time.RFC3339)), Source: source}}
	captain := byID[adviceResult.Lineup.CaptainID]
	evidence = append(evidence, advisor.Evidence{ID: "captain", Category: "captain", Text: fmt.Sprintf("%s is the recommended captain with %.1f expected points before the captain multiplier in Gameweek %d; %s is vice-captain.", captain.Name, adviceResult.Lineup.CaptainPoints, adviceResult.Gameweek, name(adviceResult.Lineup.ViceCaptainID)), Source: source})
	for index, fixture := range captain.Fixtures {
		if fixture.Gameweek == adviceResult.Gameweek {
			evidence = append(evidence, advisor.Evidence{ID: fmt.Sprintf("captain_fixture_%d", index+1), Category: "fixture", Text: fmt.Sprintf("%s faces %s %s in Gameweek %d: FDR %d and %.1f expected points from the persisted fixture model.", captain.Name, fixture.Opponent, map[bool]string{true: "at home", false: "away"}[fixture.Home], fixture.Gameweek, fixture.Difficulty, fixture.Points), Source: source})
		}
	}
	evidence = append(evidence, advisor.Evidence{ID: "lineup", Category: "lineup", Text: fmt.Sprintf("The recommended XI plus captain projects %.1f points; the currently saved selection projects %.1f points with the active chip included.", adviceResult.Lineup.Points, adviceResult.SavedLineupPoints), Source: source})
	for index, move := range adviceResult.Transfers {
		if index >= 3 {
			break
		}
		evidence = append(evidence, advisor.Evidence{ID: fmt.Sprintf("transfer_%d", index+1), Category: "transfer", Text: fmt.Sprintf("Transfer option %d is %s to %s: %.1f expected net points across the comparison window, %.1f next-Gameweek net points, a %d-point hit, and £%.1fm remaining.", index+1, name(move.OutID), name(move.InID), move.NetGain, move.NextWeekGain, move.Hit, float64(move.BankAfter)/10), Source: source})
	}
	if len(adviceResult.Transfers) == 0 {
		evidence = append(evidence, advisor.Evidence{ID: "transfer_hold", Category: "transfer", Text: "No legal one-player transfer clears the engine's two-point expected net-gain threshold with the required fixture coverage.", Source: source})
	}
	for _, chip := range adviceResult.Chips {
		evidence = append(evidence, advisor.Evidence{ID: fmt.Sprintf("chip_%d", chip.ID), Category: "chip", Text: fmt.Sprintf("%s is marked %s: %s", chip.Name, chip.Decision, chip.Reason), Source: source})
	}
	owned := map[uuid.UUID]bool{}
	for _, id := range adviceResult.Owned {
		owned[id] = true
	}
	players := append([]domain.PlayerProjection(nil), adviceResult.Players...)
	sort.Slice(players, func(i, j int) bool { return players[i].NextFiveTotal > players[j].NextFiveTotal })
	query := strings.ToLower(question)
	included := 0
	for _, player := range players {
		matched := strings.Contains(query, strings.ToLower(player.Name))
		for _, part := range strings.Fields(strings.ToLower(player.Name)) {
			if len(part) > 2 && strings.Contains(query, part) {
				matched = true
			}
		}
		if !owned[player.PlayerID] && !matched && included >= 5 {
			continue
		}
		evidence = append(evidence, advisor.Evidence{ID: "player_" + player.PlayerID.String(), Category: "player", Text: fmt.Sprintf("%s (%s, %s, £%.1fm) projects %.1f points over the next five scheduled fixtures with %d/5 fixture coverage and %s evidence strength.", player.Name, player.ClubName, player.Position, float64(player.CurrentPrice)/10, player.NextFiveTotal, player.Coverage, player.Confidence), Source: source})
		included++
		if included >= 22 {
			break
		}
	}
	plan, planErr := domain.PlanTransfers(squad, adviceResult.Forecast, 3, "conservative")
	if planErr == nil {
		planSource := fmt.Sprintf("computed_from:%s", snapshotID)
		evidence = append(evidence, advisor.Evidence{ID: "plan_summary", Category: "plan", Text: fmt.Sprintf("A bounded conservative three-Gameweek route projects %.1f points versus %.1f without planned changes, a %.1f expected-point difference; its effective horizon is %d Gameweeks.", plan.ProjectedPoints, plan.BaselinePoints, plan.ExpectedGain, plan.EffectiveHorizon), Source: planSource})
		for index, week := range plan.Weeks {
			decision := "roll the transfer"
			if week.Transfer != nil {
				decision = fmt.Sprintf("move %s to %s with a %d-point hit", name(week.Transfer.OutID), name(week.Transfer.InID), week.Transfer.Hit)
			}
			evidence = append(evidence, advisor.Evidence{ID: fmt.Sprintf("plan_week_%d", index+1), Category: "plan", Text: fmt.Sprintf("For Gameweek %d the bounded conservative route says to %s; projected XI points are %.1f, with %d free transfer(s) and £%.1fm bank carried forward.", week.Gameweek, decision, week.ProjectedXI, week.FreeAfter, float64(week.BankAfter)/10), Source: planSource})
		}
	}
	var previousPayload []byte
	var previousID uuid.UUID
	err = s.pool.QueryRow(ctx, `SELECT id,recommendation FROM recommendation_snapshots WHERE user_id=$1 AND id<>$2 ORDER BY created_at DESC LIMIT 1`, userID, snapshotID).Scan(&previousID, &previousPayload)
	if err == nil {
		var previous Advice
		if json.Unmarshal(previousPayload, &previous) == nil {
			changes := []string{}
			if previous.Lineup.CaptainID != adviceResult.Lineup.CaptainID {
				changes = append(changes, fmt.Sprintf("captain changed from %s to %s", name(previous.Lineup.CaptainID), name(adviceResult.Lineup.CaptainID)))
			}
			difference := adviceResult.SavedLineupPoints - previous.SavedLineupPoints
			if difference != 0 {
				changes = append(changes, fmt.Sprintf("saved-selection forecast changed by %.1f points", difference))
			}
			if len(changes) == 0 {
				changes = append(changes, "the captain and saved-selection forecast are unchanged")
			}
			evidence = append(evidence, advisor.Evidence{ID: "change_latest", Category: "change", Text: "Compared with the previous immutable snapshot, " + strings.Join(changes, " and ") + ".", Source: fmt.Sprintf("recommendation_snapshots:%s,%s", previousID, snapshotID)})
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		evidence = append(evidence, advisor.Evidence{ID: "change_unavailable", Category: "change", Text: "There is no earlier immutable recommendation snapshot to compare with the current one.", Source: source})
	} else {
		return AdvisorGrounding{}, err
	}
	for index, warning := range adviceResult.Warnings {
		evidence = append(evidence, advisor.Evidence{ID: fmt.Sprintf("model_warning_%d", index+1), Category: "provenance", Text: warning, Source: source})
	}
	return AdvisorGrounding{SquadID: squad.ID, SnapshotID: snapshotID, Evidence: evidence}, nil
}

func (s *Store) SaveAdvisorAnswer(ctx context.Context, userID uuid.UUID, question string, grounding AdvisorGrounding, selection advisor.Selection) (AdvisorAnswer, error) {
	byID := map[string]advisor.Evidence{}
	for _, item := range grounding.Evidence {
		byID[item.ID] = item
	}
	selected := []advisor.Evidence{}
	for _, id := range selection.EvidenceIDs {
		if item, ok := byID[id]; ok {
			selected = append(selected, item)
		}
	}
	intro := map[string]string{"explanation": "Here is the engine evidence behind that recommendation.", "comparison": "Here is the persisted evidence for that comparison.", "plan": "Here is the bounded plan evidence; it is not a guaranteed global optimum.", "change": "Here is what the immutable snapshots show.", "unknown": "I can only answer from the current persisted squad, forecast, transfer, chip, plan, and snapshot evidence."}[selection.Kind]
	parts := []string{intro}
	for _, item := range selected {
		parts = append(parts, item.Text+" ["+item.ID+"]")
	}
	answer := AdvisorAnswer{Answer: strings.Join(parts, " "), Kind: selection.Kind, Evidence: selected, Provider: selection.Provider, Model: selection.Model, GeneratedAt: time.Now().UTC()}
	evidenceJSON, _ := json.Marshal(grounding.Evidence)
	responseJSON, _ := json.Marshal(answer)
	err := s.pool.QueryRow(ctx, `INSERT INTO grounded_advisor_messages(user_id,squad_id,recommendation_snapshot_id,question,evidence,response,provider,model)
		SELECT $1,s.id,r.id,$4,$5,$6,$7,$8 FROM user_squads s JOIN recommendation_snapshots r ON r.id=$3 AND r.user_id=$1 WHERE s.id=$2 AND s.user_id=$1
		RETURNING grounded_advisor_messages.id,grounded_advisor_messages.created_at`, userID, grounding.SquadID, grounding.SnapshotID, question, evidenceJSON, responseJSON, selection.Provider, selection.Model).Scan(&answer.ID, &answer.GeneratedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdvisorAnswer{}, ErrNotFound
	}
	return answer, err
}
