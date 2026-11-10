package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

var (
	ErrOfficialImportRateLimit = errors.New("official squad import limit reached; try again in a few minutes")
	ErrOfficialImportExpired   = errors.New("official squad preview expired; fetch a fresh preview")
)

const officialImportPreviewTTL = 30 * time.Minute

type OfficialImportPlayer struct {
	PlayerID         uuid.UUID       `json:"playerId"`
	ClubID           uuid.UUID       `json:"-"`
	ProviderPlayerID int             `json:"providerPlayerId"`
	Name             string          `json:"name"`
	ClubName         string          `json:"clubName"`
	KitCode          int             `json:"kitCode,omitempty"`
	SquadNumber      *int            `json:"squadNumber"`
	Position         domain.Position `json:"position"`
	CurrentPrice     int             `json:"currentPrice"`
	LineupPosition   int             `json:"lineupPosition"`
	Captain          bool            `json:"captain"`
	ViceCaptain      bool            `json:"viceCaptain"`
}

type OfficialImportDifference struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Internal string `json:"internal,omitempty"`
	Official string `json:"official"`
}

type OfficialImportPreview struct {
	ID                        uuid.UUID                  `json:"id"`
	Official                  domain.OfficialSquad       `json:"official"`
	Players                   []OfficialImportPlayer     `json:"players"`
	Differences               []OfficialImportDifference `json:"differences"`
	ExpectedSquadRevision     int64                      `json:"expectedSquadRevision"`
	PurchasePricesUnavailable bool                       `json:"purchasePricesUnavailable"`
	ExpiresAt                 time.Time                  `json:"expiresAt"`
}

type OfficialSquadLink struct {
	ManagerID        int64                `json:"managerId"`
	LastReconciledAt time.Time            `json:"lastReconciledAt"`
	Official         domain.OfficialSquad `json:"official"`
}

func (s *Store) OfficialSquadLink(ctx context.Context, userID uuid.UUID) (OfficialSquadLink, error) {
	var result OfficialSquadLink
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT l.manager_id,l.last_reconciled_at,i.payload FROM official_squad_links l JOIN official_squad_imports i ON i.id=l.latest_import_id WHERE l.user_id=$1`, userID).Scan(&result.ManagerID, &result.LastReconciledAt, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(payload, &result.Official); err != nil {
		return result, err
	}
	return result, nil
}

// StartOfficialImport records every provider attempt and enforces a durable,
// per-user limit before any outbound request is made.
func (s *Store) StartOfficialImport(ctx context.Context, userID uuid.UUID, managerID int64) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, userID.String()); err != nil {
		return uuid.Nil, err
	}
	var recent int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM official_squad_imports WHERE user_id=$1 AND created_at>now()-interval '5 minutes'`, userID).Scan(&recent); err != nil {
		return uuid.Nil, err
	}
	if recent >= 3 {
		return uuid.Nil, ErrOfficialImportRateLimit
	}
	var id uuid.UUID
	if err = tx.QueryRow(ctx, `INSERT INTO official_squad_imports(user_id,manager_id,status) VALUES($1,$2,'fetching') RETURNING id`, userID, managerID).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) FailOfficialImport(ctx context.Context, userID, importID uuid.UUID, code string) {
	auditContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, _ = s.pool.Exec(auditContext, `UPDATE official_squad_imports SET status='failed',failure_code=$3 WHERE id=$1 AND user_id=$2 AND status='fetching'`, importID, userID, code)
}

