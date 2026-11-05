package domain

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const ForecastModelVersion = "fixture-baseline-2026.1"

type FixtureProjection struct {
	FixtureID       int                `json:"fixtureId"`
	Gameweek        int                `json:"gameweek"`
	Opponent        string             `json:"opponent"`
	Home            bool               `json:"home"`
	Kickoff         *time.Time         `json:"kickoff"`
	Difficulty      int                `json:"difficulty"`
	ExpectedMinutes float64            `json:"expectedMinutes"`
	Points          float64            `json:"points"`
	Breakdown       map[string]float64 `json:"breakdown"`
}
type PlayerProjection struct {
	SquadPlayer
	Status        string              `json:"status"`
	Confidence    string              `json:"confidence"`
	SampleMinutes int                 `json:"sampleMinutes"`
	Fixtures      []FixtureProjection `json:"fixtures"`
	Gameweeks     map[int]float64     `json:"gameweeks"`
	NextFiveTotal float64             `json:"nextFiveTotal"`
	HorizonTotal  float64             `json:"horizonTotal"`
	Coverage      int                 `json:"coverage"`
}
type Forecast struct {
	ModelVersion string             `json:"modelVersion"`
	Gameweek     int                `json:"gameweek"`
	HorizonEnd   int                `json:"horizonEnd"`
	Players      []PlayerProjection `json:"players"`
	Warnings     []string           `json:"warnings"`
}

// ValidateForecastData fails closed on missing statistical inputs. Zero is a
// measurement; a missing field is not. No synthetic production fallback exists.
func ValidateForecastData(data FootballData) error {
	if len(data.Fixtures) == 0 || len(data.Players) == 0 {
		return errors.New("forecast source has no fixtures or players")
	}
	clubs := map[int]bool{}
	for _, c := range data.Clubs {
		clubs[c.ProviderID] = true
	}
	weeks := map[int]bool{}
	for _, w := range data.Gameweeks {
		weeks[w.Number] = true
	}
	seen := map[int]bool{}
	for _, f := range data.Fixtures {
		if f.ID <= 0 || seen[f.ID] || !clubs[f.Home] || !clubs[f.Away] || f.Home == f.Away || (f.Gameweek != nil && !weeks[*f.Gameweek]) {
			return errors.New("invalid forecast fixture")
		}
		if f.HomeDifficulty < 1 || f.HomeDifficulty > 5 || f.AwayDifficulty < 1 || f.AwayDifficulty > 5 {
			return errors.New("invalid fixture difficulty")
		}
		if f.Finished && (f.HomeScore == nil || f.AwayScore == nil || *f.HomeScore < 0 || *f.AwayScore < 0) {
			return errors.New("finished fixture has no valid score")
		}
		seen[f.ID] = true
	}
	for _, p := range data.Players {
		if p.Minutes == nil || *p.Minutes < 0 || !clubs[p.ClubID] {
			return fmt.Errorf("missing player statistics: %d", p.ProviderID)
		}
		for _, raw := range []string{p.XG, p.XA} {
			n, e := strconv.ParseFloat(raw, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
				return fmt.Errorf("invalid expected statistics: %d", p.ProviderID)
			}
		}
		for _, n := range []int{p.Starts, p.Goals, p.Assists, p.Bonus, p.Saves, p.PenaltySaves, p.PenaltyMisses, p.Yellow, p.Red, p.OwnGoals, p.DefensiveActions} {
			if n < 0 {
				return errors.New("negative player statistic")
			}
		}
		if p.Chance != nil && (*p.Chance < 0 || *p.Chance > 100) {
			return errors.New("invalid availability probability")
		}
	}
	return nil
}

