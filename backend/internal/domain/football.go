package domain

import "time"

type FootballData struct {
	SeasonName string
	StartsOn   time.Time
	EndsOn     time.Time
	Gameweeks  []FootballGameweek
	Clubs      []FootballClub
	Players    []FootballPlayer
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
	ProviderID  int
	ClubID      int
	FirstName   string
	LastName    string
	Position    Position
	Price       int
	Status      string
	TotalPoints int
	Form        float64
}
