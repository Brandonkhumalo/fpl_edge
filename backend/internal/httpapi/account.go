package httpapi

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
	"time"
)

type preferences struct {
	DisplayName string `json:"displayName"`
	Strategy    string `json:"strategy"`
	Horizon     int    `json:"planningHorizon"`
}

func (a *API) preferences(c echo.Context) error {
	var data preferences
	err := a.store.Pool().QueryRow(c.Request().Context(), `SELECT u.display_name,COALESCE(p.strategy,'balanced'),COALESCE(p.planning_horizon,6) FROM users u LEFT JOIN user_preferences p ON p.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL`, userID(c)).Scan(&data.DisplayName, &data.Strategy, &data.Horizon)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, data)
}
func (a *API) updatePreferences(c echo.Context) error {
	var data preferences
	if err := c.Bind(&data); err != nil {
		return badRequest("invalid settings")
	}
	data.DisplayName = strings.TrimSpace(data.DisplayName)
	if len(data.DisplayName) < 2 || len(data.DisplayName) > 80 {
		return badRequest("display name must be between 2 and 80 characters")
	}
	if data.Strategy != "balanced" && data.Strategy != "conservative" && data.Strategy != "aggressive" {
		return badRequest("invalid strategy")
	}
	if data.Horizon < 1 || data.Horizon > 8 {
		return badRequest("planning horizon must be between 1 and 8")
	}
	ctx := c.Request().Context()
	tx, err := a.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,updated_at=now() WHERE id=$1`, userID(c), data.DisplayName); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_preferences(user_id,strategy,planning_horizon) VALUES($1,$2,$3) ON CONFLICT(user_id) DO UPDATE SET strategy=EXCLUDED.strategy,planning_horizon=EXCLUDED.planning_horizon,updated_at=now()`, userID(c), data.Strategy, data.Horizon); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, data)
}
func (a *API) history(c echo.Context) error {
	rows, err := a.store.Pool().Query(c.Request().Context(), `SELECT event_type,revision,created_at FROM squad_events WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, userID(c))
	if err != nil {
		return err
	}
	defer rows.Close()
	type event struct {
		Type     string    `json:"type"`
		Revision int64     `json:"revision"`
		At       time.Time `json:"at"`
	}
	events := []event{}
	for rows.Next() {
		var entry event
		if err = rows.Scan(&entry.Type, &entry.Revision, &entry.At); err != nil {
			return err
		}
		events = append(events, entry)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, events)
}
