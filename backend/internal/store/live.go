package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

const liveSnapshotFreshness = 90 * time.Second

type LivePlayerResult struct {
	PlayerID              uuid.UUID       `json:"playerId"`
	Name                  string          `json:"name"`
	Club                  string          `json:"club"`
	Position              domain.Position `json:"position"`
	IsStarter             bool            `json:"isStarter"`
	IsCaptain             bool            `json:"isCaptain"`
	IsViceCaptain         bool            `json:"isViceCaptain"`
	Multiplier            int             `json:"multiplier"`
	Points                int             `json:"points"`
	CountedPoints         int             `json:"countedPoints"`
	Minutes               int             `json:"minutes"`
	Bonus                 int             `json:"bonus"`
	BPS                   int             `json:"bps"`
	DefensiveContribution int             `json:"defensiveContribution"`
	DefensiveTarget       int             `json:"defensiveTarget"`
	Remaining             bool            `json:"remaining"`
	EstimatedAutoSub      bool            `json:"estimatedAutoSub"`
}

type LiveGameweekResult struct {
	Gameweek           int                `json:"gameweek"`
	Status             string             `json:"status"`
	FetchedAt          time.Time          `json:"fetchedAt"`
	Stale              bool               `json:"stale"`
	SquadRevision      int64              `json:"squadRevision"`
	ActiveChip         string             `json:"activeChip"`
	Points             int                `json:"points"`
	CaptainPoints      int                `json:"captainPoints"`
	CaptainExtra       int                `json:"captainExtra"`
	BenchPoints        int                `json:"benchPoints"`
	EstimatedBonus     int                `json:"estimatedBonus"`
	RemainingPlayers   int                `json:"remainingPlayers"`
	RemainingFixtures  int                `json:"remainingFixtures"`
	EstimatedAutoSubs  []uuid.UUID        `json:"estimatedAutoSubs"`
	Players            []LivePlayerResult `json:"players"`
	MiniLeagueMovement *int               `json:"miniLeagueMovement"`
	MiniLeagueMessage  string             `json:"miniLeagueMessage"`
	Warnings           []string           `json:"warnings"`
}

func (s *Store) TryLiveSyncLease(ctx context.Context) (bool, func(), error) {
	connection, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, nil, err
	}
	var locked bool
	if err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext('fpl_edge_live_sync'))`).Scan(&locked); err != nil {
		connection.Release()
		return false, nil, err
	}
	if !locked {
		connection.Release()
		return false, func() {}, nil
	}
	release := func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('fpl_edge_live_sync'))`)
		connection.Release()
	}
	return true, release, nil
}

func (s *Store) LiveSyncTarget(ctx context.Context) (int, bool, error) {
	var gameweek int
	err := s.pool.QueryRow(ctx, `SELECT g.number FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.is_current AND g.is_current ORDER BY g.number DESC LIMIT 1`).Scan(&gameweek)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var fetched *time.Time
	if err = s.pool.QueryRow(ctx, `SELECT max(l.fetched_at) FROM live_gameweek_snapshots l JOIN seasons s ON s.id=l.season_id WHERE s.is_current AND l.gameweek=$1`, gameweek).Scan(&fetched); err != nil {
		return 0, false, err
	}
	return gameweek, fetched == nil || time.Since(*fetched) >= liveSnapshotFreshness, nil
}

