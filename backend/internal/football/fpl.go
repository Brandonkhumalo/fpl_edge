package football

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

const (
	defaultBootstrapURL    = "https://fantasy.premierleague.com/api/bootstrap-static/"
	defaultSquadNumbersURL = "https://www.premierleague.com/en/news/4725682/all-20-premier-league-clubs-squad-numbers-for-202627-season"
	maxResponseBytes       = 20 << 20
)

var (
	ErrManagerNotFound     = errors.New("FPL manager or team ID was not found")
	ErrProviderRateLimit   = errors.New("official FPL service rate limit reached")
	ErrProviderUnavailable = errors.New("official FPL service is unavailable")
)

type Client struct {
	url             string
	squadNumbersURL string
	httpClient      *http.Client
}

func NewClient() *Client {
	return &Client{
		url:             defaultBootstrapURL,
		squadNumbersURL: defaultSquadNumbersURL,
		httpClient:      &http.Client{Timeout: 20 * time.Second},
	}
}

// FetchOfficialSquad reads only public FPL endpoints. It never accepts or sends
// an FPL password, session cookie, or authenticated account credential.
func (c *Client) FetchOfficialSquad(ctx context.Context, managerID int64) (domain.OfficialSquad, error) {
	if managerID <= 0 {
		return domain.OfficialSquad{}, ErrManagerNotFound
	}
	base := strings.TrimSuffix(c.url, "bootstrap-static/")
	var entry struct {
		ID              int64  `json:"id"`
		CurrentEvent    int    `json:"current_event"`
		PlayerFirstName string `json:"player_first_name"`
		PlayerLastName  string `json:"player_last_name"`
		Name            string `json:"name"`
	}
	if err := c.fetchJSON(ctx, fmt.Sprintf("%sentry/%d/", base, managerID), &entry); err != nil {
		return domain.OfficialSquad{}, err
	}
	if entry.ID != managerID || entry.CurrentEvent < 1 || entry.CurrentEvent > 60 {
		return domain.OfficialSquad{}, errors.New("official squad is not available for a current gameweek")
	}
	var picks struct {
		ActiveChip string `json:"active_chip"`
		History    struct {
			Event int `json:"event"`
			Bank  int `json:"bank"`
			Value int `json:"value"`
		} `json:"entry_history"`
		Picks []struct {
			Element     int  `json:"element"`
			Position    int  `json:"position"`
			Multiplier  int  `json:"multiplier"`
			Captain     bool `json:"is_captain"`
			ViceCaptain bool `json:"is_vice_captain"`
		} `json:"picks"`
	}
	if err := c.fetchJSON(ctx, fmt.Sprintf("%sentry/%d/event/%d/picks/", base, managerID, entry.CurrentEvent), &picks); err != nil {
		return domain.OfficialSquad{}, err
	}
	var history struct {
		Current []struct {
			Event        int `json:"event"`
			Points       int `json:"points"`
			TotalPoints  int `json:"total_points"`
			OverallRank  int `json:"overall_rank"`
			Bank         int `json:"bank"`
			Value        int `json:"value"`
			Transfers    int `json:"event_transfers"`
			TransferCost int `json:"event_transfers_cost"`
			BenchPoints  int `json:"points_on_bench"`
		} `json:"current"`
		Past []struct {
			Name        string `json:"season_name"`
			TotalPoints int    `json:"total_points"`
			Rank        int    `json:"rank"`
		} `json:"past"`
		Chips []struct {
			Name  string `json:"name"`
			Event int    `json:"event"`
		} `json:"chips"`
	}
	if err := c.fetchJSON(ctx, fmt.Sprintf("%sentry/%d/history/", base, managerID), &history); err != nil {
		return domain.OfficialSquad{}, err
	}
	if len(picks.Picks) != 15 {
		return domain.OfficialSquad{}, errors.New("official squad does not contain 15 players")
	}
	result := domain.OfficialSquad{
		ManagerID: managerID, ManagerName: strings.TrimSpace(entry.PlayerFirstName + " " + entry.PlayerLastName),
		TeamName: strings.TrimSpace(entry.Name), Gameweek: picks.History.Event, Bank: picks.History.Bank,
		TeamValue: picks.History.Value, ActiveChip: picks.ActiveChip, FetchedAt: time.Now().UTC(),
		FreeTransfersEstimated: true,
	}
	seen := make(map[int]bool, 15)
	seenPositions := make(map[int]bool, 15)
	for _, pick := range picks.Picks {
		if pick.Element <= 0 || pick.Position < 1 || pick.Position > 15 || seen[pick.Element] || seenPositions[pick.Position] {
			return domain.OfficialSquad{}, errors.New("official squad contains invalid player selections")
		}
		seen[pick.Element] = true
		seenPositions[pick.Position] = true
		result.Picks = append(result.Picks, domain.OfficialPick{ProviderPlayerID: pick.Element, Position: pick.Position, Multiplier: pick.Multiplier, Captain: pick.Captain, ViceCaptain: pick.ViceCaptain})
	}
	for _, item := range history.Current {
		if item.Event < 1 || item.Event > 60 || item.Bank < 0 || item.Value < 0 || item.Transfers < 0 || item.TransferCost < 0 {
			return domain.OfficialSquad{}, errors.New("official gameweek history contains invalid values")
		}
		result.GameweekHistory = append(result.GameweekHistory, domain.OfficialGameweek{Gameweek: item.Event, Points: item.Points, TotalPoints: item.TotalPoints, OverallRank: item.OverallRank, Bank: item.Bank, TeamValue: item.Value, Transfers: item.Transfers, TransferCost: item.TransferCost, BenchPoints: item.BenchPoints})
	}
	for _, item := range history.Past {
		result.PastSeasons = append(result.PastSeasons, domain.OfficialPastSeason{Name: item.Name, TotalPoints: item.TotalPoints, Rank: item.Rank})
	}
	for _, item := range history.Chips {
		if item.Event < 1 || item.Event > 60 || (item.Name != "wildcard" && item.Name != "freehit" && item.Name != "bboost" && item.Name != "3xc") {
			return domain.OfficialSquad{}, errors.New("official chip history contains invalid values")
		}
		result.Chips = append(result.Chips, domain.OfficialChipUsage{Name: item.Name, Gameweek: item.Event})
	}
	sort.Slice(result.GameweekHistory, func(i, j int) bool { return result.GameweekHistory[i].Gameweek < result.GameweekHistory[j].Gameweek })
	result.FreeTransfers = estimateFreeTransfers(result.GameweekHistory, result.Chips)
	return result, nil
}

