package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *API) deadline(c echo.Context) error {
	result, err := a.store.Deadline(c.Request().Context(), userID(c))
	if err != nil {
		return adviceError(err)
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusOK, result)
}

func (a *API) deadlineCalendar(c echo.Context) error {
	result, err := a.store.Deadline(c.Request().Context(), userID(c))
	if err != nil {
		return adviceError(err)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	deadline := result.Deadline.UTC().Format("20060102T150405Z")
	body := fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//FPL Edge//Deadline//EN\r\nBEGIN:VEVENT\r\nUID:fpl-edge-gw%d-%s\r\nDTSTAMP:%s\r\nDTSTART:%s\r\nSUMMARY:FPL Gameweek %d deadline\r\nDESCRIPTION:Review your internal FPL Edge plan and submit official changes separately.\r\nBEGIN:VALARM\r\nTRIGGER:-PT24H\r\nACTION:DISPLAY\r\nDESCRIPTION:FPL deadline tomorrow\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", result.Gameweek, userID(c), stamp, deadline, result.Gameweek)
	return c.Blob(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}

func parseID(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, badRequest("valid identifier is required")
	}
	return id, nil
}
func (a *API) watchlist(c echo.Context) error {
	data, err := a.store.Watchlist(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	notices, err := a.store.Notifications(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	events := notices[:0]
	for _, notice := range notices {
		if notice.Category == "watchlist" {
			events = append(events, notice)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data, "events": events})
}
func (a *API) addWatch(c echo.Context) error {
	id, err := parseID(c, "playerId")
	if err != nil {
		return err
	}
	if err = a.store.AddWatch(c.Request().Context(), userID(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
func (a *API) removeWatch(c echo.Context) error {
	id, err := parseID(c, "playerId")
	if err != nil {
		return err
	}
	if err = a.store.RemoveWatch(c.Request().Context(), userID(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
func (a *API) recommendationHistory(c echo.Context) error {
	data, err := a.store.RecommendationHistory(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data})
}
func (a *API) selectRecommendation(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		Action json.RawMessage `json:"action"`
	}
	if err = c.Bind(&body); err != nil || len(body.Action) == 0 {
		return badRequest("action is required")
	}
	if len(body.Action) > 4096 {
		return badRequest("action is too large")
	}
	if err = a.store.SelectRecommendation(c.Request().Context(), userID(c), id, body.Action); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
func (a *API) setRecommendationOutcome(c echo.Context) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		ActualPoints int    `json:"actualPoints"`
		Notes        string `json:"notes"`
	}
	if err = c.Bind(&body); err != nil {
		return badRequest("invalid outcome")
	}
	if body.ActualPoints < 0 {
		return badRequest("actual points cannot be negative")
	}
	body.Notes = strings.TrimSpace(body.Notes)
	if len(body.Notes) > 500 {
		return badRequest("notes must be 500 characters or less")
	}
	if err = a.store.SetRecommendationOutcome(c.Request().Context(), userID(c), id, body.ActualPoints, body.Notes); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
func (a *API) transferPlans(c echo.Context) error {
	data, err := a.store.TransferPlans(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data})
}
func (a *API) generateTransferPlan(c echo.Context) error {
	var body struct {
		Name     string `json:"name"`
		Strategy string `json:"strategy"`
		Horizon  int    `json:"horizon"`
	}
	if err := c.Bind(&body); err != nil {
		return badRequest("invalid plan")
	}
	body.Name = strings.TrimSpace(body.Name)
	if len(body.Name) < 1 || len(body.Name) > 80 {
		return badRequest("name must be between 1 and 80 characters")
	}
	valid := map[string]bool{"conservative": true, "balanced": true, "aggressive": true, "chip_preparation": true, "injury_response": true}
	if !valid[body.Strategy] {
		return badRequest("invalid strategy")
	}
	if body.Horizon < 3 || body.Horizon > 8 {
		return badRequest("horizon must be between 3 and 8")
	}
	result, err := a.store.GenerateTransferPlan(c.Request().Context(), userID(c), body.Name, body.Strategy, body.Horizon)
	if err != nil {
		return adviceError(err)
	}
	return c.JSON(http.StatusCreated, result)
}
