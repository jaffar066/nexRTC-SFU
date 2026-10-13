        // ===== Global state =====
        let socket = null;
        let localStream = null;
        let myUserId = null;
        let currentRoom = null;

        const peers = new Map(); 
        const pendingIceCandidates = new Map();

        const roomInput = document.getElementById("roomInput");
        const joinButton = document.getElementById("joinButton");
        const statusEl = document.getElementById("status");
        const usersDisplay = document.getElementById("users");
        const videoGrid = document.getElementById("videoGrid");

        // ===== UI helpers =====
        function setStatus(message) {
            statusEl.textContent = message;
            console.log(message);
        }

        function updateUsers() {
            // peers contains only remote users, so +1 for local
            usersDisplay.textContent = `Users: ${localStream ? peers.size + 1 : peers.size}`;
        }

        function createTile(id, label, stream, muted) {
            const tile = document.createElement("div");
            tile.className = "video-tile";
            if (id) tile.id = id;

            const video = document.createElement("video");
            video.autoplay = true;
            video.playsInline = true;
            video.muted = muted;
            if (stream) {
                video.srcObject = stream;
            }
            const labelEl = document.createElement("div");
            labelEl.className = "video-label";
            labelEl.textContent = label;
            tile.append(video, labelEl);
            videoGrid.appendChild(tile);
            return video;
        }

        function createRemoteVideo(remoteUserId) {
            const existing = document.getElementById(`video-${remoteUserId}`);
            if (existing){
                return existing.querySelector("video");
            }
            return createTile(`video-${remoteUserId}`, remoteUserId, null, false);
        }

        function removeRemoteVideo(remoteUserId) {
            document.getElementById(`video-${remoteUserId}`)?.remove();
        }

        // ===== Camera + microphone =====
        async function startCamera() {
            setStatus("Requesting camera and microphone...");
            localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
            createTile(null, `${myUserId || "You"} (You)`, localStream, true);
            console.log("Camera and microphone ready");
        }

        // ===== Signaling =====
        function sendSignalingMessage(message) {
            if (!socket || socket.readyState !== WebSocket.OPEN) {
                console.error("WebSocket is not connected");
                return;
            }
            socket.send(JSON.stringify(message));
        }

        // ===== Peer connections =====
        function createPeerConnection(remoteUserId) {
            const existing = peers.get(remoteUserId);
            if (existing){
                return existing;
            }

            console.log("Creating PeerConnection for:", remoteUserId);

            const remoteVideo = createRemoteVideo(remoteUserId);
            const pc = new RTCPeerConnection({
                iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
            });

            // Send our camera + mic to this peer
            localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

            // Forward local ICE candidates to the remote user
            pc.onicecandidate = event => {
                if (!event.candidate) return;
                sendSignalingMessage({
                    type: "ice-candidate",
                    target: remoteUserId,
                    data: { candidate: event.candidate }
                });
            };

            // Receive remote audio/video
            pc.ontrack = event => {
                console.log("Remote track received:", remoteUserId, event.track.kind);
                if (event.streams && event.streams[0]) {
                    remoteVideo.srcObject = event.streams[0];
                }
            };

            pc.oniceconnectionstatechange = () => {
                console.log(remoteUserId, "ICE:", pc.iceConnectionState);
            };

            pc.onconnectionstatechange = () => {
                console.log(remoteUserId, "Connection:", pc.connectionState);
                if (pc.connectionState === "failed" || pc.connectionState === "closed") {
                    removePeer(remoteUserId);
                }
            };

            peers.set(remoteUserId, pc);
            updateUsers();
            return pc;
        }

        function removePeer(remoteUserId) {
            peers.get(remoteUserId)?.close();
            peers.delete(remoteUserId);
            pendingIceCandidates.delete(remoteUserId);
            removeRemoteVideo(remoteUserId);
            updateUsers();
        }

        // ===== Offer / Answer / ICE =====
        async function createOffer(remoteUserId) {
            const pc = createPeerConnection(remoteUserId);
            console.log("Creating OFFER for:", remoteUserId);

            await pc.setLocalDescription(await pc.createOffer());
            sendSignalingMessage({
                type: "offer",
                target: remoteUserId,
                data: { sdp: pc.localDescription }
            });
            console.log("OFFER sent to:", remoteUserId);
        }

        async function handleOffer(message) {
            const remoteUserId = message.userId;
            const pc = createPeerConnection(remoteUserId);
            console.log("OFFER received from:", remoteUserId);

            await pc.setRemoteDescription(message.data.sdp);
            await flushPendingIce(remoteUserId);

            await pc.setLocalDescription(await pc.createAnswer());
            sendSignalingMessage({
                type: "answer",
                target: remoteUserId,
                data: { sdp: pc.localDescription }
            });
            console.log("ANSWER sent to:", remoteUserId);
        }

        async function handleAnswer(message) {
            const remoteUserId = message.userId;
            const pc = peers.get(remoteUserId);

            if (!pc) {
                console.error("PeerConnection not found:", remoteUserId);
                return;
            }

            console.log("ANSWER received from:", remoteUserId);
            await pc.setRemoteDescription(message.data.sdp);
            await flushPendingIce(remoteUserId);
        }

        async function handleIceCandidate(message) {
            const remoteUserId = message.userId;
            const candidate = message.data.candidate;
            const pc = createPeerConnection(remoteUserId);

            // Remote SDP not set yet -> queue the candidate
            if (!pc.remoteDescription) {
                console.log("Remote description not ready. ICE queued:", remoteUserId);
                if (!pendingIceCandidates.has(remoteUserId)) {
                    pendingIceCandidates.set(remoteUserId, []);
                }
                pendingIceCandidates.get(remoteUserId).push(candidate);
                return;
            }

            try {
                await pc.addIceCandidate(candidate);
                console.log("ICE candidate added:", remoteUserId);
            } catch (error) {
                console.error("Failed to add ICE:", error);
            }
        }

        async function flushPendingIce(remoteUserId) {
            const pc = peers.get(remoteUserId);
            const candidates = pendingIceCandidates.get(remoteUserId);
            if (!pc || !candidates) return;

            console.log("Flushing ICE:", remoteUserId, candidates.length);

            for (const candidate of candidates) {
                try {
                    await pc.addIceCandidate(candidate);
                } catch (error) {
                    console.error("Queued ICE failed:", error);
                }
            }

            pendingIceCandidates.delete(remoteUserId);
        }

        // ===== WebSocket =====
        function connectWebSocket() {
            return new Promise((resolve, reject) => {
                socket = new WebSocket("ws://localhost:8080/ws");

                socket.onopen = () => {
                    console.log("WebSocket connected");
                    setStatus("Connected to signaling server");
                    resolve();
                };

                socket.onmessage = async event => {
                    const message = JSON.parse(event.data);
                    console.log("Received:", message);
                    await handleMessage(message);
                };

                socket.onerror = error => {
                    console.error("WebSocket error:", error);
                    reject(error);
                };

                socket.onclose = () => setStatus("Disconnected from signaling server");
            });
        }

        function joinRoom() {
            const roomId = roomInput.value.trim();
            if (!roomId) {
                alert("Enter a room ID");
                return;
            }

            currentRoom = roomId;
            sendSignalingMessage({ type: "join", roomId });
            setStatus(`Joining room: ${roomId}`);
        }

        // ===== Server message handler =====
        async function handleMessage(message) {
            switch (message.type) {
                case "joined": {
                    myUserId = message.userId;
                    currentRoom = message.roomId;
                    setStatus(`Joined ${currentRoom} as ${myUserId}`);

                    const localLabel = videoGrid.querySelector(".video-label");
                    if (localLabel) localLabel.textContent = `${myUserId} (You)`;

                    updateUsers();
                    break;
                }

                // New user receives the list of existing users and sends an OFFER to each
                case "existing-peers":
                    console.log("Existing peers:", message.data);
                    for (const remoteUserId of message.data) {
                        await createOffer(remoteUserId);
                    }
                    break;

                // Existing users only get notified; the new user creates the offer
                case "peer-joined":
                    console.log("New peer joined:", message.userId);
                    setStatus(`${message.userId} joined the room`);
                    break;

                case "offer":
                    await handleOffer(message);
                    break;

                case "answer":
                    await handleAnswer(message);
                    break;

                case "ice-candidate":
                    await handleIceCandidate(message);
                    break;

                case "peer-left":
                    console.log("Peer left:", message.userId);
                    removePeer(message.userId);
                    setStatus(`${message.userId} left`);
                    break;

                case "error":
                    console.error(message.data);
                    setStatus(`Error: ${message.data}`);
                    break;

                default:
                    console.log("Unknown message:", message);
            }
        }

        // ===== Start =====
        joinButton.addEventListener("click", joinRoom);

        async function start() {
            try {
                await startCamera();
                await connectWebSocket();
                setStatus("Ready. Enter a room and click Join Room.");
            } catch (error) {
                console.error(error);
                setStatus("Failed to start NexRTC");
            }
        }

        start();