func (c *Client) fetchJSON(ctx context.Context, url string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "FPL-Edge/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrManagerNotFound
	case http.StatusTooManyRequests:
		return ErrProviderRateLimit
	default:
		return fmt.Errorf("%w: HTTP %d", ErrProviderUnavailable, response.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target); err != nil {
		return fmt.Errorf("%w: invalid response", ErrProviderUnavailable)
	}
	return nil
}

func estimateFreeTransfers(history []domain.OfficialGameweek, chips []domain.OfficialChipUsage) int {
	chipByGameweek := make(map[int]string, len(chips))
	for _, chip := range chips {
		chipByGameweek[chip.Gameweek] = chip.Name
	}
	available := 1
	for _, gameweek := range history {
		chip := chipByGameweek[gameweek.Gameweek]
		if chip != "wildcard" && chip != "freehit" {
			used := gameweek.Transfers
			if used > available {
				used = available
			}
			available -= used
		}
		if available < 5 {
			available++
		}
	}
	return available
}

func (c *Client) Fetch(ctx context.Context) (domain.FootballData, error) {
	data, err := c.fetchBootstrap(ctx)
	if err != nil {
		return data, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(c.url, "bootstrap-static/")+"fixtures/", nil)
	if err != nil {
		return domain.FootballData{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "FPL-Edge/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.FootballData{}, fmt.Errorf("fetch fixtures: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return domain.FootballData{}, fmt.Errorf("fixtures returned HTTP %d", response.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&data.Fixtures); err != nil {
		return domain.FootballData{}, fmt.Errorf("decode fixtures: %w", err)
	}
	if err = domain.ValidateForecastData(data); err != nil {
		return domain.FootballData{}, err
	}
	// Shirt numbers are display metadata from the official Premier League site.
	// A metadata outage must not reject otherwise valid football/statistical data.
	_ = c.enrichSquadNumbers(ctx, &data)
	return data, nil
}

func (c *Client) FetchLive(ctx context.Context, gameweek int) (domain.LiveGameweek, error) {
	if gameweek < 1 || gameweek > 60 {
		return domain.LiveGameweek{}, errors.New("invalid live gameweek")
	}
	base := strings.TrimSuffix(c.url, "bootstrap-static/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%sevent/%d/live/", base, gameweek), nil)
	if err != nil {
		return domain.LiveGameweek{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "FPL-Edge/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.LiveGameweek{}, fmt.Errorf("fetch live gameweek: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return domain.LiveGameweek{}, fmt.Errorf("live gameweek returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Elements []struct {
			ID    int `json:"id"`
			Stats struct {
				Minutes               int `json:"minutes"`
				TotalPoints           int `json:"total_points"`
				Bonus                 int `json:"bonus"`
				BPS                   int `json:"bps"`
				DefensiveContribution int `json:"defensive_contribution"`
				Goals                 int `json:"goals_scored"`
				Assists               int `json:"assists"`
				CleanSheets           int `json:"clean_sheets"`
				GoalsConceded         int `json:"goals_conceded"`
				Saves                 int `json:"saves"`
				YellowCards           int `json:"yellow_cards"`
				RedCards              int `json:"red_cards"`
			} `json:"stats"`
		} `json:"elements"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return domain.LiveGameweek{}, fmt.Errorf("decode live gameweek: %w", err)
	}
	if len(payload.Elements) == 0 {
		return domain.LiveGameweek{}, errors.New("live gameweek contains no players")
	}
	result := domain.LiveGameweek{Gameweek: gameweek, FetchedAt: time.Now().UTC(), Players: make([]domain.LivePlayer, 0, len(payload.Elements))}
	seen := make(map[int]bool, len(payload.Elements))
	for _, element := range payload.Elements {
		if element.ID <= 0 || seen[element.ID] || element.Stats.Minutes < 0 || element.Stats.Minutes > 300 {
			return domain.LiveGameweek{}, errors.New("invalid live player data")
		}
		seen[element.ID] = true
		result.Players = append(result.Players, domain.LivePlayer{ProviderID: element.ID, Minutes: element.Stats.Minutes, TotalPoints: element.Stats.TotalPoints, Bonus: element.Stats.Bonus, BPS: element.Stats.BPS, DefensiveContribution: element.Stats.DefensiveContribution, Goals: element.Stats.Goals, Assists: element.Stats.Assists, CleanSheets: element.Stats.CleanSheets, GoalsConceded: element.Stats.GoalsConceded, Saves: element.Stats.Saves, YellowCards: element.Stats.YellowCards, RedCards: element.Stats.RedCards})
	}
	fixtureRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%sfixtures/?event=%d", base, gameweek), nil)
	if err != nil {
		return domain.LiveGameweek{}, err
	}
	fixtureRequest.Header.Set("Accept", "application/json")
	fixtureRequest.Header.Set("User-Agent", "FPL-Edge/1.0")
	fixtureResponse, err := c.httpClient.Do(fixtureRequest)
	if err != nil {
		return domain.LiveGameweek{}, fmt.Errorf("fetch live fixtures: %w", err)
	}
	defer fixtureResponse.Body.Close()
	if fixtureResponse.StatusCode != http.StatusOK {
		return domain.LiveGameweek{}, fmt.Errorf("live fixtures returned HTTP %d", fixtureResponse.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(fixtureResponse.Body, maxResponseBytes)).Decode(&result.Fixtures); err != nil {
		return domain.LiveGameweek{}, fmt.Errorf("decode live fixtures: %w", err)
	}
	if len(result.Fixtures) == 0 {
		return domain.LiveGameweek{}, errors.New("live gameweek contains no fixtures")
	}
	return result, nil
}

func (c *Client) fetchBootstrap(ctx context.Context) (domain.FootballData, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return domain.FootballData{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "FPL-Edge/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.FootballData{}, fmt.Errorf("fetch FPL bootstrap data: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return domain.FootballData{}, fmt.Errorf("FPL bootstrap returned HTTP %d", response.StatusCode)
	}
	return decode(io.LimitReader(response.Body, maxResponseBytes))
}

type bootstrap struct {
	Chips  []domain.FootballChip `json:"chips"`
	Events []struct {
		ID           int       `json:"id"`
		DeadlineTime time.Time `json:"deadline_time"`
		IsCurrent    bool      `json:"is_current"`
	} `json:"events"`
	Teams []struct {
		ID        int    `json:"id"`
		Code      int    `json:"code"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	} `json:"teams"`
	ElementTypes []struct {
		ID        int             `json:"id"`
		ShortName domain.Position `json:"singular_name_short"`
	} `json:"element_types"`
	Elements []struct {
		domain.FootballStats
		SelectedByPercent string `json:"selected_by_percent"`
		SquadNumber       *int   `json:"squad_number"`
		ID                int    `json:"id"`
		WebName           string `json:"web_name"`
		FirstName         string `json:"first_name"`
		SecondName        string `json:"second_name"`
		Team              int    `json:"team"`
		ElementType       int    `json:"element_type"`
		NowCost           int    `json:"now_cost"`
		Status            string `json:"status"`
		TotalPoints       int    `json:"total_points"`
		Form              string `json:"form"`
	} `json:"elements"`
}

func decode(reader io.Reader) (domain.FootballData, error) {
	var payload bootstrap
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&payload); err != nil {
		return domain.FootballData{}, fmt.Errorf("decode FPL bootstrap data: %w", err)
	}
	if len(payload.Events) == 0 || len(payload.Teams) == 0 || len(payload.Elements) == 0 {
		return domain.FootballData{}, errors.New("FPL bootstrap data is incomplete")
	}

	positions := make(map[int]domain.Position, len(payload.ElementTypes))
	for _, item := range payload.ElementTypes {
		switch item.ShortName {
		case domain.Goalkeeper, domain.Defender, domain.Midfielder, domain.Forward:
			positions[item.ID] = item.ShortName
		}
	}
	if len(positions) != 4 {
		return domain.FootballData{}, errors.New("FPL bootstrap positions are incomplete")
	}

	firstDeadline := payload.Events[0].DeadlineTime
	lastDeadline := payload.Events[len(payload.Events)-1].DeadlineTime
	data := domain.FootballData{
		Chips:      payload.Chips,
		SeasonName: fmt.Sprintf("%d/%02d", firstDeadline.Year(), lastDeadline.Year()%100),
		StartsOn:   firstDeadline.AddDate(0, 0, -7),
		EndsOn:     lastDeadline.AddDate(0, 0, 7),
		Gameweeks:  make([]domain.FootballGameweek, 0, len(payload.Events)),
		Clubs:      make([]domain.FootballClub, 0, len(payload.Teams)),
		Players:    make([]domain.FootballPlayer, 0, len(payload.Elements)),
	}
	for _, event := range payload.Events {
		data.Gameweeks = append(data.Gameweeks, domain.FootballGameweek{ProviderID: event.ID, Number: event.ID, Deadline: event.DeadlineTime, IsCurrent: event.IsCurrent})
	}
	for _, team := range payload.Teams {
		data.Clubs = append(data.Clubs, domain.FootballClub{ProviderID: team.ID, KitCode: team.Code, Name: team.Name, ShortName: team.ShortName})
	}
	for _, player := range payload.Elements {
		position, ok := positions[player.ElementType]
		if !ok || player.NowCost <= 0 {
			continue
		}
		form, _ := strconv.ParseFloat(player.Form, 64)
		var selectedByPercent *float64
		if player.SelectedByPercent != "" {
			value, err := strconv.ParseFloat(player.SelectedByPercent, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
				return domain.FootballData{}, fmt.Errorf("invalid selection percentage for player %d", player.ID)
			}
			selectedByPercent = &value
		}
		data.Players = append(data.Players, domain.FootballPlayer{
			FootballStats:     player.FootballStats,
			SelectedByPercent: selectedByPercent,
			ProviderID:        player.ID, ClubID: player.Team, SquadNumber: player.SquadNumber, WebName: player.WebName, FirstName: player.FirstName,
			LastName: player.SecondName, Position: position, Price: player.NowCost,
			Status: player.Status, TotalPoints: player.TotalPoints, Form: form,
		})
	}
	if len(data.Players) == 0 {
		return domain.FootballData{}, errors.New("FPL bootstrap contains no eligible players")
	}
	return data, nil
}