// ForecastPlayers uses a common calendar horizon long enough to include each
// club's next five assigned fixtures. This avoids comparing five games ending
// in different weeks, and includes extra fixtures in double gameweeks.
func ForecastPlayers(data FootballData, ids map[int]SquadPlayer, gw int, now time.Time) (Forecast, error) {
	result := Forecast{ModelVersion: ForecastModelVersion, Gameweek: gw, HorizonEnd: gw + 4, Players: []PlayerProjection{}, Warnings: []string{}}
	if err := ValidateForecastData(data); err != nil {
		return result, err
	}
	names := map[int]string{}
	games := map[int]int{}
	conceded := map[int]float64{}
	upcoming := map[int][]FootballFixture{}
	lastWeek := 0
	for _, w := range data.Gameweeks {
		if w.Number > lastWeek {
			lastWeek = w.Number
		}
	}
	if gw > lastWeek {
		return result, errors.New("season has no remaining planning gameweek")
	}
	unassigned := 0
	for _, c := range data.Clubs {
		names[c.ProviderID] = c.ShortName
	}
	for _, f := range data.Fixtures {
		if f.Finished {
			games[f.Home]++
			games[f.Away]++
			conceded[f.Home] += float64(*f.AwayScore)
			conceded[f.Away] += float64(*f.HomeScore)
			continue
		}
		if f.Started {
			continue
		}
		if f.Gameweek == nil || f.Kickoff == nil {
			unassigned++
			continue
		}
		if *f.Gameweek < gw || f.Kickoff.Before(now) {
			continue
		}
		upcoming[f.Home] = append(upcoming[f.Home], f)
		upcoming[f.Away] = append(upcoming[f.Away], f)
	}
	for club, fixtures := range upcoming {
		sort.Slice(fixtures, func(i, j int) bool {
			if fixtures[i].Kickoff.Equal(*fixtures[j].Kickoff) {
				return fixtures[i].ID < fixtures[j].ID
			}
			return fixtures[i].Kickoff.Before(*fixtures[j].Kickoff)
		})
		upcoming[club] = fixtures
		for i := 0; i < len(fixtures) && i < 5; i++ {
			if *fixtures[i].Gameweek > result.HorizonEnd {
				result.HorizonEnd = *fixtures[i].Gameweek
			}
		}
	}
	if result.HorizonEnd > lastWeek {
		result.HorizonEnd = lastWeek
	}
	if unassigned > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d unassigned or undated fixtures excluded; predictions will change when rescheduled.", unassigned))
	}
	for _, p := range data.Players {
		identity, ok := ids[p.ProviderID]
		if !ok {
			continue
		}
		identity.CurrentPrice = p.Price
		identity.Position = p.Position
		projection := PlayerProjection{SquadPlayer: identity, Status: p.Status, Confidence: "low", SampleMinutes: *p.Minutes, Fixtures: []FixtureProjection{}, Gameweeks: map[int]float64{}}
		if *p.Minutes >= 900 {
			projection.Confidence = "moderate"
		}
		for w := gw; w <= result.HorizonEnd; w++ {
			projection.Gameweeks[w] = 0
		}
		for i, f := range upcoming[p.ClubID] {
			if *f.Gameweek > result.HorizonEnd {
				continue
			}
			fixture := projectFixture(p, f, games[p.ClubID], conceded[p.ClubID], names)
			projection.Fixtures = append(projection.Fixtures, fixture)
			projection.Gameweeks[fixture.Gameweek] += fixture.Points
			projection.HorizonTotal += fixture.Points
			if i < 5 {
				projection.NextFiveTotal += fixture.Points
				projection.Coverage++
			}
		}
		projection.ExpectedPoints = projection.Gameweeks[gw]
		for _, f := range projection.Fixtures {
			if f.Gameweek == gw {
				projection.ExpectedMins += f.ExpectedMinutes
			}
		}
		result.Players = append(result.Players, projection)
	}
	if len(result.Players) == 0 {
		return result, errors.New("no mapped players for predictions")
	}
	result.Warnings = append(result.Warnings, "Experimental statistical estimates, not guaranteed returns or a calibrated ML model. Availability is held constant across the horizon; transfers, injuries and rotation can change it.", "Low-minute players have weak evidence. Bonus and rare-event rates are estimates; future BPS rankings and automatic substitutions are not simulated.")
	return result, nil
}

