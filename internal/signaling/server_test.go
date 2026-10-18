package signaling

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

const testOrigin = "https://client.example"

func TestServerAuthenticatesAuthorizesAndRoutesSignaling(t *testing.T) {
	authenticator := mustAuthenticator(t)
	server, err := NewServer(authenticator, []string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	alice := connectAuthenticated(t, httpServer.URL, "alice", "room-a")
	defer alice.Close()
	bob := connectAuthenticated(t, httpServer.URL, "bob", "room-a")
	defer bob.Close()

	writeClientMessage(t, alice, Message{Type: TypeJoin, RequestID: "join-a", RoomID: "room-a"})
	assertMessage(t, alice, TypeJoined, "alice")
	assertMessage(t, alice, TypeExistingPeers, "")

	writeClientMessage(t, bob, Message{Type: TypeJoin, RequestID: "join-b", RoomID: "room-a"})
	assertMessage(t, bob, TypeJoined, "bob")
	assertMessage(t, bob, TypeExistingPeers, "")
	assertMessage(t, alice, TypePeerJoined, "bob")

	writeClientMessage(t, bob, Message{
		Type:   TypeOffer,
		Target: "alice",
		Data:   json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	offer := readServerMessage(t, alice)
	if offer.Type != TypeOffer || offer.UserID != "bob" || offer.Target != "alice" || offer.RoomID != "room-a" {
		t.Fatalf("forwarded offer = %#v", offer)
	}

	writeClientMessage(t, bob, Message{
		Type:   TypeICECandidate,
		Target: "alice",
		Data:   json.RawMessage(`{"candidate":"candidate:1 1 UDP 2122260223 192.0.2.1 54400 typ host","sdpMid":"0","sdpMLineIndex":0}`),
	})
	candidate := readServerMessage(t, alice)
	if candidate.Type != TypeICECandidate || candidate.UserID != "bob" {
		t.Fatalf("forwarded candidate = %#v", candidate)
	}
	var candidateData iceCandidateData
	if err := json.Unmarshal(candidate.Data, &candidateData); err != nil {
		t.Fatalf("decode forwarded candidate: %v", err)
	}
	if candidateData.Candidate != "candidate:1 1 UDP 2122260223 192.0.2.1 54400 typ host" ||
		candidateData.SDPMid == nil || *candidateData.SDPMid != "0" ||
		candidateData.SDPMLineIndex == nil || *candidateData.SDPMLineIndex != 0 {
		t.Fatalf("forwarded candidate data = %#v", candidateData)
	}

	writeClientMessage(t, bob, Message{
		Type:   TypeICECandidate,
		Target: "not-in-room",
		Data:   json.RawMessage(`{"candidate":"candidate:1"}`),
	})
	assertErrorCode(t, bob, "target_not_found")
}

func TestServerRejectsUnauthorizedRoomAndOrigin(t *testing.T) {
	server, err := NewServer(mustAuthenticator(t), []string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	client := connectAuthenticated(t, httpServer.URL, "alice", "room-a")
	defer client.Close()
	writeClientMessage(t, client, Message{Type: TypeJoin, RequestID: "join-x", RoomID: "room-x"})
	assertErrorCode(t, client, "forbidden")
	writeClientMessage(t, client, Message{Type: TypeJoin, RequestID: "join-a", RoomID: "room-a"})
	assertMessage(t, client, TypeJoined, "alice")

	dialer := websocket.Dialer{}
	header := make(http.Header)
	header.Set("Origin", "https://attacker.example")
	_, response, err := dialer.Dial(websocketURL(httpServer.URL), header)
	if err == nil {
		t.Fatal("server accepted a disallowed origin")
	}
	if response == nil || response.StatusCode != 403 {
		t.Fatalf("disallowed origin response = %#v, error=%v", response, err)
	}
}

func TestServerNotifiesAndClosesConnectionWhenTokenExpires(t *testing.T) {
	server, err := NewServer(mustAuthenticator(t), []string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	conn := connectAuthenticatedFor(t, httpServer.URL, "short-lived", "room-a", 3*time.Second)
	writeClientMessage(t, conn, Message{Type: TypeJoin, RequestID: "join-short", RoomID: "room-a"})
	assertMessage(t, conn, TypeJoined, "short-lived")
	assertMessage(t, conn, TypeExistingPeers, "")

	observer := connectAuthenticated(t, httpServer.URL, "observer", "room-a")
	defer observer.Close()
	writeClientMessage(t, observer, Message{Type: TypeJoin, RequestID: "join-observer", RoomID: "room-a"})
	assertMessage(t, observer, TypeJoined, "observer")
	assertMessage(t, observer, TypeExistingPeers, "")
	assertMessage(t, conn, TypePeerJoined, "observer")

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	message := assertMessage(t, conn, TypeTokenExpired, "short-lived")
	var data ErrorData
	if err := json.Unmarshal(message.Data, &data); err != nil {
		t.Fatalf("decode token expiry event: %v", err)
	}
	if data.Code != "token_expired" {
		t.Fatalf("expiry code = %q, want token_expired", data.Code)
	}
	_, _, err = conn.ReadMessage()
	if !websocket.IsCloseError(err, 4001) {
		t.Fatalf("read after expiry = %v, want close code 4001", err)
	}
	assertMessage(t, observer, TypePeerLeft, "short-lived")
}

func TestDevelopmentTokenEndpointIssuesRoomTokenOnlyWhenEnabledLocally(t *testing.T) {
	authenticator := mustAuthenticator(t)
	server, err := NewServerWithOptions(authenticator, []string{testOrigin}, ServerOptions{
		EnableDevTokenIssuer: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/dev/token", strings.NewReader(`{"roomId":"room-a"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", testOrigin)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var issued issueTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&issued); err != nil {
		t.Fatalf("decode issued token response: %v", err)
	}
	principal, err := authenticator.Authenticate(t.Context(), issued.Token)
	if err != nil {
		t.Fatalf("authenticate issued token: %v", err)
	}
	if principal.RoomID != "room-a" || principal.ParticipantID != issued.ParticipantID || principal.ExpiresAt.IsZero() {
		t.Fatalf("issued token principal = %#v, response = %#v", principal, issued)
	}

	preflight, err := http.NewRequest(http.MethodOptions, httpServer.URL+"/dev/token", nil)
	if err != nil {
		t.Fatal(err)
	}
	preflight.Header.Set("Origin", testOrigin)
	response, err = http.DefaultClient.Do(preflight)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("preflight response status=%d allow-origin=%q", response.StatusCode, response.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestDevelopmentTokenEndpointAcceptsEquivalentLoopbackOrigin(t *testing.T) {
	server, err := NewServerWithOptions(mustAuthenticator(t), []string{"http://localhost:5500"}, ServerOptions{
		EnableDevTokenIssuer: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodOptions, httpServer.URL+"/dev/token", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "http://127.0.0.1:5500")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5500" {
		t.Fatalf("allow-origin = %q, want request origin", got)
	}
}

func TestDevelopmentTokenEndpointIsDisabledByDefault(t *testing.T) {
	server, err := NewServer(mustAuthenticator(t), []string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/dev/token", strings.NewReader(`{"roomId":"room-a"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", testOrigin)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	if response.Header.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("disabled response missing CORS origin: %q", response.Header.Get("Access-Control-Allow-Origin"))
	}
	detail, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detail), "NEXRTC_ENABLE_DEV_TOKEN_ISSUER=true") {
		t.Fatalf("disabled response lacks setup instruction: %q", detail)
	}

	preflight, err := http.NewRequest(http.MethodOptions, httpServer.URL+"/dev/token", nil)
	if err != nil {
		t.Fatal(err)
	}
	preflight.Header.Set("Origin", testOrigin)
	response, err = http.DefaultClient.Do(preflight)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("disabled preflight status=%d allow-origin=%q", response.StatusCode, response.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestNewServerRequiresExplicitAuthenticationAndOrigins(t *testing.T) {
	if _, err := NewServer(nil, []string{testOrigin}); err == nil {
		t.Fatal("accepted a nil authenticator")
	}
	if _, err := NewServer(mustAuthenticator(t), []string{"*"}); err == nil {
		t.Fatal("accepted wildcard origin")
	}
	if _, err := NewServer(mustAuthenticator(t), nil); err == nil {
		t.Fatal("accepted empty origin list")
	}
}

func connectAuthenticated(t *testing.T, httpURL, participantID, roomID string) *websocket.Conn {
	return connectAuthenticatedFor(t, httpURL, participantID, roomID, time.Minute)
}

func connectAuthenticatedFor(t *testing.T, httpURL, participantID, roomID string, lifetime time.Duration) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{}
	header := make(http.Header)
	header.Set("Origin", testOrigin)
	conn, response, err := dialer.Dial(websocketURL(httpURL), header)
	if err != nil {
		if response != nil {
			t.Fatalf("connect signaling server: %v (status %d)", err, response.StatusCode)
		}
		t.Fatalf("connect signaling server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	token := signTestToken(t, jwt.SigningMethodHS256, roomClaims{
		RoomID: roomID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   participantID,
			Issuer:    "nexrtc-test",
			Audience:  jwt.ClaimStrings{"nexrtc"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(lifetime)),
		},
	})
	writeClientMessage(t, conn, Message{
		Type:      TypeAuthenticate,
		RequestID: "auth-" + participantID,
		Data:      json.RawMessage(`{"token":"` + token + `"}`),
	})
	authenticated := assertMessage(t, conn, TypeAuthenticated, participantID)
	var authData AuthenticatedData
	if err := json.Unmarshal(authenticated.Data, &authData); err != nil {
		t.Fatalf("decode authentication details: %v", err)
	}
	if authData.ExpiresAt.IsZero() {
		t.Fatal("authenticated response did not include token expiry")
	}
	return conn
}

func writeClientMessage(t *testing.T, conn *websocket.Conn, message Message) {
	t.Helper()
	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("write signaling message: %v", err)
	}
}

func readServerMessage(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()
	var message Message
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("read signaling message: %v", err)
	}
	return message
}

func assertMessage(t *testing.T, conn *websocket.Conn, messageType, userID string) Message {
	t.Helper()
	message := readServerMessage(t, conn)
	if message.Type != messageType || (userID != "" && message.UserID != userID) {
		t.Fatalf("message = %#v, want type=%q userId=%q", message, messageType, userID)
	}
	return message
}

func assertErrorCode(t *testing.T, conn *websocket.Conn, code string) {
	t.Helper()
	message := readServerMessage(t, conn)
	if message.Type != TypeError {
		t.Fatalf("message type = %q, want error", message.Type)
	}
	var data ErrorData
	if err := json.Unmarshal(message.Data, &data); err != nil {
		t.Fatalf("decode error data: %v", err)
	}
	if data.Code != code {
		t.Fatalf("error code = %q, want %q (message %q)", data.Code, code, data.Message)
	}
}

func websocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http") + "/ws"
}
