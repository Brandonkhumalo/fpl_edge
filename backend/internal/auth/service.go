package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

type Service struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

type AccessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func New(secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}
}

func (s *Service) AccessTokenTTLSeconds() int64 { return int64(s.accessTTL.Seconds()) }

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 128 {
		return "", errors.New("password must be between 12 and 128 characters")
	}
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonParallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint64
	var iterations, parallelism uint64
	for _, item := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(item, "=", 2)
		if len(pair) != 2 {
			return false
		}
		value, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return false
		}
		switch pair[0] {
		case "m":
			memory = value
		case "t":
			iterations = value
		case "p":
			parallelism = value
		}
	}
	if memory == 0 || memory > 256*1024 || iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != argonKeyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Service) NewSessionTokens(userID, sessionID uuid.UUID) (accessToken, refreshToken string, refreshHash []byte, expiresAt time.Time, err error) {
	now := s.now()
	claims := AccessClaims{
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID.String(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			Issuer: "fpl-edge", Audience: jwt.ClaimStrings{"fpl-edge-web"}, ID: uuid.NewString(),
		},
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", "", nil, time.Time{}, err
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return "", "", nil, time.Time{}, err
	}
	refreshToken = sessionID.String() + "." + base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(refreshToken))
	return accessToken, refreshToken, hash[:], now.Add(s.refreshTTL), nil
}

func (s *Service) ParseAccessToken(raw string) (uuid.UUID, uuid.UUID, error) {
	claims := new(AccessClaims)
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected token signing method")
		}
		return s.secret, nil
	}, jwt.WithAudience("fpl-edge-web"), jwt.WithIssuer("fpl-edge"))
	if err != nil || !token.Valid {
		return uuid.Nil, uuid.Nil, errors.New("invalid access token")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid access token subject")
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid access token session")
	}
	return userID, sessionID, nil
}

func HashRefreshToken(raw string) []byte {
	hash := sha256.Sum256([]byte(raw))
	return hash[:]
}

func RefreshSessionID(raw string) (uuid.UUID, error) {
	prefix, _, ok := strings.Cut(raw, ".")
	if !ok {
		return uuid.Nil, errors.New("invalid refresh token")
	}
	return uuid.Parse(prefix)
}