func (s *Store) SaveOfficialImportPreview(ctx context.Context, userID, importID uuid.UUID, official domain.OfficialSquad) (OfficialImportPreview, error) {
	season, err := s.CurrentSeason(ctx)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	players, err := s.officialImportPlayers(ctx, season.ID, official.Picks)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	if err = validateOfficialPlayers(players); err != nil {
		return OfficialImportPreview{}, err
	}
	differences, revision, squadID, err := s.officialImportDifferences(ctx, userID, season.ID, official, players)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	payload, err := json.Marshal(official)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	differencePayload, err := json.Marshal(differences)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE official_squad_imports SET squad_id=$4,season_id=$5,provider_gameweek=$6,status='preview',expected_squad_revision=$7,payload=$8,differences=$9,fetched_at=$10 WHERE id=$1 AND user_id=$2 AND manager_id=$3 AND status='fetching'`, importID, userID, official.ManagerID, squadID, season.ID, official.Gameweek, revision, payload, differencePayload, official.FetchedAt)
	if err != nil {
		return OfficialImportPreview{}, err
	}
	if tag.RowsAffected() != 1 {
		return OfficialImportPreview{}, ErrNotFound
	}
	return OfficialImportPreview{ID: importID, Official: official, Players: players, Differences: differences, ExpectedSquadRevision: revision, PurchasePricesUnavailable: true, ExpiresAt: time.Now().UTC().Add(officialImportPreviewTTL)}, nil
}

func (s *Store) ApplyOfficialImport(ctx context.Context, userID, importID uuid.UUID, expectedRevision int64) (domain.Squad, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Squad{}, err
	}
	defer tx.Rollback(ctx)
	var official domain.OfficialSquad
	var raw []byte
	var managerID int64
	var seasonID uuid.UUID
	var savedRevision int64
	var createdAt time.Time
	err = tx.QueryRow(ctx, `SELECT manager_id,season_id,expected_squad_revision,payload,created_at FROM official_squad_imports WHERE id=$1 AND user_id=$2 AND status='preview' FOR UPDATE`, importID, userID).Scan(&managerID, &seasonID, &savedRevision, &raw, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Squad{}, ErrNotFound
	}
	if err != nil {
		return domain.Squad{}, err
	}
	if time.Since(createdAt) > officialImportPreviewTTL {
		return domain.Squad{}, ErrOfficialImportExpired
	}
	if expectedRevision != savedRevision {
		return domain.Squad{}, ErrRevisionConflict
	}
	if err = json.Unmarshal(raw, &official); err != nil {
		return domain.Squad{}, err
	}
	players, err := officialImportPlayersTx(ctx, tx, seasonID, official.Picks)
	if err != nil {
		return domain.Squad{}, err
	}
	if err = validateOfficialPlayers(players); err != nil {
		return domain.Squad{}, err
	}

	var squadID uuid.UUID
	var currentRevision int64
	var currentGameweek int
	err = tx.QueryRow(ctx, `SELECT id,revision FROM user_squads WHERE user_id=$1 AND season_id=$2 FOR UPDATE`, userID, seasonID).Scan(&squadID, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		if savedRevision != 0 {
			return domain.Squad{}, ErrRevisionConflict
		}
		currentRevision = 1
		captain, vice := officialCaptaincy(players)
		err = tx.QueryRow(ctx, `INSERT INTO user_squads(user_id,season_id,current_gameweek,bank,team_value,free_transfers,captain_id,vice_captain_id,chip_history_reviewed) VALUES($1,$2,(SELECT COALESCE(min(number) FILTER (WHERE deadline_at>now()),max(number)) FROM gameweeks WHERE season_id=$2),$3,$4,$5,$6,$7,true) RETURNING id,current_gameweek`, userID, seasonID, official.Bank, official.TeamValue, official.FreeTransfers, captain, vice).Scan(&squadID, &currentGameweek)
	} else if err == nil {
		if currentRevision != savedRevision {
			return domain.Squad{}, ErrRevisionConflict
		}
		currentRevision++
		captain, vice := officialCaptaincy(players)
		err = tx.QueryRow(ctx, `UPDATE user_squads SET current_gameweek=(SELECT COALESCE(min(number) FILTER (WHERE deadline_at>now()),max(number)) FROM gameweeks WHERE season_id=$2),bank=$3,team_value=$4,free_transfers=$5,captain_id=$6,vice_captain_id=$7,chip_history_reviewed=true,revision=$8,updated_at=now() WHERE id=$1 RETURNING current_gameweek`, squadID, seasonID, official.Bank, official.TeamValue, official.FreeTransfers, captain, vice, currentRevision).Scan(&currentGameweek)
	}
	if err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM squad_players WHERE squad_id=$1`, squadID); err != nil {
		return domain.Squad{}, err
	}
	for _, player := range players {
		starter := player.LineupPosition <= 11
		var benchOrder *int
		if !starter {
			value := player.LineupPosition - 11
			benchOrder = &value
		}
		if _, err = tx.Exec(ctx, `INSERT INTO squad_players(squad_id,player_id,purchase_price,selling_price,is_starter,bench_order) VALUES($1,$2,$3,$3,$4,$5)`, squadID, player.PlayerID, player.CurrentPrice, starter, benchOrder); err != nil {
			return domain.Squad{}, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM prior_chip_usage WHERE squad_id=$1`, squadID); err != nil {
		return domain.Squad{}, err
	}
	for _, chip := range official.Chips {
		if _, err = tx.Exec(ctx, `INSERT INTO prior_chip_usage(squad_id,provider_id,gameweek)
			SELECT $1,provider_id,$3 FROM season_chips WHERE season_id=$2 AND name=$4 AND $3 BETWEEN start_gameweek AND end_gameweek
			ON CONFLICT DO NOTHING`, squadID, seasonID, chip.Gameweek, chip.Name); err != nil {
			return domain.Squad{}, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM squad_chips WHERE squad_id=$1 AND gameweek<=$2`, squadID, official.Gameweek); err != nil {
		return domain.Squad{}, err
	}

	squad := buildImportedSquad(squadID, seasonID, currentRevision, currentGameweek, official, players)
	state, err := json.Marshal(squad)
	if err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_snapshots(squad_id,revision,reason,state) VALUES($1,$2,'official_squad.imported',$3)`, squadID, currentRevision, state); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO squad_events(squad_id,user_id,revision,event_type,payload) VALUES($1,$2,$3,'official_squad.imported',jsonb_build_object('importId',$4::text,'managerId',$5::bigint,'providerGameweek',$6::integer,'freeTransfersEstimated',true))`, squadID, userID, currentRevision, importID.String(), managerID, official.Gameweek); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE official_squad_imports SET squad_id=$3,status='applied',applied_at=now() WHERE id=$1 AND user_id=$2`, importID, userID, squadID); err != nil {
		return domain.Squad{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO official_squad_links(user_id,manager_id,latest_import_id,last_reconciled_at) VALUES($1,$2,$3,now()) ON CONFLICT(user_id) DO UPDATE SET manager_id=EXCLUDED.manager_id,latest_import_id=EXCLUDED.latest_import_id,last_reconciled_at=EXCLUDED.last_reconciled_at,updated_at=now()`, userID, managerID, importID); err != nil {
		return domain.Squad{}, err
	}
	return squad, tx.Commit(ctx)
}

