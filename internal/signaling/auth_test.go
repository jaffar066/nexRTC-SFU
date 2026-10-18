package signaling

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestTokenAuthenticatorValidatesRoomScopedIdentity(t *testing.T) {
	authenticator := mustAuthenticator(t)
	token := signTestToken(t, jwt.SigningMethodHS256, roomClaims{
		RoomID: "room-a",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "participant-a",
			Issuer:    "nexrtc-test",
			Audience:  jwt.ClaimStrings{"nexrtc"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})

	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.ParticipantID != "participant-a" || principal.RoomID != "room-a" {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestTokenAuthenticatorIssuesShortLivedRoomToken(t *testing.T) {
	authenticator := mustAuthenticator(t)
	token, issuedPrincipal, err := authenticator.IssueRoomToken(context.Background(), "room-a")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.ParticipantID != issuedPrincipal.ParticipantID || principal.RoomID != "room-a" {
		t.Fatalf("issued principal = %#v, authenticated principal = %#v", issuedPrincipal, principal)
	}
	if issuedPrincipal.ExpiresAt.IsZero() || time.Until(issuedPrincipal.ExpiresAt) > maxTokenLifetime {
		t.Fatalf("unexpected token expiry: %s", issuedPrincipal.ExpiresAt)
	}
	if _, _, err := authenticator.IssueRoomToken(context.Background(), ""); err == nil {
		t.Fatal("issued a token without a room ID")
	}
}

func TestTokenAuthenticatorRejectsInvalidClaimsAndAlgorithms(t *testing.T) {
	authenticator := mustAuthenticator(t)
	tests := []struct {
		name   string
		method *jwt.SigningMethodHMAC
		claims roomClaims
	}{
		{
			name:   "wrong issuer",
			method: jwt.SigningMethodHS256,
			claims: testClaims("issuer", "nexrtc", time.Now().Add(time.Minute)),
		},
		{
			name:   "wrong audience",
			method: jwt.SigningMethodHS256,
			claims: testClaims("nexrtc-test", "other", time.Now().Add(time.Minute)),
		},
		{
			name:   "expired",
			method: jwt.SigningMethodHS256,
			claims: testClaims("nexrtc-test", "nexrtc", time.Now().Add(-time.Minute)),
		},
		{
			name:   "missing expiry",
			method: jwt.SigningMethodHS256,
			claims: roomClaims{
				RoomID: "room-a",
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:  "participant-a",
					Issuer:   "nexrtc-test",
					Audience: jwt.ClaimStrings{"nexrtc"},
					IssuedAt: jwt.NewNumericDate(time.Now()),
				},
			},
		},
		{
			name:   "missing issued at",
			method: jwt.SigningMethodHS256,
			claims: roomClaims{
				RoomID: "room-a",
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   "participant-a",
					Issuer:    "nexrtc-test",
					Audience:  jwt.ClaimStrings{"nexrtc"},
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				},
			},
		},
		{
			name:   "long lifetime",
			method: jwt.SigningMethodHS256,
			claims: testClaims("nexrtc-test", "nexrtc", time.Now().Add(16*time.Minute)),
		},
		{
			name:   "wrong signing algorithm",
			method: jwt.SigningMethodHS384,
			claims: testClaims("nexrtc-test", "nexrtc", time.Now().Add(time.Minute)),
		},
		{
			name:   "missing room",
			method: jwt.SigningMethodHS256,
			claims: roomClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   "participant-a",
					Issuer:    "nexrtc-test",
					Audience:  jwt.ClaimStrings{"nexrtc"},
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := signTestToken(t, test.method, test.claims)
			if _, err := authenticator.Authenticate(context.Background(), token); err == nil {
				t.Fatal("Authenticate accepted an invalid token")
			}
		})
	}
}

func TestNewTokenAuthenticatorRequiresStrongConfiguration(t *testing.T) {
	if _, err := NewTokenAuthenticator("short", "issuer", "audience"); err == nil {
		t.Fatal("accepted a short JWT secret")
	}
	if _, err := NewTokenAuthenticator(testJWTSecret, "", "audience"); err == nil {
		t.Fatal("accepted an empty issuer")
	}
	if _, err := NewTokenAuthenticator(testJWTSecret, "issuer", ""); err == nil {
		t.Fatal("accepted an empty audience")
	}
}

func mustAuthenticator(t *testing.T) *TokenAuthenticator {
	t.Helper()
	authenticator, err := NewTokenAuthenticator(testJWTSecret, "nexrtc-test", "nexrtc")
	if err != nil {
		t.Fatal(err)
	}
	return authenticator
}

func testClaims(issuer, audience string, expiry time.Time) roomClaims {
	return roomClaims{
		RoomID: "room-a",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "participant-a",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
}

func signTestToken(t *testing.T, method jwt.SigningMethod, claims roomClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
