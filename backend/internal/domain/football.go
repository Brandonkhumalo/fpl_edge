package domain

import "time"

type FootballData struct {
	Fixtures   []FootballFixture
	Chips      []FootballChip
	SeasonName string
	StartsOn   time.Time
	EndsOn     time.Time
	Gameweeks  []FootballGameweek
	Clubs      []FootballClub
	Players    []FootballPlayer
}

type FootballChip struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Start int    `json:"start_event"`
	End   int    `json:"stop_event"`
}

type FootballGameweek struct {
	ProviderID int
	Number     int
	Deadline   time.Time
	IsCurrent  bool
}

type FootballClub struct {
	ProviderID int
	KitCode    int
	Name       string
	ShortName  string
}

type FootballPlayer struct {
	FootballStats
	SelectedByPercent *float64
	ProviderID        int
	ClubID            int
	SquadNumber       *int
	WebName           string
	FirstName         string
	LastName          string
	Position          Position
	Price             int
	Status            string
	TotalPoints       int
	Form              float64
}

// FootballStats retains provider measurements, not model predictions.
type FootballStats struct {
	Minutes          *int   `json:"minutes"`
	Starts           int    `json:"starts"`
	Goals            int    `json:"goals_scored"`
	Assists          int    `json:"assists"`
	XG               string `json:"expected_goals"`
	XA               string `json:"expected_assists"`
	Bonus            int    `json:"bonus"`
	Saves            int    `json:"saves"`
	PenaltySaves     int    `json:"penalties_saved"`
	PenaltyMisses    int    `json:"penalties_missed"`
	Yellow           int    `json:"yellow_cards"`
	Red              int    `json:"red_cards"`
	OwnGoals         int    `json:"own_goals"`
	DefensiveActions int    `json:"defensive_contribution"`
	Chance           *int   `json:"chance_of_playing_next_round"`
}

type FootballFixture struct {
	ID             int        `json:"id"`
	Gameweek       *int       `json:"event"`
	Home           int        `json:"team_h"`
	Away           int        `json:"team_a"`
	HomeScore      *int       `json:"team_h_score"`
	AwayScore      *int       `json:"team_a_score"`
	HomeDifficulty int        `json:"team_h_difficulty"`
	AwayDifficulty int        `json:"team_a_difficulty"`
	Kickoff        *time.Time `json:"kickoff_time"`
	Finished       bool       `json:"finished"`
	Started        bool       `json:"started"`
}
