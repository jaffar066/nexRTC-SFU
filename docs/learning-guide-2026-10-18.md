# Aaj ka kaam: Token, signaling aur pehli peer connection

Yeh guide 2026-10-18 ko NexRTC mein kiye gaye signaling/authentication kaam ko
step by step samjhati hai. Maqsad sirf chalana nahi, balkay yeh samajhna bhi hai
ke browser aur server ek doosre se kya baat karte hain.

## 1. Aaj kya bana?

- Room, participant, peer session, transport, producer aur consumer ke basic
  lifecycle/state models.
- WebSocket signaling ke liye JWT authentication aur room authorization.
- Har browser tab ke liye local demo token issue karne ka endpoint; token manually
  paste karne ki zaroorat nahi.
- Offer, answer aur ICE message validation.
- Token expire hone par WebSocket band karna aur browser ko `token-expired`
  event bhejna.
- Browser/server logs, localhost/127.0.0.1 development origin handling, aur
  ICE candidate format ka browser/server ke darmiyan fix.

### Lifecycle models ka matlab

Lifecycle model object ki zindagi ke marahil ko wazeh karta hai. Is se code ko
maloom hota hai ke abhi koi room, peer ya media object kis state mein hai aur
agla kaunsa action allowed hai.

- **Room:** `NEW -> ACTIVE -> CLOSING -> CLOSED`. Band hota room naye
  participants accept nahi karta; close par apne participants ki membership
  saaf karta hai.
- **Participant:** `JOINING -> ACTIVE -> DISCONNECTED -> LEFT`. Temporary
  disconnect aur room se mukammal leave alag baatein hain.
- **Peer session:** `NEW -> CONNECTING -> ACTIVE`, ya connection ke dauran
  `FAILED`; phir cleanup ke liye `CLOSING -> CLOSED`.
- **Transport:** `NEW -> CONNECTING -> CONNECTED`; failure ya cleanup ke liye
  `FAILED`, `CLOSING`, aur `CLOSED`. Transport, peer session se alag hai:
  session ka status overall peer coordination batata hai aur transport network
  connection ka.
- **Producer:** participant ka bheja hua audio/video source.
- **Consumer:** kisi producer ko receive karne wala subscription. Consumer ko
  pause karne se producer ki apni state nahi badalni chahiye.
- **Mute aur pause:** mute producer ke audio/video output ki condition hai;
  pause media forwarding/subscription rokne ka lifecycle action hai. Dono ko
  alag rakhne se unka behavior mix nahi hota.

Yeh Go domain models aur unke state-transition tests hain. Abhi live browser
signaling/peer-connection code in models ko apne har action par update nahi
karta. Yeh aglay integration work ke liye explicit building blocks hain; inka
hona apne aap production SFU ya media forwarding nahi banata.

## 2. Token ko simple alfaaz mein samjhein

Token ko temporary entry-pass samjhein. Is project mein JWT use hota hai. JWT
mein teen hisson ka text hota hai:

1. **Header**: batata hai token kis signing algorithm se bana, yahan `HS256`.
2. **Payload/claims**: participant, room, issuer, audience aur time ki
   maloomat.
3. **Signature**: server ko verify karne deti hai ke trusted signer ne token
   banaya aur payload baad mein badla nahi.

JWT ka payload aam tor par **encrypted nahi hota**. Isay decode karke padha ja
sakta hai. Is liye password ya secret payload mein na rakhein. Token ko
password ki tarah private rakhein aur HTTPS/WSS par bhejein.

### Is project ke claims

| Claim | Matlab |
| --- | --- |
| `sub` | Participant ki unique ID |
| `room_id` | Woh room jahan participant ko ijazat hai |
| `iss` | Token kis trusted issuer ne banaya |
| `aud` | Token kis application/service ke liye hai |
| `iat` | Token kab issue hua |
| `exp` | Token kab expire hoga |

Server `sub`, `iss`, `aud`, `iat`, aur `exp` validate karta hai. Iss token ki
maximum lifetime **15 minutes** hai. Client khud `userId` bhej kar apni identity
set nahi kar sakta; server identity token ke verified `sub` se leta hai.

## 3. Local demo mein token kahan se aata hai?

Normal production app mein login/identity service user ko authenticate karke
token deti hai. Is repository mein aisa login service abhi nahi hai. Is liye
local learning ke liye server mein development-only `POST /dev/token` endpoint
hai.

Jab `NEXRTC_ENABLE_DEV_TOKEN_ISSUER=true` ho:

