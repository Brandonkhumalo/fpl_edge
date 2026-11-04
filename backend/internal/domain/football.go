package domain

import "time"

type FootballData struct {
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
	Name       string
	ShortName  string
}

type FootballPlayer struct {
	SelectedByPercent *float64
	ProviderID        int
	ClubID            int
	FirstName         string
	LastName          string
	Position          Position
	Price             int
	Status            string
	TotalPoints       int
	Form              float64
}
