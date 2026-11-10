package domain

import "time"

type OfficialSquad struct {
	ManagerID              int64                `json:"managerId"`
	ManagerName            string               `json:"managerName"`
	TeamName               string               `json:"teamName"`
	Gameweek               int                  `json:"gameweek"`
	Bank                   int                  `json:"bank"`
	TeamValue              int                  `json:"teamValue"`
	FreeTransfers          int                  `json:"freeTransfers"`
	FreeTransfersEstimated bool                 `json:"freeTransfersEstimated"`
	ActiveChip             string               `json:"activeChip"`
	Picks                  []OfficialPick       `json:"picks"`
	GameweekHistory        []OfficialGameweek   `json:"gameweekHistory"`
	PastSeasons            []OfficialPastSeason `json:"pastSeasons"`
	Chips                  []OfficialChipUsage  `json:"chips"`
	FetchedAt              time.Time            `json:"fetchedAt"`
}

type OfficialPick struct {
	ProviderPlayerID int  `json:"providerPlayerId"`
	Position         int  `json:"position"`
	Multiplier       int  `json:"multiplier"`
	Captain          bool `json:"captain"`
	ViceCaptain      bool `json:"viceCaptain"`
}

type OfficialGameweek struct {
	Gameweek     int `json:"gameweek"`
	Points       int `json:"points"`
	TotalPoints  int `json:"totalPoints"`
	OverallRank  int `json:"overallRank"`
	Bank         int `json:"bank"`
	TeamValue    int `json:"teamValue"`
	Transfers    int `json:"transfers"`
	TransferCost int `json:"transferCost"`
	BenchPoints  int `json:"benchPoints"`
}

type OfficialPastSeason struct {
	Name        string `json:"name"`
	TotalPoints int    `json:"totalPoints"`
	Rank        int    `json:"rank"`
}

type OfficialChipUsage struct {
	Name     string `json:"name"`
	Gameweek int    `json:"gameweek"`
}