- Browser Join button par `roomId` ko `/dev/token` par bhejta hai.
- Server random participant ID banata hai, misal `user-...`.
- Server us ID aur room ke claims ke saath 15-minute JWT sign karta hai.
- Browser token ko WebSocket ke pehle `authenticate` message mein bhejta hai.
- Console mein token khud print nahi hota; sirf participant ID, room aur expiry
  metadata log hoti hai.

Yeh **login nahi hai**. Local demo endpoint chal raha ho to usi computer par
browser kholne wala koi bhi shakhs room ID jaan kar join kar sakta hai. Isay
production mein enable na karein. Endpoint enable karne par server sirf
loopback (`127.0.0.1`) par bind hota hai.

## 4. Browser se server tak poora flow

```text
Browser tab
  | 1. Join Room: roomId
  v
POST /dev/token ------------------------> Go server
  ^                                        | random participant ID
  | token + participant ID + expiry       | JWT sign
  +----------------------------------------+
  |
  | 2. WebSocket connect to /ws
  | 3. First frame: authenticate {token}
  v
Go server: JWT signature/claims verify
  |
  | authenticated {userId, expiresAt}
  v
Browser
  |
  | 4. join {roomId}
  v
Go server checks token room_id == requested room
  |
  | joined + existing-peers
  | peer-joined to users already in room
  v
Browser tabs exchange offer / answer / ICE through server
```

### Step by step

1. `getUserMedia()` browser se camera/microphone ki ijazat leta hai. Is stage
   par token ya server connection ki zaroorat nahi.
2. Join button dabane par `requestDevelopmentToken()` demo endpoint ko call
   karta hai.
3. Browser WebSocket kholta hai aur pehla message `authenticate` bhejta hai.
4. Server signature aur claims verify karta hai. Kamyab ho to `authenticated`
   reply mein participant ID aur expiry deta hai.
5. Browser `join` message bhejta hai. Server check karta hai ke token ka
   `room_id`, requested room se match karta ho.
6. Naye participant ko `joined` aur `existing-peers` milta hai. Room ke purane
   participants ko `peer-joined` milta hai.
7. Naya browser har existing peer ke saath WebRTC offer banata hai. Doosra
   browser answer bhejta hai.
8. Dono taraf se ICE candidates aate hain. Agar remote SDP abhi set na hui ho,
   browser candidate ko queue karta hai. SDP set hone ke baad queue flush karke
   candidate ko `addIceCandidate()` deta hai.
9. Peer connection establish hone par browser `ontrack` se remote audio/video
   dikhata hai.

## 5. Offer, answer aur ICE ka kaam

WebRTC connection banane ke liye browsers ko pehle connection ki settings
exchange karni hoti hain:

- **Offer**: pehla browser batata hai ke woh kaun se media tracks/codecs ke
  saath connect hona chahta hai.
- **Answer**: doosra browser compatible jawab deta hai.
- **ICE candidate**: network par doosre browser tak pohanchne ke mumkin raaste
  (addresses/ports/transport details) batata hai.

Yeh messages signaling server se guzarte hain, lekin server inhein abhi sirf
forward karta hai. ICE candidate ka payload ek object hota hai, misal:

```json
{
  "type": "ice-candidate",
  "target": "doosre-participant",
  "data": {
    "candidate": "candidate:...",
    "sdpMid": "0",
    "sdpMLineIndex": 0
  }
}
```

`RTCPeerConnection.addIceCandidate()` ko poora object chahiye, sirf
`data.candidate` string nahi. Browser local candidate ko `toJSON()` se serialize
karta hai aur remote candidate ka poora object add karta hai. Candidate remote
SDP se pehle aajaye to `pendingIceCandidates` mein temporarily rakha jata hai.

## 6. Token expire hone par kya hota hai?

Server token ke `exp` time par active WebSocket ko band karta hai:

1. Server log mein `Token expired` aata hai.
2. Browser ko `token-expired` message bheja jata hai.
3. Browser Console mein expiry warning aati hai aur local peer connections
   close hote hain.
4. Server WebSocket close karta hai aur participant ko room se nikalta hai;
   baqi participants ko `peer-left` milta hai.
5. Dobara join karne ke liye fresh token chahiye. Local demo mein Join dobara
   dabane se naya token request hota hai.

Token expiry calendar/date-time string mein server aur browser ke local
timezone ke mutabiq alag dikh sakti hai. Misal ke taur par `Z` UTC hai, jabke
`+05:00` Pakistan Standard Time hai; dono ek hi waqt ko represent kar sakte
hain.

## 7. Browser console aur server terminal dekhna

Browser mein `F12` dabayein aur **Console** tab kholein. Wahan yeh useful
messages dikhte hain:

