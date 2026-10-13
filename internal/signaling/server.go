package signaling

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	ID   string
	Room string
	Conn *websocket.Conn
}

type Room struct {
	ID      string
	Clients map[string]*Client
}

type Message struct {
	Type   string `json:"type"`
	RoomID string `json:"roomId,omitempty"`
	UserID string `json:"userId,omitempty"`
	Target string `json:"target,omitempty"`
	Data   any    `json:"data,omitempty"`
}

var (
	rooms = make(map[string]*Room)
	mu    sync.Mutex

	clientCounter int
)

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	client := createClient(conn)
	log.Printf("Client connected: %s", client.ID)

	for {
		var msg Message

		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Client disconnected: %s", client.ID)
			leaveRoom(client)
			return
		}

		log.Printf(
			"Message from %s: type=%s room=%s target=%s",
			client.ID, msg.Type, msg.RoomID, msg.Target,
		)

		switch msg.Type {
		case "join":
			handleJoin(client, msg.RoomID)
		default:
			handleSignalingMessage(client, msg)
		}
	}
}

func createClient(conn *websocket.Conn) *Client {
	mu.Lock()
	defer mu.Unlock()

	clientCounter++

	return &Client{
		ID:   generateClientID(clientCounter),
		Conn: conn,
	}
}

func generateClientID(number int) string {
	return fmt.Sprintf("user-%d", number)
}

func handleJoin(client *Client, roomID string) {

	if roomID == "" {

		client.Conn.WriteJSON(Message{
			Type: "error",
			Data: "roomId is required",
		})

		return
	}

	mu.Lock()

	if client.Room != "" {

		mu.Unlock()

		client.Conn.WriteJSON(Message{
			Type: "error",
			Data: "already joined a room",
		})

		return
	}

	room, exists := rooms[roomID]

	if !exists {

		room = &Room{
			ID:      roomID,
			Clients: make(map[string]*Client),
		}

		rooms[roomID] = room

		log.Printf(
			"Room created: %s",
			roomID,
		)
	}

	/*
		IMPORTANT:

		Join se pehle room mein jo users
		already maujood hain unki list
		save kar rahe hain.
	*/

	existingUsers := make([]string, 0, len(room.Clients))

	for userID := range room.Clients {

		existingUsers = append(
			existingUsers,
			userID,
		)
	}

	/*
		New user ko room mein add karo.
	*/

	client.Room = roomID

	room.Clients[client.ID] = client

	log.Printf(
		"%s joined room %s. Users: %d",
		client.ID,
		roomID,
		len(room.Clients),
	)

	/*
		Mutex release.

		Network WriteJSON ko mutex ke andar
		hold nahi karna chahiye.
	*/

	mu.Unlock()


	/*
		New user ko confirmation.
	*/

	client.Conn.WriteJSON(Message{
		Type:   "joined",
		RoomID: roomID,
		UserID: client.ID,
	})


	/*
		NEW:

		New user ko existing users
		ki complete list bhejo.
	*/

	client.Conn.WriteJSON(Message{
		Type:   "existing-peers",
		RoomID: roomID,
		Data:    existingUsers,
	})


	/*
		Ab existing users ko batao:

		"New user join hua hai."
	*/

	for _, userID := range existingUsers {

		mu.Lock()

		otherClient, exists :=
			room.Clients[userID]

		mu.Unlock()

		if !exists {
			continue
		}

		err := otherClient.Conn.WriteJSON(Message{
			Type:   "peer-joined",
			RoomID: roomID,
			UserID: client.ID,
		})

		if err != nil {

			log.Printf(
				"Failed to notify %s: %v",
				userID,
				err,
			)
		}
	}
}

func leaveRoom(client *Client) {
	mu.Lock()
	defer mu.Unlock()

	if client.Room == "" {
		return
	}

	room, exists := rooms[client.Room]
	if !exists {
		return
	}

	delete(room.Clients, client.ID)

	log.Printf(
		"%s left room %s. Users remaining: %d",
		client.ID, client.Room, len(room.Clients),
	)

	for _, otherClient := range room.Clients {
		otherClient.Conn.WriteJSON(Message{
			Type:   "peer-left",
			RoomID: client.Room,
			UserID: client.ID,
		})
	}

	if len(room.Clients) == 0 {
		delete(rooms, client.Room)
		log.Printf("Room deleted: %s", client.Room)
	}

	client.Room = ""
}

func handleSignalingMessage(client *Client, msg Message) {
	mu.Lock()
	defer mu.Unlock()

	if client.Room == "" {
		client.Conn.WriteJSON(Message{
			Type: "error",
			Data: "join a room first",
		})
		return
	}

	room, exists := rooms[client.Room]
	if !exists {
		return
	}

	// Targeted signaling.
	// Example: user-1 -> user-2, only user-2 receives the message.
	if msg.Target != "" {
		targetClient, exists := room.Clients[msg.Target]
		if !exists {
			client.Conn.WriteJSON(Message{
				Type: "error",
				Data: "target user not found",
			})
			return
		}

		err := targetClient.Conn.WriteJSON(Message{
			Type:   msg.Type,
			RoomID: client.Room,
			UserID: client.ID,
			Target: msg.Target,
			Data:   msg.Data,
		})
		if err != nil {
			log.Printf(
				"Failed to send message to %s: %v",
				msg.Target, err,
			)
		}

		return
	}

	// No target provided: broadcast inside the room.
	// Useful for room-level events.
	for userID, otherClient := range room.Clients {
		if userID == client.ID {
			continue
		}

		err := otherClient.Conn.WriteJSON(Message{
			Type:   msg.Type,
			RoomID: client.Room,
			UserID: client.ID,
			Data:   msg.Data,
		})
		if err != nil {
			log.Printf(
				"Failed to forward message to %s: %v",
				userID, err,
			)
		}
	}
}