func projectFixture(p FootballPlayer, f FootballFixture, games int, conceded float64, names map[int]string) FixtureProjection {
	home := f.Home == p.ClubID
	opponent := f.Home
	difficulty := f.AwayDifficulty
	if home {
		opponent = f.Away
		difficulty = f.HomeDifficulty
	}
	result := FixtureProjection{FixtureID: f.ID, Gameweek: *f.Gameweek, Opponent: names[opponent], Home: home, Kickoff: f.Kickoff, Difficulty: difficulty, Breakdown: map[string]float64{}}
	minutes := float64(*p.Minutes)
	matches := math.Max(float64(games), math.Ceil(minutes/90))
	availability := 1.0
	if p.Chance != nil {
		availability = float64(*p.Chance) / 100
	} else if p.Status != "a" {
		availability = 0
	}
	if p.Status == "u" {
		availability = 0
	}
	start := 0.0
	cameo := 0.0
	if matches > 0 {
		start = clamp(float64(p.Starts)/matches, 0, 1)
		cameo = clamp((minutes-float64(p.Starts)*80)/(matches*20), 0, 1-start)
	}
	// A two-state minutes approximation: 80-minute starts and 20-minute cameos.
	// Scale the start duration down for frequently substituted players.
	duration := 80.0
	if p.Starts > 0 {
		duration = clamp((minutes-cameo*matches*20)/float64(p.Starts), 45, 90)
	}
	sixty := start * clamp((duration-45)/30, 0, 1)
	playing := start + cameo
	expectedMinutes := (start*duration + cameo*20) * availability
	result.ExpectedMinutes = expectedMinutes
	exposure := expectedMinutes / 90
	attacking := math.Exp(float64(3-difficulty) * 0.18)
	if home {
		attacking *= 1.08
	} else {
		attacking *= 0.92
	}
	// Shrink sparse seasonal xG/xA towards explicit conservative position priors.
	priorGoals := map[Position]float64{Goalkeeper: 0.001, Defender: 0.04, Midfielder: 0.16, Forward: 0.3}[p.Position]
	priorAssists := map[Position]float64{Goalkeeper: 0.001, Defender: 0.06, Midfielder: 0.13, Forward: 0.1}[p.Position]
	xg, _ := strconv.ParseFloat(p.XG, 64)
	xa, _ := strconv.ParseFloat(p.XA, 64)
	goals := (xg + 5*priorGoals) / (minutes/90 + 5) * exposure * attacking
	assists := (xa + 5*priorAssists) / (minutes/90 + 5) * exposure * attacking
	coeff := DefaultProjectionCoefficients()
	b := result.Breakdown
	b["appearance"] = (playing + sixty) * availability
	b["goals"] = goals * coeff.GoalPoints[p.Position]
	b["assists"] = assists * 3
	concededRate := (conceded + 5*1.4) / (float64(games) + 5) / attacking
	b["cleanSheet"] = sixty * availability * math.Exp(-concededRate*duration/90) * coeff.CleanSheetPoints[p.Position]
	denominator := minutes/90 + 5
	b["bonus"] = clamp(float64(p.Bonus)/denominator*exposure, 0, 3*availability)
	threshold := 12
	if p.Position == Defender {
		threshold = 10
	}
	b["defence"] = 0
	if p.Position != Goalkeeper {
		actionRate := float64(p.DefensiveActions) / denominator
		b["defence"] = 2 * availability * (start*poissonTail(actionRate*duration/90, threshold) + cameo*poissonTail(actionRate*20/90, threshold))
	}
	b["saves"] = 0
	b["penaltySaves"] = 0
	b["goalsConceded"] = 0
	if p.Position == Goalkeeper {
		rate := float64(p.Saves) / denominator / attacking
		b["saves"] = availability * (start*poissonGroups(rate*duration/90, 3) + cameo*poissonGroups(rate*20/90, 3))
		b["penaltySaves"] = 5 * float64(p.PenaltySaves) / denominator * exposure
	}
	if p.Position == Goalkeeper || p.Position == Defender {
		b["goalsConceded"] = -availability * (start*poissonGroups(concededRate*duration/90, 2) + cameo*poissonGroups(concededRate*20/90, 2))
	}
	b["yellowCards"] = -float64(p.Yellow) / denominator * exposure
	b["redCards"] = -3 * float64(p.Red) / denominator * exposure
	b["ownGoals"] = -2 * float64(p.OwnGoals) / denominator * exposure
	b["penaltyMisses"] = -2 * float64(p.PenaltyMisses) / denominator * exposure
	// Fixed accumulation order keeps identical-input rankings deterministic.
	for _, key := range []string{"appearance", "goals", "assists", "cleanSheet", "bonus", "defence", "saves", "penaltySaves", "goalsConceded", "yellowCards", "redCards", "ownGoals", "penaltyMisses"} {
		result.Points += b[key]
	}
	return result
}

func clamp(n, lo, hi float64) float64 { return math.Min(hi, math.Max(lo, n)) }
func poissonTail(mean float64, threshold int) float64 {
	mass := math.Exp(-mean)
	cdf := mass
	for n := 1; n < threshold; n++ {
		mass *= mean / float64(n)
		cdf += mass
	}
	return clamp(1-cdf, 0, 1)
}
func poissonGroups(mean float64, group int) float64 {
	result := 0.0
	for n := group; n <= 120; n += group {
		tail := poissonTail(mean, n)
		result += tail
		if tail < 1e-10 {
			break
		}
	}
	return result
}

func (f Forecast) ByID() map[uuid.UUID]PlayerProjection {
	result := make(map[uuid.UUID]PlayerProjection, len(f.Players))
	for _, p := range f.Players {
		result[p.PlayerID] = p
	}
	return result
}
