package main

import (
	"log"
	"net/http"

	"github.com/jaffar066/nexrtc/internal/signaling"
)

func main() {
	http.HandleFunc("/ws", signaling.HandleWebSocket)

	log.Println("NexRTC SFU Server Starting...")
	log.Println("WebSocket server running on ws://localhost:8080/ws")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}