func (s *Store) SyncLiveData(ctx context.Context, data domain.LiveGameweek) error {
	status := "scheduled"
	started, final := false, len(data.Fixtures) > 0
	for _, fixture := range data.Fixtures {
		started = started || fixture.Started || fixture.Finished || fixture.FinishedProvisional
		final = final && fixture.Finished
	}
	if started {
		status = "provisional"
	}
	if final {
		status = "final"
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	content := data
	content.FetchedAt = time.Time{}
	contentPayload, err := json.Marshal(content)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(contentPayload)
	_, err = s.pool.Exec(ctx, `INSERT INTO live_gameweek_snapshots(season_id,gameweek,payload_hash,payload,status,fetched_at)
		SELECT id,$1,$2,$3,$4,$5 FROM seasons WHERE is_current
		ON CONFLICT(season_id,gameweek,payload_hash) DO UPDATE SET fetched_at=EXCLUDED.fetched_at,status=EXCLUDED.status,payload=EXCLUDED.payload`, data.Gameweek, hash[:], payload, status, data.FetchedAt)
	return err
}

type liveIdentity struct {
	ID         uuid.UUID
	ProviderID int
	ClubID     int
	Name       string
	Club       string
	Position   domain.Position
}

func (s *Store) LiveGameweek(ctx context.Context, userID uuid.UUID) (LiveGameweekResult, error) {
	squad, err := s.Squad(ctx, userID)
	if err != nil {
		return LiveGameweekResult{}, err
	}
	var payload []byte
	result := LiveGameweekResult{MiniLeagueMessage: "Connect a public FPL team and mini-league before rank movement can be calculated.", Warnings: []string{}}
	err = s.pool.QueryRow(ctx, `SELECT gameweek,status,fetched_at,payload FROM live_gameweek_snapshots WHERE season_id=$1 ORDER BY fetched_at DESC,created_at DESC LIMIT 1`, squad.SeasonID).Scan(&result.Gameweek, &result.Status, &result.FetchedAt, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrLiveUnavailable
	}
	if err != nil {
		return result, err
	}
	var live domain.LiveGameweek
	if err = json.Unmarshal(payload, &live); err != nil {
		return result, err
	}
	result.Stale = time.Since(result.FetchedAt) > 5*time.Minute
	if result.Stale {
		result.Warnings = append(result.Warnings, "Live data is older than five minutes; scores may have changed.")
	}
	selection := squad
	if result.Gameweek != squad.CurrentGameweek {
		var state []byte
		err = s.pool.QueryRow(ctx, `SELECT squad_revision,state FROM gameweek_squad_selections WHERE squad_id=$1 AND gameweek=$2`, squad.ID, result.Gameweek).Scan(&selection.Revision, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, ErrLiveSelectionUnavailable
		}
		if err != nil {
			return result, err
		}
		var locked struct {
			Players       []uuid.UUID `json:"players"`
			Starters      []uuid.UUID `json:"starters"`
			Bench         []uuid.UUID `json:"bench"`
			CaptainID     uuid.UUID   `json:"captainId"`
			ViceCaptainID uuid.UUID   `json:"viceCaptainId"`
		}
		if err = json.Unmarshal(state, &locked); err != nil {
			return result, err
		}
		selection.Players = nil
		selection.Starters = locked.Starters
		selection.Bench = locked.Bench
		selection.CaptainID = locked.CaptainID
		selection.ViceCaptainID = locked.ViceCaptainID
		for _, id := range locked.Players {
			selection.Players = append(selection.Players, domain.SquadPlayer{PlayerID: id})
		}
	}
	result.SquadRevision = selection.Revision
	if err = s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT name FROM squad_chips WHERE squad_id=$1 AND gameweek=$2),'')`, squad.ID, result.Gameweek).Scan(&result.ActiveChip); err != nil {
		return result, err
	}
	playerIDs := make([]uuid.UUID, 0, len(selection.Players))
	for _, player := range selection.Players {
		playerIDs = append(playerIDs, player.PlayerID)
	}
	identities, err := s.liveIdentities(ctx, squad.SeasonID, playerIDs)
	if err != nil {
		return result, err
	}
	if len(identities) != 15 {
		return result, ErrLiveSelectionUnavailable
	}
	selection.Players = selection.Players[:0]
	for _, identity := range identities {
		selection.Players = append(selection.Players, domain.SquadPlayer{PlayerID: identity.ID, Position: identity.Position})
	}
	providerStats := make(map[int]domain.LivePlayer, len(live.Players))
	for _, player := range live.Players {
		providerStats[player.ProviderID] = player
	}
	stats := make(map[uuid.UUID]domain.LivePlayer, len(identities))
	clubFixtures := map[int][]domain.FootballFixture{}
	for _, fixture := range live.Fixtures {
		clubFixtures[fixture.Home] = append(clubFixtures[fixture.Home], fixture)
		clubFixtures[fixture.Away] = append(clubFixtures[fixture.Away], fixture)
		if !fixture.Finished {
			result.RemainingFixtures++
		}
	}
	for _, identity := range identities {
		stats[identity.ID] = providerStats[identity.ProviderID]
	}
	noShows := map[uuid.UUID]bool{}
	for _, identity := range identities {
		fixtures := clubFixtures[identity.ClubID]
		finished := len(fixtures) > 0
		for _, fixture := range fixtures {
			finished = finished && fixture.Finished
		}
		noShows[identity.ID] = finished && stats[identity.ID].Minutes == 0
	}
	multipliers, autoSubs := liveMultipliers(selection, stats, noShows, result.ActiveChip)
	result.EstimatedAutoSubs = autoSubs
	autoSubSet := map[uuid.UUID]bool{}
	for _, id := range autoSubs {
		autoSubSet[id] = true
	}
	for _, identity := range identities {
		playerStats := stats[identity.ID]
		remaining := false
		for _, fixture := range clubFixtures[identity.ClubID] {
			if !fixture.Finished {
				remaining = true
				break
			}
		}
		multiplier := multipliers[identity.ID]
		entry := LivePlayerResult{PlayerID: identity.ID, Name: identity.Name, Club: identity.Club, Position: identity.Position, IsStarter: containsID(selection.Starters, identity.ID), IsCaptain: selection.CaptainID == identity.ID, IsViceCaptain: selection.ViceCaptainID == identity.ID, Multiplier: multiplier, Points: playerStats.TotalPoints, CountedPoints: playerStats.TotalPoints * multiplier, Minutes: playerStats.Minutes, Bonus: playerStats.Bonus, BPS: playerStats.BPS, DefensiveContribution: playerStats.DefensiveContribution, Remaining: remaining, EstimatedAutoSub: autoSubSet[identity.ID]}
		fixtureCount := 0
		for _, fixture := range clubFixtures[identity.ClubID] {
			if fixture.Started || fixture.Finished || fixture.FinishedProvisional {
				fixtureCount++
			}
		}
		if fixtureCount == 0 {
			fixtureCount = 1
		}
		if identity.Position == domain.Defender {
			entry.DefensiveTarget = 10 * fixtureCount
		} else if identity.Position == domain.Midfielder || identity.Position == domain.Forward {
			entry.DefensiveTarget = 12 * fixtureCount
		}
		result.Points += entry.CountedPoints
		result.EstimatedBonus += playerStats.Bonus * multiplier
		if multiplier == 0 {
			result.BenchPoints += playerStats.TotalPoints
		}
		if multiplier > 1 {
			result.CaptainPoints = entry.CountedPoints
			result.CaptainExtra = playerStats.TotalPoints * maxInt(0, multiplier-1)
		}
		if remaining {
			result.RemainingPlayers++
		}
		result.Players = append(result.Players, entry)
	}
	result.Warnings = append(result.Warnings, "Live totals are provisional until every fixture is final and provider corrections settle.", "Automatic substitutions and vice-captain fallback are estimates based on minutes currently reported.")
	return result, nil
}

func (s *Store) liveIdentities(ctx context.Context, seasonID uuid.UUID, playerIDs []uuid.UUID) ([]liveIdentity, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.provider_id,c.provider_id,p.first_name||' '||p.last_name,c.short_name,p.position FROM players p JOIN clubs c ON c.id=p.club_id WHERE p.season_id=$1 AND p.id=ANY($2) ORDER BY p.id`, seasonID, playerIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []liveIdentity{}
	for rows.Next() {
		var item liveIdentity
		if err = rows.Scan(&item.ID, &item.ProviderID, &item.ClubID, &item.Name, &item.Club, &item.Position); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func liveMultipliers(squad domain.Squad, stats map[uuid.UUID]domain.LivePlayer, noShows map[uuid.UUID]bool, chip string) (map[uuid.UUID]int, []uuid.UUID) {
	multipliers := map[uuid.UUID]int{}
	for _, id := range squad.Starters {
		multipliers[id] = 1
	}
	if chip == "bboost" {
		for _, id := range squad.Bench {
			multipliers[id] = 1
		}
	}
	autoSubs := []uuid.UUID{}
	if chip != "bboost" {
		positions := map[uuid.UUID]domain.Position{}
		for _, p := range squad.Players {
			positions[p.PlayerID] = p.Position
		}
		used := map[uuid.UUID]bool{}
		for _, starter := range squad.Starters {
			if !noShows[starter] {
				continue
			}
			for _, bench := range squad.Bench {
				if used[bench] || stats[bench].Minutes == 0 || (positions[starter] == domain.Goalkeeper) != (positions[bench] == domain.Goalkeeper) {
					continue
				}
				if positions[starter] != domain.Goalkeeper && !validLiveFormation(squad.Starters, multipliers, positions, starter, bench) {
					continue
				}
				multipliers[starter] = 0
				multipliers[bench] = 1
				used[bench] = true
				autoSubs = append(autoSubs, bench)
				break
			}
		}
	}
	captain, vice := squad.CaptainID, squad.ViceCaptainID
	if noShows[captain] && stats[vice].Minutes > 0 {
		captain = vice
	}
	if multipliers[captain] > 0 {
		if chip == "3xc" {
			multipliers[captain] = 3
		} else {
			multipliers[captain] = 2
		}
	}
	return multipliers, autoSubs
}

func validLiveFormation(starters []uuid.UUID, multipliers map[uuid.UUID]int, positions map[uuid.UUID]domain.Position, outgoing, incoming uuid.UUID) bool {
	counts := map[domain.Position]int{}
	for _, id := range starters {
		if id != outgoing && multipliers[id] > 0 {
			counts[positions[id]]++
		}
	}
	counts[positions[incoming]]++
	return counts[domain.Defender] >= 3 && counts[domain.Midfielder] >= 2 && counts[domain.Forward] >= 1
}

func containsID(ids []uuid.UUID, target uuid.UUID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
