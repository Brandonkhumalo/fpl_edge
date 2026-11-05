package football

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tishanyq/fpl-edge/backend/internal/domain"
)

const (
	defaultBootstrapURL = "https://fantasy.premierleague.com/api/bootstrap-static/"
	maxResponseBytes    = 20 << 20
)

type Client struct {
	url        string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		url:        defaultBootstrapURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
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
	return data, nil
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
		ID                int    `json:"id"`
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
		data.Clubs = append(data.Clubs, domain.FootballClub{ProviderID: team.ID, Name: team.Name, ShortName: team.ShortName})
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
			ProviderID:        player.ID, ClubID: player.Team, FirstName: player.FirstName,
			LastName: player.SecondName, Position: position, Price: player.NowCost,
			Status: player.Status, TotalPoints: player.TotalPoints, Form: form,
		})
	}
	if len(data.Players) == 0 {
		return domain.FootballData{}, errors.New("FPL bootstrap contains no eligible players")
	}
	return data, nil
}
