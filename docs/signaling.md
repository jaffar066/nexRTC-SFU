# Signaling authentication and protocol

The signaling endpoint authenticates each WebSocket before accepting room or
peer messages. The client must send `authenticate` as its first message with a
short-lived HS256 JWT. Required token claims are:

- `sub`: participant ID (1-128 characters)
- `room_id`: the one room the participant may join (1-128 characters)
- `iss`: configured issuer
- `aud`: configured audience
- `iat`: issued-at time; tokens may be valid for at most 15 minutes
- `exp`: required expiration time

The server assigns `userId` from the verified `sub`; a client-supplied identity
is rejected. A participant may join only the room in `room_id`. Offers, answers,
and ICE candidates are forwarded only to a target currently in that same room.

## Server configuration

Set these environment variables before starting `go run ./cmd/server`:

- `NEXRTC_JWT_SECRET`: at least 32 bytes of random secret material, shared only
  with the trusted token issuer.
- `NEXRTC_JWT_ISSUER`: exact JWT issuer.
- `NEXRTC_JWT_AUDIENCE`: expected JWT audience.
- `NEXRTC_ALLOWED_ORIGINS`: comma-separated exact browser origins, such as
  `http://localhost:5500`. Wildcard origins are rejected.

Example PowerShell setup for local development:

```powershell
$env:NEXRTC_JWT_SECRET = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$env:NEXRTC_JWT_ISSUER = "nexrtc-local"
$env:NEXRTC_JWT_AUDIENCE = "nexrtc"
$env:NEXRTC_ALLOWED_ORIGINS = "http://localhost:5500"
$env:NEXRTC_ENABLE_DEV_TOKEN_ISSUER = "true"
go run ./cmd/server
```

Production tokens must be issued by a trusted identity service using the
configured claims and signing secret. Never use a development secret in
production. The server closes an authenticated WebSocket when the token expires
and sends a `token-expired` event immediately before closing; the client must
reconnect with a fresh token. The server currently speaks plain HTTP for local
development; production deployments must terminate TLS and expose the endpoint
as `wss://`.

### Local demo token issuer

For local learning/demo only, set `NEXRTC_ENABLE_DEV_TOKEN_ISSUER=true`. The
server then binds to `127.0.0.1:8080` and exposes `POST /dev/token`. The browser
automatically requests a room-scoped, 15-minute token when Join Room is
clicked, so no token needs to be pasted. Each request receives a random
participant ID. Anyone running a browser on the same computer who knows the
room ID can join; this endpoint is not an account login and must never be
enabled in a production deployment.

When this flag is disabled (the default), `/dev/token` does not exist and a
trusted identity provider must supply the token to the client. In either mode,
the client sends the token in the first WebSocket message, not in the URL or
query string.

## Client message envelope

Messages are JSON objects with a required `type` and optional `requestId`,
`roomId`, `target`, and `data`. Unknown envelope fields and message types are
rejected. The server supplies `userId` in server-to-client messages.

Client messages:

- `authenticate`: first frame only; `data` is `{ "token": "<JWT>" }`.
- `join`: `roomId` must equal the token's `room_id`.
- `offer` / `answer`: `target` plus `data.sdp` containing the matching SDP type
  and a non-empty SDP string.
- `ice-candidate`: `target` plus `data.candidate`, with optional standard ICE
  candidate fields.

Successful authentication returns `authenticated` with `data.expiresAt`.
Joining returns `joined` and `existing-peers`; room peers receive `peer-joined`
and `peer-left`.
Validation and authorization failures return an `error` with `data.code` and
`data.message`. SDP and candidate messages retain the existing peer-to-peer
demo behavior; the SFU media plane is a later milestone.