func (s *Store) officialImportPlayers(ctx context.Context, seasonID uuid.UUID, picks []domain.OfficialPick) ([]OfficialImportPlayer, error) {
	return officialImportPlayersQuery(ctx, s.pool, seasonID, picks)
}

type rowQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func officialImportPlayersTx(ctx context.Context, tx pgx.Tx, seasonID uuid.UUID, picks []domain.OfficialPick) ([]OfficialImportPlayer, error) {
	return officialImportPlayersQuery(ctx, tx, seasonID, picks)
}

func officialImportPlayersQuery(ctx context.Context, query rowQuerier, seasonID uuid.UUID, picks []domain.OfficialPick) ([]OfficialImportPlayer, error) {
	providerIDs := make([]int, 0, len(picks))
	pickByProvider := make(map[int]domain.OfficialPick, len(picks))
	for _, pick := range picks {
		providerIDs = append(providerIDs, pick.ProviderPlayerID)
		pickByProvider[pick.ProviderPlayerID] = pick
	}
	rows, err := query.Query(ctx, `SELECT p.id,p.club_id,p.provider_id,p.first_name||' '||p.last_name,c.short_name,COALESCE(c.kit_code,0),p.squad_number,p.position,p.price FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 AND p.provider_id=ANY($2)`, seasonID, providerIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := make([]OfficialImportPlayer, 0, len(picks))
	for rows.Next() {
		var player OfficialImportPlayer
		if err = rows.Scan(&player.PlayerID, &player.ClubID, &player.ProviderPlayerID, &player.Name, &player.ClubName, &player.KitCode, &player.SquadNumber, &player.Position, &player.CurrentPrice); err != nil {
			return nil, err
		}
		pick := pickByProvider[player.ProviderPlayerID]
		player.LineupPosition, player.Captain, player.ViceCaptain = pick.Position, pick.Captain, pick.ViceCaptain
		players = append(players, player)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(players) != len(picks) {
		return nil, fmt.Errorf("official squad includes players missing from the current catalogue; refresh football data and try again")
	}
	sort.Slice(players, func(i, j int) bool { return players[i].LineupPosition < players[j].LineupPosition })
	return players, nil
}

func validateOfficialPlayers(players []OfficialImportPlayer) error {
	if len(players) != 15 {
		return errors.New("official squad must contain 15 mapped players")
	}
	positions := map[domain.Position]int{}
	clubs := map[uuid.UUID]int{}
	captains, viceCaptains := 0, 0
	for _, player := range players {
		positions[player.Position]++
		clubs[player.ClubID]++
		if clubs[player.ClubID] > domain.DefaultRules().MaxPlayersPerClub {
			return errors.New("official squad exceeds the three-player club limit")
		}
		if player.Captain {
			captains++
		}
		if player.ViceCaptain {
			viceCaptains++
		}
		if (player.Captain || player.ViceCaptain) && player.LineupPosition > 11 {
			return errors.New("official captaincy is not assigned to the starting XI")
		}
	}
	for position, required := range domain.DefaultRules().PositionAllocation {
		if positions[position] != required {
			return fmt.Errorf("official squad has an invalid %s allocation", position)
		}
	}
	if captains != 1 || viceCaptains != 1 {
		return errors.New("official squad must have one captain and one vice-captain")
	}
	validation := domain.Squad{}
	for _, player := range players {
		validation.Players = append(validation.Players, domain.SquadPlayer{PlayerID: player.PlayerID, Position: player.Position})
		if player.LineupPosition <= 11 {
			validation.Starters = append(validation.Starters, player.PlayerID)
		} else {
			validation.Bench = append(validation.Bench, player.PlayerID)
		}
		if player.Captain {
			validation.CaptainID = player.PlayerID
		}
		if player.ViceCaptain {
			validation.ViceCaptainID = player.PlayerID
		}
	}
	return domain.ValidateLineup(validation)
}

func (s *Store) officialImportDifferences(ctx context.Context, userID, seasonID uuid.UUID, official domain.OfficialSquad, players []OfficialImportPlayer) ([]OfficialImportDifference, int64, *uuid.UUID, error) {
	var squadID uuid.UUID
	var revision int64
	var bank, teamValue, free int
	var captainName, viceName string
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.revision,u.bank,u.team_value,u.free_transfers,COALESCE(cp.first_name||' '||cp.last_name,''),COALESCE(vp.first_name||' '||vp.last_name,'') FROM user_squads u LEFT JOIN players cp ON cp.id=u.captain_id LEFT JOIN players vp ON vp.id=u.vice_captain_id WHERE u.user_id=$1 AND u.season_id=$2`, userID, seasonID).Scan(&squadID, &revision, &bank, &teamValue, &free, &captainName, &viceName)
	if errors.Is(err, pgx.ErrNoRows) {
		return []OfficialImportDifference{{Kind: "new_squad", Label: "Create internal squad", Official: "Import all 15 official players"}}, 0, nil, nil
	}
	if err != nil {
		return nil, 0, nil, err
	}
	differences := make([]OfficialImportDifference, 0)
	if bank != official.Bank {
		differences = append(differences, OfficialImportDifference{Kind: "bank", Label: "Bank", Internal: money(bank), Official: money(official.Bank)})
	}
	if teamValue != official.TeamValue {
		differences = append(differences, OfficialImportDifference{Kind: "team_value", Label: "Team value", Internal: money(teamValue), Official: money(official.TeamValue)})
	}
	if free != official.FreeTransfers {
		differences = append(differences, OfficialImportDifference{Kind: "free_transfers", Label: "Free transfers (estimated)", Internal: fmt.Sprint(free), Official: fmt.Sprint(official.FreeTransfers)})
	}
	officialIDs := make(map[int]OfficialImportPlayer, len(players))
	for _, player := range players {
		officialIDs[player.ProviderPlayerID] = player
	}
	rows, err := s.pool.Query(ctx, `SELECT p.provider_id,p.first_name||' '||p.last_name FROM squad_players sp JOIN players p ON p.id=sp.player_id WHERE sp.squad_id=$1`, squadID)
	if err != nil {
		return nil, 0, nil, err
	}
	internalIDs := map[int]string{}
	for rows.Next() {
		var id int
		var name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, 0, nil, err
		}
		internalIDs[id] = name
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, nil, err
	}
	for id, name := range internalIDs {
		if _, ok := officialIDs[id]; !ok {
			differences = append(differences, OfficialImportDifference{Kind: "player_removed", Label: "Player out", Internal: name, Official: "Not in official squad"})
		}
	}
	for id, player := range officialIDs {
		if _, ok := internalIDs[id]; !ok {
			differences = append(differences, OfficialImportDifference{Kind: "player_added", Label: "Player in", Internal: "Not in FPL Edge squad", Official: player.Name})
		}
	}
	for _, player := range players {
		if player.Captain && captainName != player.Name {
			differences = append(differences, OfficialImportDifference{Kind: "captain", Label: "Captain", Internal: captainName, Official: player.Name})
		}
		if player.ViceCaptain && viceName != player.Name {
			differences = append(differences, OfficialImportDifference{Kind: "vice_captain", Label: "Vice-captain", Internal: viceName, Official: player.Name})
		}
	}
	if len(differences) == 0 {
		differences = append(differences, OfficialImportDifference{Kind: "no_change", Label: "Squad", Official: "Already matches the public official selection"})
	}
	return differences, revision, &squadID, nil
}

func officialCaptaincy(players []OfficialImportPlayer) (uuid.UUID, uuid.UUID) {
	var captain, vice uuid.UUID
	for _, player := range players {
		if player.Captain {
			captain = player.PlayerID
		}
		if player.ViceCaptain {
			vice = player.PlayerID
		}
	}
	return captain, vice
}

func buildImportedSquad(squadID, seasonID uuid.UUID, revision int64, currentGameweek int, official domain.OfficialSquad, players []OfficialImportPlayer) domain.Squad {
	result := domain.Squad{ID: squadID, SeasonID: seasonID, Revision: revision, CurrentGameweek: currentGameweek, Bank: official.Bank, FreeTransfers: official.FreeTransfers}
	for _, player := range players {
		mapped := domain.SquadPlayer{PlayerID: player.PlayerID, ClubID: player.ClubID, ClubName: player.ClubName, KitCode: player.KitCode, SquadNumber: player.SquadNumber, Name: player.Name, Position: player.Position, CurrentPrice: player.CurrentPrice, PurchasePrice: player.CurrentPrice, SellingPrice: player.CurrentPrice}
		result.Players = append(result.Players, mapped)
		if player.LineupPosition <= 11 {
			result.Starters = append(result.Starters, player.PlayerID)
		} else {
			result.Bench = append(result.Bench, player.PlayerID)
		}
		if player.Captain {
			result.CaptainID = player.PlayerID
		}
		if player.ViceCaptain {
			result.ViceCaptainID = player.PlayerID
		}
	}
	return result
}

func money(value int) string { return fmt.Sprintf("£%.1fm", float64(value)/10) }
