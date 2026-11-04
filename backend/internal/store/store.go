package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrConflict         = errors.New("resource already exists")
	ErrRevisionConflict = errors.New("squad was updated in another session")
)

type Store struct{ pool *pgxpool.Pool }

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type Season struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	CurrentGameweek int       `json:"currentGameweek"`
}

type CreateSquadInput struct {
	SeasonID        uuid.UUID
	CurrentGameweek int
	Bank            int
	FreeTransfers   int
	Players         []domain.SquadPlayer
	Starters        []uuid.UUID
	Bench           []uuid.UUID
	CaptainID       uuid.UUID
	ViceCaptainID   uuid.UUID
}

type ConfirmTransferInput struct {
	ExpectedRevision int64
	PlayerOutID      uuid.UUID
	PlayerInID       uuid.UUID
	IdempotencyKey   string
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) CurrentSeason(ctx context.Context) (Season, error) {
	var season Season
	err := s.pool.QueryRow(ctx, `SELECT s.id,s.name,COALESCE((SELECT min(number) FROM gameweeks WHERE season_id=s.id AND deadline_at>now()),(SELECT number FROM gameweeks WHERE season_id=s.id AND is_current LIMIT 1),1) FROM seasons s WHERE s.is_current`).Scan(&season.ID, &season.Name, &season.CurrentGameweek)
	if errors.Is(err, pgx.ErrNoRows) {
		return Season{}, ErrNotFound
	}
	return season, err
}

