package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"github.com/tishanyq/fpl-edge/backend/migrations"
	"os"
	"testing"
	"time"
)

func TestAdviceIntegration(t *testing.T) {
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
	schema := "advice_" + uuid.New().String()[:8]
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := pool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); e != nil {
			t.Error(e)
		}
	}()
	config := pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	testAdviceDatabase(t, ctx, isolated)
}

func testAdviceDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var err error
	if err = migrations.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := New(pool)
	now := time.Now().UTC()
	minutes := 900
	data := domain.FootballData{SeasonName: "Advice " + uuid.NewString(), StartsOn: now.AddDate(0, -1, 0), EndsOn: now.AddDate(0, 6, 0), Chips: []domain.FootballChip{{ID: 1, Name: "bboost", Start: 1, End: 7}, {ID: 2, Name: "3xc", Start: 1, End: 7}, {ID: 3, Name: "freehit", Start: 1, End: 7}}}
	for club := 1; club <= 6; club++ {
		data.Clubs = append(data.Clubs, domain.FootballClub{ProviderID: club, Name: fmt.Sprintf("Club%d", club), ShortName: fmt.Sprintf("C%d", club)})
	}
	for w := 1; w <= 7; w++ {
		data.Gameweeks = append(data.Gameweeks, domain.FootballGameweek{Number: w, Deadline: now.Add(time.Duration(w-2)*7*24*time.Hour + 12*time.Hour)})
	}
	for w := 2; w <= 7; w++ {
		for club := 1; club <= 5; club += 2 {
			week := w
			kick := now.Add(time.Duration(w-1) * 7 * 24 * time.Hour)
			data.Fixtures = append(data.Fixtures, domain.FootballFixture{ID: w*10 + club, Home: club, Away: club + 1, Gameweek: &week, Kickoff: &kick, HomeDifficulty: 2, AwayDifficulty: 4})
		}
	}
	positions := []domain.Position{domain.Goalkeeper, domain.Goalkeeper, domain.Defender, domain.Defender, domain.Defender, domain.Defender, domain.Defender, domain.Midfielder, domain.Midfielder, domain.Midfielder, domain.Midfielder, domain.Midfielder, domain.Forward, domain.Forward, domain.Forward, domain.Defender}
	for i, pos := range positions {
		data.Players = append(data.Players, domain.FootballPlayer{ProviderID: i + 1, ClubID: i%6 + 1, FirstName: "Forecast", LastName: fmt.Sprint(i), Position: pos, Price: 50, Status: "a", FootballStats: domain.FootballStats{Minutes: &minutes, Starts: 10, XG: fmt.Sprint(i%3 + 1), XA: "1.2", Bonus: 4, DefensiveActions: 80}})
	}
	if err = s.SyncFootballData(ctx, data); err != nil {
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
	var players []domain.SquadPlayer
	for _, p := range catalogue {
		if p.Name != "Forecast 15" {
			players = append(players, p.SquadPlayer)
		}
	}
	starters, bench, captain, vice, err := domain.OptimizeLineup(players)
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(ctx, uuid.NewString()+"@example.test", "Advisor", "unused")
	if err != nil {
		t.Fatal(err)
	}
	squad, err := s.CreateSquad(ctx, user.ID, CreateSquadInput{SeasonID: season.ID, CurrentGameweek: 2, Players: players, Starters: starters, Bench: bench, CaptainID: captain, ViceCaptainID: vice, Bank: 250, FreeTransfers: 1})
	if err != nil {
		t.Fatal(err)
	}
	advice, err := s.Advice(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	prediction, err := s.Prediction(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prediction.SquadRevision != squad.Revision || prediction.Gameweek != squad.CurrentGameweek || prediction.Points != advice.SavedLineupPoints {
		t.Fatalf("saved-team prediction diverged from advice: %+v versus %.2f", prediction, advice.SavedLineupPoints)
	}
	if advice.SquadRevision != squad.Revision || len(advice.Players) != 16 || advice.Lineup.Points <= 0 || len(advice.Owned) != 15 {
		t.Fatal("incomplete personalized advice")
	}
	history, err := s.RecommendationHistory(ctx, user.ID)
	if err != nil || len(history) != 1 || history[0].SquadRevision != squad.Revision {
		t.Fatalf("immutable recommendation snapshot missing: %+v, %v", history, err)
	}
	if err = s.SetRecommendationOutcome(ctx, uuid.New(), history[0].ID, 42, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user outcome update allowed: %v", err)
	}
	if err = s.SetRecommendationOutcome(ctx, user.ID, history[0].ID, 42, "final score"); err != nil {
		t.Fatal(err)
	}
	if err = s.AddWatch(ctx, user.ID, catalogue[0].PlayerID); err != nil {
		t.Fatal(err)
	}
	watched, err := s.Watchlist(ctx, user.ID)
	if err != nil || len(watched) != 1 {
		t.Fatalf("watchlist not persisted: %+v, %v", watched, err)
	}
	if watchedOther, watchErr := s.Watchlist(ctx, uuid.New()); watchErr != nil || len(watchedOther) != 0 {
		t.Fatalf("watchlist tenant isolation failed: %+v, %v", watchedOther, watchErr)
	}
	plan, err := s.GenerateTransferPlan(ctx, user.ID, "Test route", "balanced", 3)
	if err != nil || len(plan.Plan.Weeks) == 0 || plan.SquadRevision != squad.Revision {
		t.Fatalf("saved transfer plan missing: %+v, %v", plan, err)
	}
	for _, c := range advice.Chips {
		if c.Decision != "unavailable" {
			t.Fatal("unreviewed chip history ignored")
		}
	}
	if _, err = s.Advice(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user squad access: %v", err)
	}
	week := 1
	if err = s.SetChipHistory(ctx, user.ID, squad.Revision, []PriorChipUsage{{ChipID: 1, Gameweek: &week}}); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Advice(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SquadRevision <= advice.SquadRevision {
		t.Fatal("advice revision was not recalculated")
	}
	for _, c := range updated.Chips {
		if c.ID == 1 && c.Decision != "unavailable" {
			t.Fatal("used chip recommended")
		}
	}
	var outgoing, incoming uuid.UUID
	for _, p := range players {
		if p.Position == domain.Defender {
			outgoing = p.PlayerID
			break
		}
	}
	for _, p := range catalogue {
		if p.Name == "Forecast 15" {
			incoming = p.PlayerID
		}
	}
	transferred, err := s.ConfirmTransfer(ctx, user.ID, ConfirmTransferInput{ExpectedRevision: updated.SquadRevision, PlayerOutID: outgoing, PlayerInID: incoming, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	afterTransfer, err := s.Advice(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterTransfer.SquadRevision != transferred.Revision {
		t.Fatal("advice did not follow transfer revision")
	}
	found := false
	for _, id := range afterTransfer.Owned {
		if id == outgoing {
			t.Fatal("advice still owns transferred-out player")
		}
		if id == incoming {
			found = true
		}
	}
	if !found {
		t.Fatal("incoming player missing from recalculated advice")
	}
	if _, err = pool.Exec(ctx, `UPDATE football_forecast_sources SET fetched_at=now()-interval '25 hours' WHERE season_id=$1`, season.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Advice(ctx, user.ID); !errors.Is(err, ErrAdviceUnavailable) {
		t.Fatalf("stale advice allowed: %v", err)
	}
}

func TestChipAdviceRespectsHistoryWindowsAndConsecutiveFreeHit(t *testing.T) {
	previous := 19
	tactics := Tactics{HistoryReviewed: true, Chips: []ChipOption{
		{ID: 1, Name: "bboost", Start: 1, End: 19},
		{ID: 2, Name: "3xc", Start: 20, End: 38, Used: true},
		{ID: 3, Name: "freehit", Start: 1, End: 19, Used: true, UsedGameweek: &previous},
		{ID: 4, Name: "freehit", Start: 20, End: 38},
	}}
	advice, err := recommendChips(domain.Squad{CurrentGameweek: 20}, tactics, domain.Forecast{Gameweek: 20}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, chip := range advice {
		if chip.Decision != "unavailable" {
			t.Fatalf("ineligible chip recommended: %+v", chip)
		}
	}
}
