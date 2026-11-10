package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/tishanyq/fpl-edge/backend/internal/store"
)

func (a *API) liveGameweek(c echo.Context) error {
	result, err := a.store.LiveGameweek(c.Request().Context(), userID(c))
	if errors.Is(err, store.ErrLiveUnavailable) {
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}
	if errors.Is(err, store.ErrLiveSelectionUnavailable) {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusOK, result)
}

func (a *API) askAdvisor(c echo.Context) error {
	var request struct {
		Question         string `json:"question"`
		UseLanguageModel bool   `json:"useLanguageModel"`
	}
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid advisor request")
	}
	request.Question = strings.TrimSpace(request.Question)
	if utf8.RuneCountInString(request.Question) < 3 || utf8.RuneCountInString(request.Question) > 500 {
		return badRequest("question must be between 3 and 500 characters")
	}
	allowed, err := a.store.AdvisorRequestAllowed(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusTooManyRequests, "advisor limit reached; try again later")
	}
	grounding, err := a.store.AdvisorGrounding(c.Request().Context(), userID(c), request.Question)
	if err != nil {
		return adviceError(err)
	}
	selection := a.advisor.Select(c.Request().Context(), request.Question, grounding.Evidence, request.UseLanguageModel)
	answer, err := a.store.SaveAdvisorAnswer(c.Request().Context(), userID(c), request.Question, grounding, selection)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.JSON(http.StatusOK, answer)
}
