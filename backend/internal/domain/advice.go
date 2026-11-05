package domain

import (
	"errors"
	"sort"

	"github.com/google/uuid"
)

type LineupAdvice struct {
	Starters      []uuid.UUID `json:"starters"`
	Bench         []uuid.UUID `json:"bench"`
	CaptainID     uuid.UUID   `json:"captainId"`
	ViceCaptainID uuid.UUID   `json:"viceCaptainId"`
	Points        float64     `json:"points"`
	BenchPoints   float64     `json:"benchPoints"`
	CaptainPoints float64     `json:"captainPoints"`
}
type TransferAdvice struct {
	OutID        uuid.UUID `json:"outId"`
	InID         uuid.UUID `json:"inId"`
	NetGain      float64   `json:"netGain"`
	NextWeekGain float64   `json:"nextWeekGain"`
	Hit          int       `json:"hit"`
	BankAfter    int       `json:"bankAfter"`
}

type RebuildAdvice struct {
	Moves        []TransferAdvice `json:"moves"`
	Gain         float64          `json:"gain"`
	NextWeekGain float64          `json:"nextWeekGain"`
}

// PlanRebuild is a bounded greedy search, not a global optimizer. Each step
// preserves the budget and club/position constraints. It cannot discover every
// downgrade-to-fund-an-upgrade combination, so gains are conservative examples.
func PlanRebuild(squad Squad, forecast Forecast, oneWeek bool) (RebuildAdvice, error) {
	result := RebuildAdvice{Moves: []TransferAdvice{}}
	byID := forecast.ByID()
	end := forecast.HorizonEnd
	if oneWeek {
		end = forecast.Gameweek
	}
	score := func(players []SquadPlayer) (float64, float64, error) {
		total, next := 0.0, 0.0
		for gw := forecast.Gameweek; gw <= end; gw++ {
			lineup, err := recommendLineup(players, byID, gw)
			if err != nil {
				return 0, 0, err
			}
			total += lineup.Points
			if gw == forecast.Gameweek {
				next = lineup.Points
			}
		}
		return total, next, nil
	}
	baseline, initialNext, err := score(squad.Players)
	if err != nil {
		return result, err
	}
	current := append([]SquadPlayer(nil), squad.Players...)
	bank := squad.Bank
	total := baseline
	next := initialNext
	changed := map[int]bool{}
	for step := 0; step < 8; step++ {
		owned := map[uuid.UUID]bool{}
		clubs := map[uuid.UUID]int{}
		for _, p := range current {
			owned[p.PlayerID] = true
			clubs[p.ClubID]++
		}
		bestIndex := -1
		var bestPlayer SquadPlayer
		bestGain := 0.5
		bestTotal, bestNext := total, next
		bestBank := bank
		for i, out := range current {
			if changed[i] {
				continue
			}
			for _, in := range forecast.Players {
				if owned[in.PlayerID] || in.Position != out.Position || in.Status != "a" || in.Coverage < 5 {
					continue
				}
				remaining := bank + SellingPrice(out.PurchasePrice, out.CurrentPrice) - in.CurrentPrice
				if remaining < 0 || (in.ClubID != out.ClubID && clubs[in.ClubID] >= 3) {
					continue
				}
				inPoints, outPoints := in.HorizonTotal, byID[out.PlayerID].HorizonTotal
				if oneWeek {
					inPoints = in.Gameweeks[forecast.Gameweek]
					outPoints = byID[out.PlayerID].Gameweeks[forecast.Gameweek]
				}
				if inPoints <= outPoints {
					continue
				}
				candidate := append([]SquadPlayer(nil), current...)
				candidate[i] = in.SquadPlayer
				candidateTotal, candidateNext, e := score(candidate)
				if e != nil {
					return result, e
				}
				if candidateTotal-total > bestGain {
					bestIndex = i
					bestPlayer = in.SquadPlayer
					bestGain = candidateTotal - total
					bestTotal = candidateTotal
					bestNext = candidateNext
					bestBank = remaining
				}
			}
		}
		if bestIndex < 0 {
			break
		}
		result.Moves = append(result.Moves, TransferAdvice{OutID: current[bestIndex].PlayerID, InID: bestPlayer.PlayerID, NetGain: bestGain, NextWeekGain: bestNext - next, BankAfter: bestBank})
		bestPlayer.PurchasePrice = bestPlayer.CurrentPrice
		current[bestIndex] = bestPlayer
		changed[bestIndex] = true
		bank = bestBank
		total = bestTotal
		next = bestNext
	}
	result.Gain = total - baseline
	result.NextWeekGain = next - initialNext
	return result, nil
}

func RecommendLineup(players []SquadPlayer, forecast Forecast, gw int) (LineupAdvice, error) {
	return recommendLineup(players, forecast.ByID(), gw)
}

