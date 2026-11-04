package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tishanyq/fpl-edge/backend/internal/auth"
	"github.com/tishanyq/fpl-edge/backend/internal/config"
	"github.com/tishanyq/fpl-edge/backend/internal/domain"
	"github.com/tishanyq/fpl-edge/backend/internal/store"
)

const userContextKey = "authenticated_user_id"

type API struct {
	store         *store.Store
	auth          *auth.Service
	log           *slog.Logger
	secureCookies bool
}

func New(cfg config.Config, dataStore *store.Store, authService *auth.Service, logger *slog.Logger) *echo.Echo {
	api := &API{store: dataStore, auth: authService, log: logger, secureCookies: cfg.Environment == "production"}
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover(), middleware.RequestID(), middleware.Secure())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: cfg.CORSOrigins, AllowCredentials: true, AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "Idempotency-Key"}}))
	e.HTTPErrorHandler = api.errorHandler
	e.GET("/health/live", func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	e.GET("/health/ready", api.readiness)

	v1 := e.Group("/api/v1")
	v1.POST("/auth/register", api.register)
	v1.POST("/auth/login", api.login)
	v1.POST("/auth/refresh", api.refresh)
	protected := v1.Group("")
	protected.Use(api.requireAuth)
	protected.POST("/auth/logout", api.logout)
	protected.GET("/auth/me", api.me)
	protected.GET("/seasons/current", api.currentSeason)
	protected.GET("/players", api.players)
	protected.GET("/squad", api.getSquad)
	protected.POST("/squad", api.createSquad)
	protected.POST("/transfers/confirm", api.confirmTransfer)
	protected.POST("/optimizer/lineup", api.optimizeLineup)
	if cfg.StaticDir != "" {
		e.GET("/*", spaHandler(cfg.StaticDir))
	}
	return e
}

func (a *API) readiness(c echo.Context) error {
	if err := a.store.Pool().Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}

type credentialsRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (a *API) register(c echo.Context) error {
	var request credentialsRequest
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid JSON request")
	}
	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if _, err := mail.ParseAddress(request.Email); err != nil {
		return badRequest("a valid email address is required")
	}
	if len(request.DisplayName) < 2 || len(request.DisplayName) > 80 {
		return badRequest("display name must be between 2 and 80 characters")
	}
	hash, err := auth.HashPassword(request.Password)
	if err != nil {
		return badRequest(err.Error())
	}
	user, err := a.store.CreateUser(c.Request().Context(), request.Email, request.DisplayName, hash)
	if errors.Is(err, store.ErrConflict) {
		return echo.NewHTTPError(http.StatusConflict, "an account with this email already exists")
	}
	if err != nil {
		return err
	}
	return a.issueSession(c, user, http.StatusCreated)
}

func (a *API) login(c echo.Context) error {
	var request credentialsRequest
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid JSON request")
	}
	user, err := a.store.UserByEmail(c.Request().Context(), request.Email)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, request.Password) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}
	return a.issueSession(c, user, http.StatusOK)
}

func (a *API) issueSession(c echo.Context, user store.User, status int) error {
	sessionID := uuid.New()
	access, refresh, refreshHash, expires, err := a.auth.NewSessionTokens(user.ID, sessionID)
	if err != nil {
		return err
	}
	if err = a.store.CreateSession(c.Request().Context(), sessionID, user.ID, refreshHash, expires, c.Request().UserAgent(), c.RealIP()); err != nil {
		return err
	}
	a.setRefreshCookie(c, refresh, expires)
	return c.JSON(status, map[string]any{"user": user, "accessToken": access, "expiresIn": a.auth.AccessTokenTTLSeconds()})
}

func (a *API) refresh(c echo.Context) error {
	cookie, err := c.Cookie("fpl_edge_refresh")
	if err != nil || cookie.Value == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "refresh session is required")
	}
	oldID, err := auth.RefreshSessionID(cookie.Value)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}
	newID := uuid.New()
	// Generate a candidate refresh token; its access token is discarded until rotation succeeds.
	_, newRefresh, newHash, expires, err := a.auth.NewSessionTokens(uuid.Nil, newID)
	if err != nil {
		return err
	}
	userID, err := a.store.RotateSession(c.Request().Context(), oldID, newID, auth.HashRefreshToken(cookie.Value), newHash, expires, c.Request().UserAgent(), c.RealIP())
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}
	access, _, _, _, err := a.auth.NewSessionTokens(userID, newID)
	if err != nil {
		return err
	}
	a.setRefreshCookie(c, newRefresh, expires)
	return c.JSON(http.StatusOK, map[string]any{"accessToken": access, "expiresIn": a.auth.AccessTokenTTLSeconds()})
}

func (a *API) requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get(echo.HeaderAuthorization)
		scheme, raw, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || raw == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
		}
		userID, sessionID, err := a.auth.ParseAccessToken(raw)
		if err != nil || !a.store.SessionActive(c.Request().Context(), sessionID, userID) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired session")
		}
		c.Set(userContextKey, userID)
		c.Set("session_id", sessionID)
		return next(c)
	}
}

