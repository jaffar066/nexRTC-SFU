package signaling

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeClientMessageRejectsMalformedAndForgedEnvelopes(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "empty", payload: ""},
		{name: "invalid JSON", payload: `{"type":`},
		{name: "unknown field", payload: `{"type":"join","roomId":"room-a","admin":true}`},
		{name: "forged identity", payload: `{"type":"join","roomId":"room-a","userId":"victim"}`},
		{name: "trailing JSON", payload: `{"type":"join","roomId":"room-a"} {}`},
		{name: "missing type", payload: `{"roomId":"room-a"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeClientMessage([]byte(test.payload)); err == nil {
				t.Fatal("DecodeClientMessage accepted invalid input")
			}
		})
	}
}

func TestValidateClientMessageEnforcesSignalingSchemas(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name:    "valid join",
			message: Message{Type: TypeJoin, RoomID: "room-a"},
		},
		{
			name:    "join with data",
			message: Message{Type: TypeJoin, RoomID: "room-a", Data: json.RawMessage(`{}`)},
			wantErr: true,
		},
		{
			name:    "unknown client type",
			message: Message{Type: "broadcast"},
			wantErr: true,
		},
		{
			name: "valid offer",
			message: Message{
				Type:   TypeOffer,
				Target: "participant-b",
				Data:   json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
			},
		},
		{
			name: "mismatched SDP type",
			message: Message{
				Type:   TypeOffer,
				Target: "participant-b",
				Data:   json.RawMessage(`{"sdp":{"type":"answer","sdp":"v=0"}}`),
			},
			wantErr: true,
		},
		{
			name: "missing target",
			message: Message{
				Type: TypeICECandidate,
				Data: json.RawMessage(`{"candidate":"candidate:1"}`),
			},
			wantErr: true,
		},
		{
			name: "valid ICE candidate",
			message: Message{
				Type:   TypeICECandidate,
				Target: "participant-b",
				Data:   json.RawMessage(`{"candidate":"candidate:1","sdpMid":"0","sdpMLineIndex":0}`),
			},
		},
		{
			name: "nested ICE candidate object",
			message: Message{
				Type:   TypeICECandidate,
				Target: "participant-b",
				Data:   json.RawMessage(`{"candidate":{"candidate":"candidate:1","sdpMid":"0","sdpMLineIndex":0}}`),
			},
			wantErr: true,
		},
		{
			name: "empty ICE candidate",
			message: Message{
				Type:   TypeICECandidate,
				Target: "participant-b",
				Data:   json.RawMessage(`{"candidate":" "}`),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateClientMessage(test.message)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateClientMessage() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestDecodeTokenMessageRequiresAuthenticationRequestID(t *testing.T) {
	token := strings.Repeat("x", 32)
	message := Message{
		Type:      TypeAuthenticate,
		RequestID: "auth-1",
		Data:      json.RawMessage(`{"token":"` + token + `"}`),
	}
	if _, err := decodeTokenMessage(message); err != nil {
		t.Fatalf("decode valid authentication message: %v", err)
	}
	message.RequestID = ""
	if _, err := decodeTokenMessage(message); err == nil {
		t.Fatal("accepted authentication message without requestId")
	}
}