// SyncFootballData atomically refreshes the current season catalogue from the
// provider. User squads continue to reference stable player UUIDs because all
// provider entities are upserted by their provider identifiers.
func (s *Store) SyncFootballData(ctx context.Context, data domain.FootballData) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `UPDATE seasons SET is_current=false WHERE is_current`); err != nil {
		return err
	}
	var seasonID uuid.UUID
	rules := `{"squadSize":15,"budget":1000,"maxPlayersPerClub":3,"freeTransferCap":5,"transferHitPoints":4,"positions":{"GKP":2,"DEF":5,"MID":5,"FWD":3}}`
	err = tx.QueryRow(ctx, `
		INSERT INTO seasons(name,starts_on,ends_on,is_current,rules)
		VALUES($1,$2,$3,true,$4)
		ON CONFLICT (name) DO UPDATE SET starts_on=EXCLUDED.starts_on,ends_on=EXCLUDED.ends_on,is_current=true,rules=EXCLUDED.rules
		RETURNING id`, data.SeasonName, data.StartsOn, data.EndsOn, rules).Scan(&seasonID)
	if err != nil {
		return err
	}

	clubIDs := make(map[int]uuid.UUID, len(data.Clubs))
	for _, chip := range data.Chips {
		switch chip.Name {
		case "wildcard", "freehit", "bboost", "3xc":
		default:
			continue
		}
		if chip.Start < 1 || chip.End < chip.Start {
			return errors.New("invalid provider chip window")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO season_chips(season_id,provider_id,name,start_gameweek,end_gameweek) VALUES($1,$2,$3,$4,$5) ON CONFLICT(season_id,provider_id) DO UPDATE SET name=EXCLUDED.name,start_gameweek=EXCLUDED.start_gameweek,end_gameweek=EXCLUDED.end_gameweek`, seasonID, chip.ID, chip.Name, chip.Start, chip.End); err != nil {
			return err
		}
	}
	for _, club := range data.Clubs {
		var clubID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO clubs(season_id,provider_id,name,short_name) VALUES($1,$2,$3,$4)
			ON CONFLICT (season_id,provider_id) DO UPDATE SET name=EXCLUDED.name,short_name=EXCLUDED.short_name
			RETURNING id`, seasonID, club.ProviderID, club.Name, club.ShortName).Scan(&clubID)
		if err != nil {
			return err
		}
		clubIDs[club.ProviderID] = clubID
	}

	for _, player := range data.Players {
		clubID, ok := clubIDs[player.ClubID]
		if !ok {
			return fmt.Errorf("player %d references unknown provider club %d", player.ProviderID, player.ClubID)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO players(season_id,club_id,provider_id,first_name,last_name,position,price,status,total_points,form,selected_by_percent)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (season_id,provider_id) DO UPDATE SET
				club_id=EXCLUDED.club_id,first_name=EXCLUDED.first_name,last_name=EXCLUDED.last_name,
				position=EXCLUDED.position,price=EXCLUDED.price,status=EXCLUDED.status,
				total_points=EXCLUDED.total_points,form=EXCLUDED.form,selected_by_percent=EXCLUDED.selected_by_percent,updated_at=now()`,
			seasonID, clubID, player.ProviderID, player.FirstName, player.LastName, player.Position,
			player.Price, player.Status, player.TotalPoints, player.Form, player.SelectedByPercent)
		if err != nil {
			return err
		}
	}

	if _, err = tx.Exec(ctx, `UPDATE gameweeks SET is_current=false WHERE season_id=$1`, seasonID); err != nil {
		return err
	}
	for _, gameweek := range data.Gameweeks {
		_, err = tx.Exec(ctx, `
			INSERT INTO gameweeks(season_id,number,deadline_at,is_current) VALUES($1,$2,$3,$4)
			ON CONFLICT (season_id,number) DO UPDATE SET deadline_at=EXCLUDED.deadline_at,is_current=EXCLUDED.is_current`,
			seasonID, gameweek.Number, gameweek.Deadline, gameweek.IsCurrent)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SeedDevelopmentData creates synthetic, clearly labelled data only for local development.
// Production football data must be populated by an authorized provider adapter.
func (s *Store) SeedDevelopmentData(ctx context.Context) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seasons WHERE is_current)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var seasonID uuid.UUID
	rules := `{"squadSize":15,"budget":1000,"maxPlayersPerClub":3,"freeTransferCap":5,"transferHitPoints":4,"positions":{"GKP":2,"DEF":5,"MID":5,"FWD":3}}`
	if err = tx.QueryRow(ctx, `INSERT INTO seasons(name,starts_on,ends_on,is_current,rules) VALUES('Development 2026/27','2026-08-01','2027-05-31',true,$1) RETURNING id`, rules).Scan(&seasonID); err != nil {
		return err
	}
	positions := []domain.Position{domain.Goalkeeper, domain.Defender, domain.Midfielder, domain.Forward}
	for clubNumber := 1; clubNumber <= 20; clubNumber++ {
		var clubID uuid.UUID
		if err = tx.QueryRow(ctx, `INSERT INTO clubs(season_id,provider_id,name,short_name) VALUES($1,$2,$3,$4) RETURNING id`, seasonID, clubNumber, fmt.Sprintf("Development Club %02d", clubNumber), fmt.Sprintf("D%02d", clubNumber)).Scan(&clubID); err != nil {
			return err
		}
		for round := 1; round <= 2; round++ {
			for positionIndex, position := range positions {
				providerID := clubNumber*100 + round*10 + positionIndex
				price := 45 + positionIndex*10 + round*2 + (clubNumber % 4)
				if _, err = tx.Exec(ctx, `INSERT INTO players(season_id,club_id,provider_id,first_name,last_name,position,price,total_points,form,expected_minutes) VALUES($1,$2,$3,'Demo',$4,$5,$6,$7,$8,90)`, seasonID, clubID, providerID, fmt.Sprintf("%s %02d-%d", position, clubNumber, round), position, price, 30+clubNumber+positionIndex*8, 3.5+float64(positionIndex)/2); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateUser(ctx context.Context, email, displayName, passwordHash string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user User
	err := s.pool.QueryRow(ctx, `INSERT INTO users(email, display_name, password_hash) VALUES($1,$2,$3) RETURNING id,email,display_name,password_hash`, email, displayName, passwordHash).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_unique_active") {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO user_preferences(user_id) VALUES($1)`, user.ID)
	return user, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id,email,display_name,password_hash FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL`, strings.TrimSpace(email)).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id,email,display_name,password_hash FROM users WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, sessionID, userID uuid.UUID, tokenHash []byte, expires time.Time, userAgent, ip string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO user_sessions(id,user_id,refresh_token_hash,expires_at,user_agent,ip_address) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::inet)`, sessionID, userID, tokenHash, expires, userAgent, ip)
	return err
}

func (s *Store) RotateSession(ctx context.Context, oldID, newID uuid.UUID, oldHash, newHash []byte, expires time.Time, userAgent, ip string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT user_id FROM user_sessions WHERE id=$1 AND refresh_token_hash=$2 AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, oldID, oldHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now(), replaced_by=$2 WHERE id=$1`, oldID, newID); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_sessions(id,user_id,refresh_token_hash,expires_at,user_agent,ip_address) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::inet)`, newID, userID, newHash, expires, userAgent, ip); err != nil {
		return uuid.Nil, err
	}
	return userID, tx.Commit(ctx)
}