- `[auth] Local demo token issued` - test token bana.
- `[auth] Authentication succeeded` - WebSocket authentication kamyab.
- `[room] Joined room` - server ne room membership confirm ki.
- `[room] Peer joined after server authentication` - doosra user authenticate
  hokar room mein aaya.
- `Creating OFFER`, `ANSWER received`, `ICE candidate added`, `Remote track
  received` - WebRTC setup ke steps.
- `[auth] Token expired` - token expire hua aur connection band ho raha hai.

Server terminal mein `Participant authenticated`, `Participant joined room`,
aur `Token expired` logs dikhte hain. JWT ki poori string log nahi honi chahiye;
token private credential hai.

## 8. Aaj ke errors aur unka sabab

### `Signaling URL and room token are required`

Purana browser flow page load hote hi token maang raha tha, lekin token paste
nahi hua tha. Flow badal kar ab token Join button par automatically issue hota
hai.

### CORS error `/dev/token`

Browser origin mein hostname aur port dono shamil hote hain. Yeh do origins
alag hain:

- `http://localhost:5500`
- `http://127.0.0.1:5500`

Local demo mode ab in loopback aliases ko accept karta hai, agar ek configured
ho. Server ka purana process chal raha ho to code change load nahi hota; Go
server ko stop karke dobara start karna hota hai.

### `ICE candidate data is invalid`

Browser candidate object ko `data.candidate` ke andar nested bhej raha tha,
jabke Go server ko `data` ke andar candidate string chahiye thi. Browser
serialization ko `event.candidate.toJSON()` par align kiya gaya.

### `provided value is not of type RTCIceCandidateInit`

Remote message mein candidate fields `message.data` mein hoti hain. Browser
pehle sirf `message.data.candidate` string pass kar raha tha. Ab woh full
candidate object pass karta hai. Agar remote SDP abhi nahi aayi hoti to isi
object ko queue karke baad mein add karta hai.

### `favicon.ico` ka 404

Yeh browser ka optional icon request hai. Camera, token, signaling ya media
connection ki failure nahi.

## 9. Project ko local par chalana

PowerShell **Terminal 1** mein:

```powershell
Set-Location D:\NexRTC
$env:NEXRTC_JWT_SECRET = "nexrtc-local-only-secret-do-not-use-in-production"
$env:NEXRTC_JWT_ISSUER = "nexrtc-local"
$env:NEXRTC_JWT_AUDIENCE = "nexrtc"
$env:NEXRTC_ALLOWED_ORIGINS = "http://localhost:5500"
$env:NEXRTC_ENABLE_DEV_TOKEN_ISSUER = "true"
go run ./cmd/server
```

PowerShell **Terminal 2** mein:

```powershell
Set-Location D:\NexRTC
py -m http.server 5500 --directory client
```

Phir `http://127.0.0.1:5500` ya `http://localhost:5500` kholein. Do browser
tabs mein ek hi room ID enter karke Join Room dabayein. Dono tabs ko alag random
participant ID aur token milega.

Go server ka process pehle se chal raha ho to `Ctrl+C` se band karke dobara
start karein. Browser ke purane JavaScript ke liye page hard refresh (`Ctrl+F5`)
karna madadgar hai.

## 10. Architecture ki aham had

Browser mein remote video/audio kaam karna yeh sabit karta hai ke **current
browser-to-browser WebRTC mesh demo** chal raha hai. Iska matlab yeh nahi ke Go
server abhi SFU media forward kar raha hai:

- Go server WebSocket signaling relay karta hai.
- Har browser apne remote peers ke saath `RTCPeerConnection` banata hai.
- Media abhi browser se browser tak jaati hai.
- ICE mein STUN configured hai; production NAT traversal aur relay ke liye
  TURN, aur SFU transport ke liye ICE/DTLS-SRTP integration aglay kaam hain.

## 11. Code kahan dekhna hai?

- Token verify/issue aur claims: [`internal/signaling/auth.go`](../internal/signaling/auth.go)
- Message types aur payload validation: [`internal/signaling/protocol.go`](../internal/signaling/protocol.go)
- WebSocket, room membership, expiry aur routing:
  [`internal/signaling/server.go`](../internal/signaling/server.go)
- Local environment variables/server startup: [`cmd/server/main.go`](../cmd/server/main.go)
- Browser token, WebSocket, offer/answer aur ICE:
  [`client/index.js`](../client/index.js)
- Short protocol/configuration reference: [`signaling.md`](./signaling.md)

## 12. Verify karne ke liye tests

Project root se:

```powershell
go test ./... -count=1
go vet ./...
node --check client\index.js
```

Signaling tests JWT claims, room authorization, local token endpoint, CORS
preflight, WebSocket messaging, expiry notification, aur candidate forwarding
check karte hain.
