package signaling

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const maxTokenLength = 8192
const maxTokenLifetime = 15 * time.Minute

type Principal struct {
	ParticipantID string
	RoomID        string
	ExpiresAt     time.Time
}

type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}

type TokenIssuer interface {
	IssueRoomToken(context.Context, string) (string, Principal, error)
}

type TokenAuthenticator struct {
	secret   []byte
	parser   *jwt.Parser
	issuer   string
	audience string
}

type roomClaims struct {
	RoomID string `json:"room_id"`
	jwt.RegisteredClaims
}

func NewTokenAuthenticator(secret, issuer, audience string) (*TokenAuthenticator, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must be at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("JWT issuer is empty")
	}
	if strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT audience is empty")
	}

	return &TokenAuthenticator{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
		),
	}, nil
}

func (a *TokenAuthenticator) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}
	if rawToken == "" || len(rawToken) > maxTokenLength {
		return Principal{}, errors.New("invalid token")
	}

	claims := &roomClaims{}
	token, err := a.parser.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	})
	if err != nil || token == nil || !token.Valid {
		return Principal{}, errors.New("invalid token")
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return Principal{}, errors.New("invalid token")
	}

	principal := Principal{
		ParticipantID: claims.Subject,
		RoomID:        claims.RoomID,
		ExpiresAt:     claims.ExpiresAt.Time,
	}
	if principal.ParticipantID == "" || len(principal.ParticipantID) > 128 {
		return Principal{}, errors.New("invalid token")
	}
	if principal.RoomID == "" || len(principal.RoomID) > 128 {
		return Principal{}, errors.New("invalid token")
	}
	lifetime := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if lifetime <= 0 || lifetime > maxTokenLifetime {
		return Principal{}, errors.New("invalid token")
	}
	return principal, nil
}

func (a *TokenAuthenticator) IssueRoomToken(ctx context.Context, roomID string) (string, Principal, error) {
	if err := ctx.Err(); err != nil {
		return "", Principal{}, err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" || len(roomID) > 128 {
		return "", Principal{}, fmt.Errorf("room ID must be between 1 and 128 characters")
	}

	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", Principal{}, fmt.Errorf("generate participant ID: %w", err)
	}

	now := time.Now().UTC()
	principal := Principal{
		ParticipantID: "user-" + hex.EncodeToString(randomID),
		RoomID:        roomID,
		ExpiresAt:     now.Add(maxTokenLifetime),
	}
	claims := roomClaims{
		RoomID: roomID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   principal.ParticipantID,
			Issuer:    a.issuer,
			Audience:  jwt.ClaimStrings{a.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(principal.ExpiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return "", Principal{}, fmt.Errorf("sign room token: %w", err)
	}
	return token, principal, nil
}