func (s *Store) SessionActive(ctx context.Context, sessionID, userID uuid.UUID) bool {
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>now())`, sessionID, userID).Scan(&active)
	return err == nil && active
}

func (s *Store) RevokeSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1 AND user_id=$2`, sessionID, userID)
	return err
}

type CataloguePlayer struct {
	domain.SquadPlayer
	ClubFullName      string   `json:"clubFullName"`
	SelectedByPercent *float64 `json:"selectedByPercent"`
}

func (s *Store) ListPlayers(ctx context.Context, seasonID uuid.UUID, position string, maxPrice int) ([]CataloguePlayer, error) {
	query := `SELECT p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price,p.price,p.price,0,COALESCE(p.expected_minutes,0),c.name,p.selected_by_percent FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 AND ($2='' OR p.position=$2) AND ($3=0 OR p.price<=$3) ORDER BY p.total_points DESC,p.last_name,p.id`
	rows, err := s.pool.Query(ctx, query, seasonID, position, maxPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := make([]CataloguePlayer, 0)
	for rows.Next() {
		var p CataloguePlayer
		if err := rows.Scan(&p.PlayerID, &p.ClubID, &p.ClubName, &p.Name, &p.Position, &p.CurrentPrice, &p.PurchasePrice, &p.SellingPrice, &p.ExpectedPoints, &p.ExpectedMins, &p.ClubFullName, &p.SelectedByPercent); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func (s *Store) PlayersByIDs(ctx context.Context, seasonID uuid.UUID, ids []uuid.UUID) ([]domain.SquadPlayer, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price,p.price,p.price,0,COALESCE(p.expected_minutes,0) FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 AND p.id=ANY($2)`, seasonID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := make([]domain.SquadPlayer, 0, len(ids))
	for rows.Next() {
		var p domain.SquadPlayer
		if err := rows.Scan(&p.PlayerID, &p.ClubID, &p.ClubName, &p.Name, &p.Position, &p.CurrentPrice, &p.PurchasePrice, &p.SellingPrice, &p.ExpectedPoints, &p.ExpectedMins); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(players) != len(ids) {
		return nil, errors.New("one or more selected players are not eligible for this season")
	}
	return players, nil
}

func (s *Store) CreateSquad(ctx context.Context, userID uuid.UUID, input CreateSquadInput) (domain.Squad, error) {
	squad := domain.Squad{Revision: 1, Bank: input.Bank, FreeTransfers: input.FreeTransfers, CurrentGameweek: input.CurrentGameweek, Players: input.Players, Starters: input.Starters, Bench: input.Bench, CaptainID: input.CaptainID, ViceCaptainID: input.ViceCaptainID}
	if err := domain.ValidateSquad(input.Players, input.Bank, domain.DefaultRules()); err != nil {
		return domain.Squad{}, err
	}
	if err := domain.ValidateLineup(squad); err != nil {
		return domain.Squad{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Squad{}, err
	}
	defer tx.Rollback(ctx)
	teamValue := input.Bank
	for _, p := range input.Players {
		teamValue += p.CurrentPrice
	}
	err = tx.QueryRow(ctx, `INSERT INTO user_squads(user_id,season_id,current_gameweek,bank,team_value,free_transfers,captain_id,vice_captain_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, userID, input.SeasonID, input.CurrentGameweek, input.Bank, teamValue, input.FreeTransfers, input.CaptainID, input.ViceCaptainID).Scan(&squad.ID)
	if err != nil {
		return domain.Squad{}, err
	}
	benchOrder := make(map[uuid.UUID]int)
	for i, id := range input.Bench {
		benchOrder[id] = i + 1
	}
	starters := make(map[uuid.UUID]bool)
	for _, id := range input.Starters {
		starters[id] = true
	}
	for _, p := range input.Players {
		var order *int
		if value, ok := benchOrder[p.PlayerID]; ok {
			valueCopy := value
			order = &valueCopy
		}
		if _, err = tx.Exec(ctx, `INSERT INTO squad_players(squad_id,player_id,purchase_price,selling_price,is_starter,bench_order) VALUES($1,$2,$3,$4,$5,$6)`, squad.ID, p.PlayerID, p.PurchasePrice, p.SellingPrice, starters[p.PlayerID], order); err != nil {
			return domain.Squad{}, err
		}
	}
	if err = snapshotAndEvent(ctx, tx, userID, squad, "squad.created"); err != nil {
		return domain.Squad{}, err
	}
	return squad, tx.Commit(ctx)
}

func (s *Store) Squad(ctx context.Context, userID uuid.UUID) (domain.Squad, error) {
	var squad domain.Squad
	err := s.pool.QueryRow(ctx, `SELECT id,revision,bank,free_transfers,current_gameweek,captain_id,vice_captain_id,season_id FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1`, userID).Scan(&squad.ID, &squad.Revision, &squad.Bank, &squad.FreeTransfers, &squad.CurrentGameweek, &squad.CaptainID, &squad.ViceCaptainID, &squad.SeasonID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, ErrNotFound
	}
	if err != nil {
		return domain.Squad{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price,sp.purchase_price,sp.selling_price,0,COALESCE(p.expected_minutes,0),sp.is_starter,sp.bench_order FROM squad_players sp JOIN players p ON p.id=sp.player_id JOIN clubs c ON c.id=p.club_id WHERE sp.squad_id=$1 ORDER BY sp.is_starter DESC,sp.bench_order NULLS FIRST`, squad.ID)
	if err != nil {
		return domain.Squad{}, err
	}
	defer rows.Close()
	type row struct {
		p       domain.SquadPlayer
		starter bool
		order   *int
	}
	bench := make([]row, 0, 4)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.p.PlayerID, &r.p.ClubID, &r.p.ClubName, &r.p.Name, &r.p.Position, &r.p.CurrentPrice, &r.p.PurchasePrice, &r.p.SellingPrice, &r.p.ExpectedPoints, &r.p.ExpectedMins, &r.starter, &r.order); err != nil {
			return domain.Squad{}, err
		}
		squad.Players = append(squad.Players, r.p)
		if r.starter {
			squad.Starters = append(squad.Starters, r.p.PlayerID)
		} else {
			bench = append(bench, r)
		}
	}
	for order := 1; order <= 4; order++ {
		for _, r := range bench {
			if r.order != nil && *r.order == order {
				squad.Bench = append(squad.Bench, r.p.PlayerID)
			}
		}
	}
	return squad, rows.Err()
}

func (s *Store) ConfirmTransfer(ctx context.Context, userID uuid.UUID, input ConfirmTransferInput) (domain.Squad, error) {
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return domain.Squad{}, errors.New("idempotency key is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Squad{}, err
	}
	defer tx.Rollback(ctx)
	var squadID, seasonID uuid.UUID
	var revision int64
	var bank, freeTransfers, gw int
	err = tx.QueryRow(ctx, `SELECT id,season_id,revision,bank,free_transfers,current_gameweek FROM user_squads WHERE user_id=$1 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&squadID, &seasonID, &revision, &bank, &freeTransfers, &gw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, ErrNotFound
	}
	if err != nil {
		return domain.Squad{}, err
	}
	if revision != input.ExpectedRevision {
		return domain.Squad{}, ErrRevisionConflict
	}
	var existingRevision int64
	err = tx.QueryRow(ctx, `SELECT squad_revision FROM user_transfer_ledger WHERE squad_id=$1 AND idempotency_key=$2`, squadID, input.IdempotencyKey).Scan(&existingRevision)
	if err == nil {
		return domain.Squad{}, ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, err
	}
	var sellingPrice int
	var purchasePrice, currentPrice int
	err = tx.QueryRow(ctx, `SELECT sp.purchase_price,p.price FROM squad_players sp JOIN players p ON p.id=sp.player_id WHERE squad_id=$1 AND player_id=$2`, squadID, input.PlayerOutID).Scan(&purchasePrice, &currentPrice)
	sellingPrice = domain.SellingPrice(purchasePrice, currentPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, errors.New("player out is not owned")
	}
	if err != nil {
		return domain.Squad{}, err
	}
	var incoming domain.SquadPlayer
	err = tx.QueryRow(ctx, `SELECT p.id,p.club_id,c.short_name,p.first_name||' '||p.last_name,p.position,p.price,p.price,p.price,0,COALESCE(p.expected_minutes,0) FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.id=$1 AND p.season_id=$2`, input.PlayerInID, seasonID).Scan(&incoming.PlayerID, &incoming.ClubID, &incoming.ClubName, &incoming.Name, &incoming.Position, &incoming.CurrentPrice, &incoming.PurchasePrice, &incoming.SellingPrice, &incoming.ExpectedPoints, &incoming.ExpectedMins)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, errors.New("incoming player is not eligible")
	}
	if err != nil {
		return domain.Squad{}, err
	}
	var alreadyOwned bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM squad_players WHERE squad_id=$1 AND player_id=$2)`, squadID, input.PlayerInID).Scan(&alreadyOwned); err != nil {
		return domain.Squad{}, err
	}
	if alreadyOwned {
		return domain.Squad{}, errors.New("incoming player is already owned")
	}
	available := bank + sellingPrice
	if incoming.CurrentPrice > available {
		return domain.Squad{}, errors.New("insufficient transfer funds")
	}
	var oldPosition domain.Position
	var wasStarter bool
	var benchOrder *int
	if err = tx.QueryRow(ctx, `SELECT p.position,sp.is_starter,sp.bench_order FROM squad_players sp JOIN players p ON p.id=sp.player_id WHERE sp.squad_id=$1 AND sp.player_id=$2`, squadID, input.PlayerOutID).Scan(&oldPosition, &wasStarter, &benchOrder); err != nil {
		return domain.Squad{}, err
	}
	if incoming.Position != oldPosition {
		return domain.Squad{}, errors.New("replacement must have the same position")
	}
	var clubCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM squad_players sp JOIN players p ON p.id=sp.player_id WHERE sp.squad_id=$1 AND p.club_id=$2 AND sp.player_id<>$3`, squadID, incoming.ClubID, input.PlayerOutID).Scan(&clubCount); err != nil {
		return domain.Squad{}, err
	}
	if clubCount >= domain.DefaultRules().MaxPlayersPerClub {
		return domain.Squad{}, errors.New("transfer exceeds club player limit")
	}
	newBank := available - incoming.CurrentPrice
	newRevision := revision + 1
	pointsHit := domain.TransferHit(freeTransfers, 1, domain.DefaultRules())
	var transferChip bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM squad_chips WHERE squad_id=$1 AND gameweek=$2 AND name IN ('wildcard','freehit'))`, squadID, gw).Scan(&transferChip); err != nil {
		return domain.Squad{}, err
	}
	newFreeTransfers := freeTransfers - 1
	if newFreeTransfers < 0 {
		newFreeTransfers = 0
	}
	if transferChip {
		pointsHit = 0
		newFreeTransfers = freeTransfers
	}
	if _, err = tx.Exec(ctx, `DELETE FROM squad_players WHERE squad_id=$1 AND player_id=$2`, squadID, input.PlayerOutID); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_players(squad_id,player_id,purchase_price,selling_price,is_starter,bench_order) VALUES($1,$2,$3,$3,$4,$5)`, squadID, input.PlayerInID, incoming.CurrentPrice, wasStarter, benchOrder); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_squads SET bank=$2,free_transfers=$3,revision=$4,updated_at=now(),captain_id=CASE WHEN captain_id=$5 THEN $6 ELSE captain_id END,vice_captain_id=CASE WHEN vice_captain_id=$5 THEN $6 ELSE vice_captain_id END WHERE id=$1`, squadID, newBank, newFreeTransfers, newRevision, input.PlayerOutID, input.PlayerInID); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_transfer_ledger(squad_id,player_out_id,player_in_id,selling_price,purchase_price,bank_before,bank_after,points_hit,idempotency_key,squad_revision,gameweek) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, squadID, input.PlayerOutID, input.PlayerInID, sellingPrice, incoming.CurrentPrice, bank, newBank, pointsHit, input.IdempotencyKey, newRevision, gw); err != nil {
		return domain.Squad{}, err
	}
	payload, _ := json.Marshal(map[string]any{"revision": newRevision, "playerOutId": input.PlayerOutID, "playerInId": input.PlayerInID})
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'transfer.confirmed',$4)`, squadID, userID, newRevision, payload); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(aggregate_type,aggregate_id,event_type,user_id,payload) VALUES('squad',$1,'squad.updated',$2,$3)`, squadID, userID, payload); err != nil {
		return domain.Squad{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Squad{}, err
	}
	return s.Squad(ctx, userID)
}

func snapshotAndEvent(ctx context.Context, tx pgx.Tx, userID uuid.UUID, squad domain.Squad, eventType string) error {
	state, err := json.Marshal(squad)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_snapshots(squad_id,revision,reason,state) VALUES($1,$2,$3,$4)`, squad.ID, squad.Revision, eventType, state); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,$4,$5)`, squad.ID, userID, squad.Revision, eventType, state); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(aggregate_type,aggregate_id,event_type,user_id,payload) VALUES('squad',$1,$2,$3,$4)`, squad.ID, eventType, userID, state); err != nil {
		return fmt.Errorf("write outbox event: %w", err)
	}
	return nil
}
