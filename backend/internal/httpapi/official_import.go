package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tishanyq/fpl-edge/backend/internal/football"
	"github.com/tishanyq/fpl-edge/backend/internal/store"
)

func (a *API) officialSquadLink(c echo.Context) error {
	result, err := a.store.OfficialSquadLink(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusOK, result)
}

func (a *API) previewOfficialSquad(c echo.Context) error {
	var request struct {
		ManagerID int64 `json:"managerId"`
	}
	if err := c.Bind(&request); err != nil || request.ManagerID <= 0 || request.ManagerID > 9_999_999_999 {
		return badRequest("enter a valid public FPL manager or team ID")
	}
	importID, err := a.store.StartOfficialImport(c.Request().Context(), userID(c), request.ManagerID)
	if errors.Is(err, store.ErrOfficialImportRateLimit) {
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	}
	if err != nil {
		return err
	}
	official, err := a.publicSquads.FetchOfficialSquad(c.Request().Context(), request.ManagerID)
	if err != nil {
		code := "provider_unavailable"
		status := http.StatusBadGateway
		message := "official FPL service is unavailable; try again later"
		switch {
		case errors.Is(err, football.ErrManagerNotFound):
			code, status, message = "manager_not_found", http.StatusNotFound, football.ErrManagerNotFound.Error()
		case errors.Is(err, football.ErrProviderRateLimit):
			code, status, message = "provider_rate_limit", http.StatusServiceUnavailable, "official FPL service is busy; try again later"
		}
		a.store.FailOfficialImport(c.Request().Context(), userID(c), importID, code)
		return echo.NewHTTPError(status, message)
	}
	preview, err := a.store.SaveOfficialImportPreview(c.Request().Context(), userID(c), importID, official)
	if err != nil {
		a.store.FailOfficialImport(c.Request().Context(), userID(c), importID, "preview_failed")
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusCreated, preview)
}

func (a *API) applyOfficialSquad(c echo.Context) error {
	importID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest("valid import preview ID is required")
	}
	var request struct {
		ExpectedRevision int64 `json:"expectedRevision"`
	}
	if err = c.Bind(&request); err != nil || request.ExpectedRevision < 0 {
		return badRequest("valid expected squad revision is required")
	}
	squad, err := a.store.ApplyOfficialImport(c.Request().Context(), userID(c), importID, request.ExpectedRevision)
	if errors.Is(err, store.ErrRevisionConflict) {
		return echo.NewHTTPError(http.StatusConflict, "Your squad changed after this preview. Fetch a fresh comparison.")
	}
	if errors.Is(err, store.ErrOfficialImportExpired) {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusOK, squad)
}
