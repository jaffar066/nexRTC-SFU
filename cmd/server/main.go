package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaffar066/nexrtc/internal/signaling"
)

func main() {
	authenticator, err := signaling.NewTokenAuthenticator(
		os.Getenv("NEXRTC_JWT_SECRET"),
		os.Getenv("NEXRTC_JWT_ISSUER"),
		os.Getenv("NEXRTC_JWT_AUDIENCE"),
	)
	if err != nil {
		log.Fatalf("Invalid JWT configuration: %v", err)
	}
	allowedOrigins := splitOrigins(os.Getenv("NEXRTC_ALLOWED_ORIGINS"))
	enableDevTokens, err := strconv.ParseBool(os.Getenv("NEXRTC_ENABLE_DEV_TOKEN_ISSUER"))
	if err != nil && os.Getenv("NEXRTC_ENABLE_DEV_TOKEN_ISSUER") != "" {
		log.Fatalf("Invalid NEXRTC_ENABLE_DEV_TOKEN_ISSUER value: %v", err)
	}
	server, err := signaling.NewServerWithOptions(authenticator, allowedOrigins, signaling.ServerOptions{
		EnableDevTokenIssuer: enableDevTokens,
	})
	if err != nil {
		log.Fatalf("Invalid signaling configuration: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", server)
	mux.Handle("/dev/token", server)

	log.Println("NexRTC SFU Server Starting...")
	log.Println("WebSocket signaling server listening on :8080")
	log.Printf("Allowed browser origins: %s", strings.Join(allowedOrigins, ", "))

	listenAddress := ":8080"
	if enableDevTokens {
		listenAddress = "127.0.0.1:8080"
		log.Printf("Local development token issuer enabled at http://127.0.0.1:8080/dev/token; server bound to loopback")
	} else {
		log.Println("Local development token issuer disabled")
	}
	httpServer := &http.Server{
		Addr:              listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(httpServer.ListenAndServe())
}

func splitOrigins(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	origins := strings.Split(value, ",")
	for index := range origins {
		origins[index] = strings.TrimSpace(origins[index])
	}
	return origins
}
