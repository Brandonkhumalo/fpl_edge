package domain

import (
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

type Position string

const (
	Goalkeeper Position = "GKP"
	Defender   Position = "DEF"
	Midfielder Position = "MID"
	Forward    Position = "FWD"
)

type SeasonRules struct {
	SquadSize          int
	Budget             int
	MaxPlayersPerClub  int
	FreeTransferCap    int
	TransferHitPoints  int
	PositionAllocation map[Position]int
}

func DefaultRules() SeasonRules {
	return SeasonRules{
		SquadSize: 15, Budget: 1000, MaxPlayersPerClub: 3,
		FreeTransferCap: 5, TransferHitPoints: 4,
		PositionAllocation: map[Position]int{Goalkeeper: 2, Defender: 5, Midfielder: 5, Forward: 3},
	}
}

type SquadPlayer struct {
	PlayerID       uuid.UUID `json:"playerId"`
	ClubID         uuid.UUID `json:"clubId"`
	ClubName       string    `json:"clubName"`
	KitCode        int       `json:"kitCode,omitempty"`
	SquadNumber    *int      `json:"squadNumber"`
	Name           string    `json:"name"`
	Position       Position  `json:"position"`
	CurrentPrice   int       `json:"currentPrice"`
	PurchasePrice  int       `json:"purchasePrice"`
	SellingPrice   int       `json:"sellingPrice"`
	ExpectedPoints float64   `json:"expectedPoints"`
	ExpectedMins   float64   `json:"expectedMinutes"`
}

type Squad struct {
	SeasonID        uuid.UUID     `json:"seasonId"`
	ID              uuid.UUID     `json:"id"`
	Revision        int64         `json:"revision"`
	Bank            int           `json:"bank"`
	FreeTransfers   int           `json:"freeTransfers"`
	CurrentGameweek int           `json:"currentGameweek"`
	Players         []SquadPlayer `json:"players"`
	Starters        []uuid.UUID   `json:"starters"`
	Bench           []uuid.UUID   `json:"bench"`
	CaptainID       uuid.UUID     `json:"captainId"`
	ViceCaptainID   uuid.UUID     `json:"viceCaptainId"`
}

func SellingPrice(purchasePrice, currentPrice int) int {
	if currentPrice <= purchasePrice {
		return currentPrice
	}
	return purchasePrice + (currentPrice-purchasePrice)/2
}

func ValidateSquad(players []SquadPlayer, bank int, rules SeasonRules) error {
	if len(players) != rules.SquadSize {
		return fmt.Errorf("squad must contain exactly %d players", rules.SquadSize)
	}
	if bank < 0 {
		return errors.New("bank balance cannot be negative")
	}
	positions := make(map[Position]int)
	clubs := make(map[uuid.UUID]int)
	seen := make(map[uuid.UUID]struct{})
	total := bank
	for _, player := range players {
		if _, exists := seen[player.PlayerID]; exists {
			return errors.New("a player cannot appear more than once")
		}
		seen[player.PlayerID] = struct{}{}
		positions[player.Position]++
		clubs[player.ClubID]++
		if clubs[player.ClubID] > rules.MaxPlayersPerClub {
			return fmt.Errorf("a squad cannot contain more than %d players from one club", rules.MaxPlayersPerClub)
		}
		if player.PurchasePrice <= 0 {
			return errors.New("purchase prices must be positive")
		}
		total += player.PurchasePrice
	}
	for position, required := range rules.PositionAllocation {
		if positions[position] != required {
			return fmt.Errorf("squad requires %d %s players", required, position)
		}
	}
	if total > rules.Budget {
		return fmt.Errorf("squad cost and bank cannot exceed %d price units", rules.Budget)
	}
	return nil
}

func ValidateLineup(squad Squad) error {
	if len(squad.Starters) != 11 || len(squad.Bench) != 4 {
		return errors.New("lineup requires 11 starters and four ordered substitutes")
	}
	owned := make(map[uuid.UUID]SquadPlayer, len(squad.Players))
	for _, player := range squad.Players {
		owned[player.PlayerID] = player
	}
	selected := make(map[uuid.UUID]struct{}, 15)
	counts := make(map[Position]int)
	for _, id := range append(append([]uuid.UUID{}, squad.Starters...), squad.Bench...) {
		player, exists := owned[id]
		if !exists {
			return errors.New("lineup includes a player not owned by the squad")
		}
		if _, exists := selected[id]; exists {
			return errors.New("lineup includes a duplicate player")
		}
		selected[id] = struct{}{}
		if len(selected) <= 11 {
			counts[player.Position]++
		}
	}
	if counts[Goalkeeper] != 1 || counts[Defender] < 3 || counts[Midfielder] < 2 || counts[Forward] < 1 {
		return errors.New("starting XI does not form a legal formation")
	}
	if _, exists := selected[squad.CaptainID]; !exists || !contains(squad.Starters, squad.CaptainID) {
		return errors.New("captain must be a starter")
	}
	if _, exists := selected[squad.ViceCaptainID]; !exists || !contains(squad.Starters, squad.ViceCaptainID) {
		return errors.New("vice-captain must be a starter")
	}
	if squad.CaptainID == squad.ViceCaptainID {
		return errors.New("captain and vice-captain must differ")
	}
	return nil
}

func OptimizeLineup(players []SquadPlayer) (starters, bench []uuid.UUID, captain, vice uuid.UUID, err error) {
	if len(players) != 15 {
		return nil, nil, uuid.Nil, uuid.Nil, errors.New("optimizer requires a complete 15-player squad")
	}
	counts := map[Position]int{}
	unique := map[uuid.UUID]bool{}
	for _, p := range players {
		if unique[p.PlayerID] {
			return nil, nil, uuid.Nil, uuid.Nil, errors.New("duplicate optimizer player")
		}
		unique[p.PlayerID] = true
		counts[p.Position]++
	}
	for position, count := range DefaultRules().PositionAllocation {
		if counts[position] != count {
			return nil, nil, uuid.Nil, uuid.Nil, errors.New("invalid optimizer squad composition")
		}
	}
	sorted := append([]SquadPlayer(nil), players...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return adjustedPoints(sorted[i]) > adjustedPoints(sorted[j])
	})

	pickBest := func(position Position, count int) []SquadPlayer {
		result := make([]SquadPlayer, 0, count)
		for _, p := range sorted {
			if p.Position == position && len(result) < count {
				result = append(result, p)
			}
		}
		return result
	}
	selected := append([]SquadPlayer{}, pickBest(Goalkeeper, 1)...)
	selected = append(selected, pickBest(Defender, 3)...)
	selected = append(selected, pickBest(Midfielder, 2)...)
	selected = append(selected, pickBest(Forward, 1)...)
	chosen := make(map[uuid.UUID]struct{})
	for _, p := range selected {
		chosen[p.PlayerID] = struct{}{}
	}
	for _, p := range sorted {
		if len(selected) == 11 {
			break
		}
		if _, exists := chosen[p.PlayerID]; !exists && p.Position != Goalkeeper {
			selected = append(selected, p)
			chosen[p.PlayerID] = struct{}{}
		}
	}
	sort.SliceStable(selected, func(i, j int) bool { return adjustedPoints(selected[i]) > adjustedPoints(selected[j]) })
	for _, p := range selected {
		starters = append(starters, p.PlayerID)
	}
	for _, p := range sorted {
		if _, exists := chosen[p.PlayerID]; !exists {
			bench = append(bench, p.PlayerID)
		}
	}
	return starters, bench, selected[0].PlayerID, selected[1].PlayerID, nil
}

func TransferHit(freeTransfers, transfers int, rules SeasonRules) int {
	chargeable := transfers - freeTransfers
	if chargeable < 0 {
		chargeable = 0
	}
	return chargeable * rules.TransferHitPoints
}

func contains(ids []uuid.UUID, wanted uuid.UUID) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func adjustedPoints(player SquadPlayer) float64 {
	// Expected points already include minutes and availability probabilities.
	return player.ExpectedPoints
}