func (a *API) logout(c echo.Context) error {
	if err := a.store.RevokeSession(c.Request().Context(), c.Get("session_id").(uuid.UUID), userID(c)); err != nil {
		return err
	}
	a.setRefreshCookie(c, "", time.Unix(0, 0))
	return c.NoContent(http.StatusNoContent)
}
func (a *API) me(c echo.Context) error {
	user, err := a.store.UserByID(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

func (a *API) currentSeason(c echo.Context) error {
	season, err := a.store.CurrentSeason(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, season)
}

func (a *API) players(c echo.Context) error {
	seasonID, err := uuid.Parse(c.QueryParam("seasonId"))
	if err != nil {
		return badRequest("valid seasonId is required")
	}
	maxPrice := 0
	if raw := c.QueryParam("maxPrice"); raw != "" {
		if _, err := fmtSscan(raw, &maxPrice); err != nil {
			return badRequest("maxPrice must be an integer")
		}
	}
	players, err := a.store.ListPlayers(c.Request().Context(), seasonID, c.QueryParam("position"), maxPrice)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": players})
}

type createSquadRequest struct {
	SeasonID        uuid.UUID   `json:"seasonId"`
	CurrentGameweek int         `json:"currentGameweek"`
	Bank            int         `json:"bank"`
	FreeTransfers   int         `json:"freeTransfers"`
	PlayerIDs       []uuid.UUID `json:"playerIds"`
	Starters        []uuid.UUID `json:"starters"`
	Bench           []uuid.UUID `json:"bench"`
	CaptainID       uuid.UUID   `json:"captainId"`
	ViceCaptainID   uuid.UUID   `json:"viceCaptainId"`
}

func (a *API) createSquad(c echo.Context) error {
	var request createSquadRequest
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid squad request")
	}
	players, err := a.store.PlayersByIDs(c.Request().Context(), request.SeasonID, request.PlayerIDs)
	if err != nil {
		return badRequest(err.Error())
	}
	for i := range players {
		players[i].SellingPrice = domain.SellingPrice(players[i].PurchasePrice, players[i].CurrentPrice)
	}
	squad, err := a.store.CreateSquad(c.Request().Context(), userID(c), store.CreateSquadInput{SeasonID: request.SeasonID, CurrentGameweek: request.CurrentGameweek, Bank: request.Bank, FreeTransfers: request.FreeTransfers, Players: players, Starters: request.Starters, Bench: request.Bench, CaptainID: request.CaptainID, ViceCaptainID: request.ViceCaptainID})
	if err != nil {
		return badRequest(err.Error())
	}
	return c.JSON(http.StatusCreated, squad)
}

func (a *API) getSquad(c echo.Context) error {
	squad, err := a.store.Squad(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, squad)
}

func (a *API) confirmTransfer(c echo.Context) error {
	var request struct {
		ExpectedRevision int64     `json:"expectedRevision"`
		PlayerOutID      uuid.UUID `json:"playerOutId"`
		PlayerInID       uuid.UUID `json:"playerInId"`
	}
	if err := c.Bind(&request); err != nil {
		return badRequest("invalid transfer request")
	}
	squad, err := a.store.ConfirmTransfer(c.Request().Context(), userID(c), store.ConfirmTransferInput{ExpectedRevision: request.ExpectedRevision, PlayerOutID: request.PlayerOutID, PlayerInID: request.PlayerInID, IdempotencyKey: c.Request().Header.Get("Idempotency-Key")})
	if errors.Is(err, store.ErrRevisionConflict) {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if errors.Is(err, store.ErrConflict) {
		return echo.NewHTTPError(http.StatusConflict, "transfer request was already processed")
	}
	if err != nil {
		return badRequest(err.Error())
	}
	return c.JSON(http.StatusOK, squad)
}

func (a *API) optimizeLineup(c echo.Context) error {
	squad, err := a.store.Squad(c.Request().Context(), userID(c))
	if err != nil {
		return err
	}
	starters, bench, captain, vice, err := domain.OptimizeLineup(squad.Players)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"squadRevision": squad.Revision, "starters": starters, "bench": bench, "captainId": captain, "viceCaptainId": vice})
}

func (a *API) errorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	status := http.StatusInternalServerError
	message := "internal server error"
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) {
		status = httpError.Code
		if text, ok := httpError.Message.(string); ok {
			message = text
		}
	}
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
		message = "resource not found"
	}
	if status >= 500 {
		a.log.Error("request failed", "request_id", c.Response().Header().Get(echo.HeaderXRequestID), "error", err)
	}
	_ = c.JSON(status, map[string]any{"error": map[string]any{"code": http.StatusText(status), "message": message, "requestId": c.Response().Header().Get(echo.HeaderXRequestID)}})
}

func userID(c echo.Context) uuid.UUID { return c.Get(userContextKey).(uuid.UUID) }
func badRequest(message string) *echo.HTTPError {
	return echo.NewHTTPError(http.StatusBadRequest, message)
}

// fmtSscan keeps input parsing in one place and avoids accepting decimal prices.
func fmtSscan(value string, target *int) (int, error) {
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, errors.New("not an integer")
		}
		parsed = parsed*10 + int(r-'0')
	}
	*target = parsed
	return 1, nil
}

func (a *API) setRefreshCookie(c echo.Context, value string, expires time.Time) {
	c.SetCookie(&http.Cookie{
		Name: "fpl_edge_refresh", Value: value, Path: "/api/v1/auth",
		Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
		HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode,
	})
}
