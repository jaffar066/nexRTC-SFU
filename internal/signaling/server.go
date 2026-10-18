package signaling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const authenticationTimeout = 10 * time.Second

type Server struct {
	authenticator   Authenticator
	allowedOrigins  map[string]struct{}
	enableDevTokens bool

	mu    sync.Mutex
	rooms map[string]*Room
}

type Client struct {
	ID            string
	Room          string
	Principal     Principal
	Conn          *websocket.Conn
	authRequestID string

	writeMu sync.Mutex
}

type Room struct {
	ID      string
	Clients map[string]*Client
}

type ServerOptions struct {
	EnableDevTokenIssuer bool
}

type issueTokenRequest struct {
	RoomID string `json:"roomId"`
}

type issueTokenResponse struct {
	Token         string    `json:"token"`
	ParticipantID string    `json:"participantId"`
	RoomID        string    `json:"roomId"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

func NewServer(authenticator Authenticator, allowedOrigins []string) (*Server, error) {
	return NewServerWithOptions(authenticator, allowedOrigins, ServerOptions{})
}

func NewServerWithOptions(authenticator Authenticator, allowedOrigins []string, options ServerOptions) (*Server, error) {
	if authenticator == nil {
		return nil, fmt.Errorf("signaling authenticator is required")
	}
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			return nil, fmt.Errorf("allowed origins must be explicit, non-wildcard origins")
		}
		origins[origin] = struct{}{}
		if options.EnableDevTokenIssuer {
			for _, alias := range loopbackOriginAliases(origin) {
				origins[alias] = struct{}{}
			}
		}
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("at least one allowed WebSocket origin is required")
	}
	return &Server{
		authenticator:   authenticator,
		allowedOrigins:  origins,
		enableDevTokens: options.EnableDevTokenIssuer,
		rooms:           make(map[string]*Room),
	}, nil
}

func loopbackOriginAliases(origin string) []string {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil
	}
	port := parsed.Port()
	aliases := make([]string, 0, 2)
	for _, aliasHost := range []string{"localhost", "127.0.0.1"} {
		if aliasHost == host {
			continue
		}
		alias := *parsed
		if port != "" {
			alias.Host = net.JoinHostPort(aliasHost, port)
		} else {
			alias.Host = aliasHost
		}
		aliases = append(aliases, alias.String())
	}
	return aliases
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/dev/token" {
		s.serveDevelopmentToken(w, r)
		return
	}
	if r.URL.Path != "/ws" {
		http.NotFound(w, r)
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(request *http.Request) bool {
			_, ok := s.allowedOrigins[request.Header.Get("Origin")]
			return ok
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade rejected: %v", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(maxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(authenticationTimeout)); err != nil {
		log.Printf("Failed to set authentication deadline: %v", err)
		return
	}
	client, err := s.authenticate(r.Context(), conn)
	if err != nil {
		log.Printf("WebSocket authentication rejected: %v", err)
		return
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("Failed to clear authentication deadline for %s: %v", client.ID, err)
		s.leaveRoom(client)
		return
	}
	authenticatedData, err := messageData(AuthenticatedData{ExpiresAt: client.Principal.ExpiresAt})
	if err != nil {
		log.Printf("Failed to encode authentication response for %s: %v", client.ID, err)
		return
	}
	if err := client.send(Message{
		Type:      TypeAuthenticated,
		RequestID: client.authRequestID,
		UserID:    client.ID,
		Data:      authenticatedData,
	}); err != nil {
		log.Printf("Failed to acknowledge authentication for %s: %v", client.ID, err)
		return
	}
	log.Printf("Participant authenticated: participant=%s room=%s expires_at=%s",
		client.ID, client.Principal.RoomID, client.Principal.ExpiresAt.UTC().Format(time.RFC3339))

	expiryDone := make(chan struct{})
	defer close(expiryDone)
	go s.closeAtTokenExpiry(client, expiryDone)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("WebSocket read failed for %s: %v", client.ID, err)
			}
			s.leaveRoom(client)
			return
		}

		message, err := DecodeClientMessage(payload)
		if err != nil {
			if sendErr := client.sendError("", "invalid_message", err.Error()); sendErr != nil {
				log.Printf("Failed to report invalid message for %s: %v", client.ID, sendErr)
				s.leaveRoom(client)
				return
			}
			continue
		}
		if err := ValidateClientMessage(message); err != nil {
			if sendErr := client.sendError(message.RequestID, "invalid_message", err.Error()); sendErr != nil {
				log.Printf("Failed to report invalid message for %s: %v", client.ID, sendErr)
				s.leaveRoom(client)
				return
			}
			continue
		}
		if err := s.handleClientMessage(client, message); err != nil {
			log.Printf("Signaling message failed for %s: %v", client.ID, err)
			s.leaveRoom(client)
			return
		}
	}
}

func (s *Server) serveDevelopmentToken(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if _, ok := s.allowedOrigins[origin]; !ok {
		http.Error(w, "development token endpoint is only available to allowed local origins", http.StatusForbidden)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.enableDevTokens {
		http.Error(w, "local demo token issuer is disabled; set NEXRTC_ENABLE_DEV_TOKEN_ISSUER=true and restart the server", http.StatusNotFound)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "development token endpoint is only available on loopback", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	issuer, ok := s.authenticator.(TokenIssuer)
	if !ok {
		http.Error(w, "development token issuer is not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request issueTokenRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid token request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "request must contain one JSON object", http.StatusBadRequest)
		return
	}
	token, principal, err := issuer.IssueRoomToken(r.Context(), request.RoomID)
	if err != nil {
		http.Error(w, "could not create room token", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(issueTokenResponse{
		Token:         token,
		ParticipantID: principal.ParticipantID,
		RoomID:        principal.RoomID,
		ExpiresAt:     principal.ExpiresAt,
	}); err != nil {
		log.Printf("Failed to return development token: %v", err)
	}
}

func isLoopbackRequest(r *http.Request) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remoteHost).IsLoopback() {
		return false
	}
	host := r.Host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func (s *Server) authenticate(ctx context.Context, conn *websocket.Conn) (*Client, error) {
	reject := func(requestID, code, message string, cause error) (*Client, error) {
		if err := writeError(conn, requestID, code, message); err != nil {
			return nil, fmt.Errorf("%w (failed to send rejection: %v)", cause, err)
		}
		return nil, cause
	}

	_, payload, err := conn.ReadMessage()
	if err != nil {
		return reject("", "authentication_required", "authentication message is required",
			fmt.Errorf("read authentication message: %w", err))
	}
	message, err := DecodeClientMessage(payload)
	if err != nil {
		return reject("", "invalid_message", err.Error(), err)
	}
	token, err := decodeTokenMessage(message)
	if err != nil {
		return reject(message.RequestID, "authentication_required", err.Error(), err)
	}
	principal, err := s.authenticator.Authenticate(ctx, token)
	if err != nil {
		return reject(message.RequestID, "unauthorized", "authentication failed", errors.New("token rejected"))
	}
	return &Client{
		ID:            principal.ParticipantID,
		Principal:     principal,
		Conn:          conn,
		authRequestID: message.RequestID,
	}, nil
}

func (s *Server) closeAtTokenExpiry(client *Client, done <-chan struct{}) {
	delay := time.Until(client.Principal.ExpiresAt)
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-done:
		return
	case <-timer.C:
		log.Printf("Token expired: participant=%s room=%s", client.ID, client.Principal.RoomID)
		expiredData, err := messageData(ErrorData{Code: "token_expired", Message: "Authentication token expired"})
		if err != nil {
			log.Printf("Failed to encode token-expiry notification for %s: %v", client.ID, err)
		} else if err = client.send(Message{
			Type:   TypeTokenExpired,
			UserID: client.ID,
			Data:   expiredData,
		}); err != nil {
			log.Printf("Failed to notify %s about token expiry: %v", client.ID, err)
		}
		client.writeMu.Lock()
		err = client.Conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "token expired"),
			time.Now().Add(time.Second),
		)
		client.writeMu.Unlock()
		if err != nil {
			log.Printf("Failed to send token-expiry close frame to %s: %v", client.ID, err)
		}
		if err := client.Conn.Close(); err != nil {
			log.Printf("Failed to close expired connection for %s: %v", client.ID, err)
		}
	}
}

func (s *Server) handleClientMessage(client *Client, message Message) error {
	if message.Type == TypeJoin {
		return s.joinRoom(client, message)
	}

	s.mu.Lock()
	if client.Room == "" {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "room_required", "join your authorized room first")
	}
	if message.RoomID != "" && message.RoomID != client.Room {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "forbidden", "message room does not match membership")
	}
	room := s.rooms[client.Room]
	if room == nil {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "room_not_found", "room membership is no longer active")
	}
	target := room.Clients[message.Target]
	if target == nil || target.ID == client.ID {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "target_not_found", "target participant is not in your room")
	}
	outbound := Message{
		Type:      message.Type,
		RequestID: message.RequestID,
		RoomID:    client.Room,
		UserID:    client.ID,
		Target:    target.ID,
		Data:      message.Data,
	}
	s.mu.Unlock()
	if err := target.send(outbound); err != nil {
		log.Printf("Failed to forward %s from %s to %s: %v", message.Type, client.ID, target.ID, err)
		return client.sendError(message.RequestID, "delivery_failed", "target is temporarily unavailable")
	}
	return nil
}

func (s *Server) joinRoom(client *Client, message Message) error {
	if message.RoomID != client.Principal.RoomID {
		return client.sendError(message.RequestID, "forbidden", "token does not grant access to this room")
	}

	s.mu.Lock()
	if client.Room != "" {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "already_joined", "client already joined a room")
	}
	room := s.rooms[message.RoomID]
	if room == nil {
		room = &Room{
			ID:      message.RoomID,
			Clients: make(map[string]*Client),
		}
		s.rooms[message.RoomID] = room
	}
	if _, exists := room.Clients[client.ID]; exists {
		s.mu.Unlock()
		return client.sendError(message.RequestID, "participant_connected", "participant already has an active connection")
	}
	existingUsers := make([]string, 0, len(room.Clients))
	peers := make([]*Client, 0, len(room.Clients))
	for id, otherClient := range room.Clients {
		existingUsers = append(existingUsers, id)
		peers = append(peers, otherClient)
	}
	client.Room = room.ID
	room.Clients[client.ID] = client
	peerCount := len(room.Clients)
	s.mu.Unlock()
	log.Printf("Participant joined room: participant=%s room=%s participants=%d",
		client.ID, room.ID, peerCount)

	if err := client.send(Message{
		Type:      TypeJoined,
		RequestID: message.RequestID,
		RoomID:    room.ID,
		UserID:    client.ID,
	}); err != nil {
		return err
	}
	existingPeersData, err := messageData(existingUsers)
	if err != nil {
		return fmt.Errorf("encode existing peers: %w", err)
	}
	if err := client.send(Message{
		Type:   TypeExistingPeers,
		RoomID: room.ID,
		Data:   existingPeersData,
	}); err != nil {
		return err
	}
	for _, peer := range peers {
		if err := peer.send(Message{
			Type:   TypePeerJoined,
			RoomID: room.ID,
			UserID: client.ID,
		}); err != nil {
			log.Printf("Failed to notify participant %s about join: %v", peer.ID, err)
		}
	}
	return nil
}

func (s *Server) leaveRoom(client *Client) {
	s.mu.Lock()
	room := s.rooms[client.Room]
	if room == nil {
		client.Room = ""
		s.mu.Unlock()
		return
	}
	delete(room.Clients, client.ID)
	client.Room = ""
	peers := make([]*Client, 0, len(room.Clients))
	for _, peer := range room.Clients {
		peers = append(peers, peer)
	}
	if len(room.Clients) == 0 {
		delete(s.rooms, room.ID)
	}
	s.mu.Unlock()

	for _, peer := range peers {
		if err := peer.send(Message{
			Type:   TypePeerLeft,
			RoomID: room.ID,
			UserID: client.ID,
		}); err != nil {
			log.Printf("Failed to notify participant %s about departure: %v", peer.ID, err)
		}
	}
}

func (c *Client) send(message Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return c.Conn.WriteJSON(message)
}

func (c *Client) sendError(requestID, code, text string) error {
	data, err := messageData(ErrorData{Code: code, Message: text})
	if err != nil {
		return fmt.Errorf("encode error message: %w", err)
	}
	return c.send(Message{
		Type:      TypeError,
		RequestID: requestID,
		Data:      data,
	})
}

func writeError(conn *websocket.Conn, requestID, code, text string) error {
	data, err := messageData(ErrorData{Code: code, Message: text})
	if err != nil {
		return fmt.Errorf("encode error message: %w", err)
	}
	return conn.WriteJSON(Message{
		Type:      TypeError,
		RequestID: requestID,
		Data:      data,
	})
}