func recommendLineup(players []SquadPlayer, byID map[uuid.UUID]PlayerProjection, gw int) (LineupAdvice, error) {
	projected := append([]SquadPlayer(nil), players...)
	for i, p := range projected {
		prediction, ok := byID[p.PlayerID]
		if !ok {
			return LineupAdvice{}, errors.New("owned player is missing from forecast")
		}
		projected[i].ExpectedPoints = prediction.Gameweeks[gw]
	}
	// Stable horizon tie-breaks never override the current gameweek objective.
	sort.SliceStable(projected, func(i, j int) bool {
		return byID[projected[i].PlayerID].NextFiveTotal > byID[projected[j].PlayerID].NextFiveTotal
	})
	starters, bench, captain, vice, err := OptimizeLineup(projected)
	if err != nil {
		return LineupAdvice{}, err
	}
	result := LineupAdvice{Starters: starters, Bench: bench, CaptainID: captain, ViceCaptainID: vice, CaptainPoints: byID[captain].Gameweeks[gw]}
	for _, id := range starters {
		result.Points += byID[id].Gameweeks[gw]
	}
	result.Points += result.CaptainPoints
	for _, id := range bench {
		result.BenchPoints += byID[id].Gameweeks[gw]
	}
	return result, nil
}

// RankTransfers compares optimized XI plus captain totals over the SAME weeks,
// not raw squad totals. Recommendations are independent one-transfer options.
func RankTransfers(squad Squad, forecast Forecast, chip string) ([]TransferAdvice, error) {
	result := []TransferAdvice{}
	byID := forecast.ByID()
	end := forecast.HorizonEnd
	if chip == "freehit" {
		end = forecast.Gameweek
	}
	weighted := func(lineup LineupAdvice, gw int) float64 {
		points := lineup.Points
		if gw == forecast.Gameweek {
			if chip == "bboost" {
				points += lineup.BenchPoints
			}
			if chip == "3xc" {
				points += lineup.CaptainPoints
			}
		}
		return points
	}
	baseline := 0.0
	nextBaseline := 0.0
	for gw := forecast.Gameweek; gw <= end; gw++ {
		lineup, err := recommendLineup(squad.Players, byID, gw)
		if err != nil {
			return nil, err
		}
		baseline += weighted(lineup, gw)
		if gw == forecast.Gameweek {
			nextBaseline = weighted(lineup, gw)
		}
	}
	owned := map[uuid.UUID]bool{}
	clubs := map[uuid.UUID]int{}
	for _, p := range squad.Players {
		owned[p.PlayerID] = true
		clubs[p.ClubID]++
	}
	hit := TransferHit(squad.FreeTransfers, 1, DefaultRules())
	if chip == "wildcard" || chip == "freehit" {
		hit = 0
	}
	for i, out := range squad.Players {
		for _, in := range forecast.Players {
			if owned[in.PlayerID] || in.Position != out.Position || in.Status != "a" || in.Coverage < 5 || byID[out.PlayerID].Coverage < 5 {
				continue
			}
			bank := squad.Bank + SellingPrice(out.PurchasePrice, out.CurrentPrice) - in.CurrentPrice
			if bank < 0 || (in.ClubID != out.ClubID && clubs[in.ClubID] >= 3) {
				continue
			}
			// Only shortlist positive raw expected-point improvements; XI scoring then
			// rejects bench-only gains and deducts the immediate transfer hit.
			inTotal, outTotal := in.HorizonTotal, byID[out.PlayerID].HorizonTotal
			if chip == "freehit" {
				inTotal = in.Gameweeks[forecast.Gameweek]
				outTotal = byID[out.PlayerID].Gameweeks[forecast.Gameweek]
			}
			if inTotal <= outTotal {
				continue
			}
			candidate := append([]SquadPlayer(nil), squad.Players...)
			candidate[i] = in.SquadPlayer
			total := 0.0
			next := 0.0
			for gw := forecast.Gameweek; gw <= end; gw++ {
				lineup, err := recommendLineup(candidate, byID, gw)
				if err != nil {
					return nil, err
				}
				total += weighted(lineup, gw)
				if gw == forecast.Gameweek {
					next = weighted(lineup, gw)
				}
			}
			gain := total - baseline - float64(hit)
			if gain < 2 {
				continue
			}
			result = append(result, TransferAdvice{OutID: out.PlayerID, InID: in.PlayerID, NetGain: gain, NextWeekGain: next - nextBaseline - float64(hit), Hit: hit, BankAfter: bank})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].NetGain == result[j].NetGain {
			return result[i].InID.String() < result[j].InID.String()
		}
		return result[i].NetGain > result[j].NetGain
	})
	if len(result) > 5 {
		result = result[:5]
	}
	return result, nil
}
