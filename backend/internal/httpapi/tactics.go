package httpapi

import (
	"errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tishanyq/fpl-edge/backend/internal/store"
	"net/http"
)

func tacticsResponse(err error) error {
	if errors.Is(err, store.ErrRevisionConflict) {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if errors.Is(err, store.ErrTactics) {
		return badRequest(err.Error())
	}
	return err
}
func (a *API) tactics(c echo.Context) error {
	data, err := a.store.Tactics(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, data)
}
func (a *API) captaincy(c echo.Context) error {
	var request struct {
		Revision int64     `json:"expectedRevision"`
		Captain  uuid.UUID `json:"captainId"`
		Vice     uuid.UUID `json:"viceCaptainId"`
	}
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid captaincy request")
	}
	if err := a.store.SetCaptaincy(c.Request().Context(), userID(c), request.Revision, request.Captain, request.Vice); err != nil {
		return tacticsResponse(err)
	}
	return a.getSquad(c)
}
func (a *API) chip(c echo.Context) error {
	var request struct {
		Revision int64 `json:"expectedRevision"`
		ChipID   int   `json:"chipId"`
		Cancel   bool  `json:"cancel"`
	}
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid chip request")
	}
	if err := a.store.SetChip(c.Request().Context(), userID(c), request.Revision, request.ChipID, request.Cancel); err != nil {
		return tacticsResponse(err)
	}
	return a.tactics(c)
}

func (a *API) chipHistory(c echo.Context) error {
	var request struct {
		Revision int64                  `json:"expectedRevision"`
		Usages   []store.PriorChipUsage `json:"usages"`
	}
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid chip history request")
	}
	if err := a.store.SetChipHistory(c.Request().Context(), userID(c), request.Revision, request.Usages); err != nil {
		return tacticsResponse(err)
	}
	return a.tactics(c)
}
