package signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	TypeAuthenticate  = "authenticate"
	TypeAuthenticated = "authenticated"
	TypeJoin          = "join"
	TypeJoined        = "joined"
	TypeExistingPeers = "existing-peers"
	TypePeerJoined    = "peer-joined"
	TypePeerLeft      = "peer-left"
	TypeOffer         = "offer"
	TypeAnswer        = "answer"
	TypeICECandidate  = "ice-candidate"
	TypeTokenExpired  = "token-expired"
	TypeError         = "error"

	maxMessageSize = 1 << 20
	maxSDPSize     = 256 << 10
	maxCandidate   = 4096
)

type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	RoomID    string          `json:"roomId,omitempty"`
	UserID    string          `json:"userId,omitempty"`
	Target    string          `json:"target,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AuthenticatedData struct {
	ExpiresAt time.Time `json:"expiresAt"`
}

type tokenData struct {
	Token string `json:"token"`
}

type sdpData struct {
	SDP struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	} `json:"sdp"`
}

type iceCandidateData struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdpMid,omitempty"`
	SDPMLineIndex    *uint16 `json:"sdpMLineIndex,omitempty"`
	UsernameFragment *string `json:"usernameFragment,omitempty"`
}

func DecodeClientMessage(payload []byte) (Message, error) {
	if len(payload) == 0 || len(payload) > maxMessageSize {
		return Message{}, fmt.Errorf("message size must be between 1 and %d bytes", maxMessageSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var message Message
	if err := decoder.Decode(&message); err != nil {
		return Message{}, fmt.Errorf("invalid message envelope: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return Message{}, fmt.Errorf("message contains multiple JSON values")
		}
		return Message{}, fmt.Errorf("invalid trailing message data: %w", err)
	}
	if message.Type == "" {
		return Message{}, fmt.Errorf("message type is required")
	}
	if len(message.RequestID) > 128 {
		return Message{}, fmt.Errorf("requestId is too long")
	}
	if message.UserID != "" {
		return Message{}, fmt.Errorf("userId is assigned by the server")
	}
	return message, nil
}

func ValidateClientMessage(message Message) error {
	switch message.Type {
	case TypeJoin:
		if message.RoomID == "" || len(message.RoomID) > 128 {
			return fmt.Errorf("roomId must be between 1 and 128 characters")
		}
		if message.Target != "" || len(message.Data) != 0 {
			return fmt.Errorf("join does not accept target or data")
		}
	case TypeOffer, TypeAnswer:
		if err := validateTargetedMessage(message); err != nil {
			return err
		}
		var data sdpData
		if err := decodeStrict(message.Data, &data); err != nil {
			return fmt.Errorf("%s data must contain an SDP object: %w", message.Type, err)
		}
		expectedType := message.Type
		if data.SDP.Type != expectedType {
			return fmt.Errorf("%s SDP type must be %q", message.Type, expectedType)
		}
		if strings.TrimSpace(data.SDP.SDP) == "" || len(data.SDP.SDP) > maxSDPSize {
			return fmt.Errorf("%s SDP must be between 1 and %d bytes", message.Type, maxSDPSize)
		}
	case TypeICECandidate:
		if err := validateTargetedMessage(message); err != nil {
			return err
		}
		var data iceCandidateData
		if err := decodeStrict(message.Data, &data); err != nil {
			return fmt.Errorf("ICE candidate data is invalid: %w", err)
		}
		if strings.TrimSpace(data.Candidate) == "" || len(data.Candidate) > maxCandidate {
			return fmt.Errorf("ICE candidate must be between 1 and %d bytes", maxCandidate)
		}
	default:
		return fmt.Errorf("unsupported client message type %q", message.Type)
	}
	return nil
}

func decodeTokenMessage(message Message) (string, error) {
	if message.Type != TypeAuthenticate {
		return "", fmt.Errorf("first message must be %q", TypeAuthenticate)
	}
	if message.RequestID == "" || message.RoomID != "" || message.UserID != "" || message.Target != "" {
		return "", fmt.Errorf("authenticate requires requestId and only token data")
	}
	var data tokenData
	if err := decodeStrict(message.Data, &data); err != nil {
		return "", fmt.Errorf("authenticate data must contain a token: %w", err)
	}
	if data.Token == "" || len(data.Token) > maxTokenLength {
		return "", fmt.Errorf("token is missing or too long")
	}
	return data.Token, nil
}

func validateTargetedMessage(message Message) error {
	if message.Target == "" || len(message.Target) > 128 {
		return fmt.Errorf("%s target must be between 1 and 128 characters", message.Type)
	}
	if len(message.Data) == 0 || bytes.Equal(bytes.TrimSpace(message.Data), []byte("null")) {
		return fmt.Errorf("%s data is required", message.Type)
	}
	if message.RoomID != "" && len(message.RoomID) > 128 {
		return fmt.Errorf("roomId is too long")
	}
	return nil
}

func messageData(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal signaling response data: %w", err)
	}
	return json.RawMessage(data), nil
}

func decodeStrict(payload []byte, target any) error {
	if len(payload) == 0 || bytes.Equal(bytes.TrimSpace(payload), []byte("null")) {
		return fmt.Errorf("data object is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("data contains multiple JSON values")
		}
		return fmt.Errorf("invalid trailing data: %w", err)
	}
	return nil
}
