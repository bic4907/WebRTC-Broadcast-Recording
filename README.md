# WebRTC Broadcasting Recording

## Feature
- One Broadcaster:Multiple Subscriber (1:N SFU)
- Subscribers listen events `Broadcaster Connected`, `Broadcaster Unconnected` (No need to refresh a page)
- Recording Broadcaster's Tracks as `.mp4` using ffmpeg
- Go-lang SFU server, web-based demostration client
- Signaling (Offer, Answer) using gorilla websocket
