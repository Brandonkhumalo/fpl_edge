package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"github.com/tishanyq/fpl-edge/backend/migrations"
)

func TestOfficialImportPreviewAndApply(t *testing.T) {
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
	schema := "official_import_" + uuid.NewString()[:8]
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()
	config := pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	if err = migrations.Up(ctx, isolated); err != nil {
		t.Fatal(err)
	}
	s := New(isolated)
	now := time.Now().UTC()
	data := domain.FootballData{SeasonName: "Import " + uuid.NewString(), StartsOn: now.AddDate(0, -1, 0), EndsOn: now.AddDate(0, 6, 0), Chips: []domain.FootballChip{{ID: 1, Name: "bboost", Start: 1, End: 38}}}
	for club := 1; club <= 5; club++ {
		data.Clubs = append(data.Clubs, domain.FootballClub{ProviderID: club, Name: fmt.Sprintf("Club %d", club), ShortName: fmt.Sprintf("C%d", club)})
	}
	for gameweek := 1; gameweek <= 3; gameweek++ {
		data.Gameweeks = append(data.Gameweeks, domain.FootballGameweek{Number: gameweek, Deadline: now.Add(time.Duration(gameweek-2)*7*24*time.Hour + time.Hour)})
	}
	positions := []domain.Position{domain.Goalkeeper, domain.Defender, domain.Defender, domain.Defender, domain.Defender, domain.Defender, domain.Midfielder, domain.Midfielder, domain.Midfielder, domain.Midfielder, domain.Forward, domain.Goalkeeper, domain.Midfielder, domain.Forward, domain.Forward}
	for index, position := range positions {
		data.Players = append(data.Players, domain.FootballPlayer{ProviderID: index + 1, ClubID: index%5 + 1, FirstName: "Official", LastName: fmt.Sprint(index + 1), Position: position, Price: 50 + index, Status: "a"})
	}
	if err = s.SyncFootballData(ctx, data); err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(ctx, uuid.NewString()+"@example.test", "Importer", "unused")
	if err != nil {
		t.Fatal(err)
	}
	official := domain.OfficialSquad{ManagerID: 42, ManagerName: "Test Manager", TeamName: "Imported XI", Gameweek: 1, Bank: 12, TeamValue: 1012, FreeTransfers: 2, FreeTransfersEstimated: true, FetchedAt: now, Chips: []domain.OfficialChipUsage{{Name: "bboost", Gameweek: 1}}}
	for index := range positions {
		official.Picks = append(official.Picks, domain.OfficialPick{ProviderPlayerID: index + 1, Position: index + 1, Captain: index == 9, ViceCaptain: index == 10})
	}
	importID, err := s.StartOfficialImport(ctx, user.ID, official.ManagerID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.SaveOfficialImportPreview(ctx, user.ID, importID, official)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ExpectedSquadRevision != 0 || len(preview.Players) != 15 || preview.Differences[0].Kind != "new_squad" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	squad, err := s.ApplyOfficialImport(ctx, user.ID, importID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if squad.Bank != 12 || squad.FreeTransfers != 2 || squad.CurrentGameweek != 2 || len(squad.Players) != 15 || len(squad.Starters) != 11 || len(squad.Bench) != 4 {
		t.Fatalf("unexpected imported squad: %+v", squad)
	}
	var eventCount, chipCount, linkCount int
	if err = isolated.QueryRow(ctx, `SELECT count(*) FROM squad_events WHERE squad_id=$1 AND event_type='official_squad.imported'`, squad.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err = isolated.QueryRow(ctx, `SELECT count(*) FROM prior_chip_usage WHERE squad_id=$1`, squad.ID).Scan(&chipCount); err != nil {
		t.Fatal(err)
	}
	if err = isolated.QueryRow(ctx, `SELECT count(*) FROM official_squad_links WHERE user_id=$1 AND manager_id=42`, user.ID).Scan(&linkCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || chipCount != 1 || linkCount != 1 {
		t.Fatalf("missing audit state: events=%d chips=%d links=%d", eventCount, chipCount, linkCount)
	}
	if _, err = s.ApplyOfficialImport(ctx, user.ID, importID, 0); err != ErrNotFound {
		t.Fatalf("applied preview was reusable: %v", err)
	}
	staleImportID, err := s.StartOfficialImport(ctx, user.ID, 42)
	if err != nil {
		t.Fatal(err)
	}
	stalePreview, err := s.SaveOfficialImportPreview(ctx, user.ID, staleImportID, official)
	if err != nil || stalePreview.ExpectedSquadRevision != squad.Revision {
		t.Fatalf("unexpected reconciliation preview: %+v %v", stalePreview, err)
	}
	if err = s.SetCaptaincy(ctx, user.ID, squad.Revision, squad.ViceCaptainID, squad.CaptainID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyOfficialImport(ctx, user.ID, staleImportID, stalePreview.ExpectedSquadRevision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale preview overwrote a newer squad: %v", err)
	}
	if _, err = s.StartOfficialImport(ctx, user.ID, 43); err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartOfficialImport(ctx, user.ID, 44); !errors.Is(err, ErrOfficialImportRateLimit) {
		t.Fatalf("expected durable import rate limit, got %v", err)
	}
}
