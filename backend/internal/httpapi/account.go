package httpapi

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
	"time"
)

type preferences struct {
	DisplayName           string `json:"displayName"`
	Strategy              string `json:"strategy"`
	Horizon               int    `json:"planningHorizon"`
	Timezone              string `json:"timezone"`
	DeadlineReminderHours int    `json:"deadlineReminderHours"`
	InAppAlerts           bool   `json:"inAppAlerts"`
	EmailAlerts           bool   `json:"emailAlerts"`
	WebPushAlerts         bool   `json:"webPushAlerts"`
}

func (a *API) preferences(c echo.Context) error {
	var data preferences
	err := a.store.Pool().QueryRow(c.Request().Context(), `SELECT u.display_name,COALESCE(p.strategy,'balanced'),COALESCE(p.planning_horizon,6),COALESCE(p.timezone,'UTC'),COALESCE(p.deadline_reminder_hours,24),COALESCE(p.in_app_alerts,true),COALESCE(p.email_alerts,false),COALESCE(p.web_push_alerts,false) FROM users u LEFT JOIN user_preferences p ON p.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL`, userID(c)).Scan(&data.DisplayName, &data.Strategy, &data.Horizon, &data.Timezone, &data.DeadlineReminderHours, &data.InAppAlerts, &data.EmailAlerts, &data.WebPushAlerts)
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
	data.Timezone = strings.TrimSpace(data.Timezone)
	if _, err := time.LoadLocation(data.Timezone); err != nil {
		return badRequest("timezone must be a valid IANA timezone")
	}
	if data.DeadlineReminderHours < 1 || data.DeadlineReminderHours > 168 {
		return badRequest("deadline reminder must be between 1 and 168 hours")
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
	if _, err = tx.Exec(ctx, `INSERT INTO user_preferences(user_id,strategy,planning_horizon,timezone,deadline_reminder_hours,in_app_alerts,email_alerts,web_push_alerts) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(user_id) DO UPDATE SET strategy=EXCLUDED.strategy,planning_horizon=EXCLUDED.planning_horizon,timezone=EXCLUDED.timezone,deadline_reminder_hours=EXCLUDED.deadline_reminder_hours,in_app_alerts=EXCLUDED.in_app_alerts,email_alerts=EXCLUDED.email_alerts,web_push_alerts=EXCLUDED.web_push_alerts,updated_at=now()`, userID(c), data.Strategy, data.Horizon, data.Timezone, data.DeadlineReminderHours, data.InAppAlerts, data.EmailAlerts, data.WebPushAlerts); err != nil {
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
