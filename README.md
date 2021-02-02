# WebRTC Broadcasting Recording
This repository is not a peer-to-peer example. The server totally cares signaling and broadcasting

## Features
- Multiple Broadcaster, multiple subscribers architecture (1:N SFU)
- Subscribers listen events `Broadcaster Connected`, `Broadcaster Disconnected`
- Recording Broadcaster's Tracks as `.mp4` using ffmpeg
- Go-lang SFU server, web-based demostration client
- Signaling (Offer, Answer) using gorilla websocket

## How To Run

### Dockerfile
You need to install Docker (Windows, Linux, MacOS) on your computer.

```
cd WebRTC-Broadcast-Recording
docker-compose up
```
You also change exposed port using docker-compose's port-forwarding option.
Our Dockerfile will automatically install go-lang and FFmpeg on docker image.


### Go compiling
If you didn't install FFmpeg, you need to install your own version of FFmpeg binaries.
On your command line, `ffmpeg` command should be available. FFmpeg is needed to record your video and audio track as a MPEG4 media file.

```
cd src/github.com/bic4907/webrtc   
go run main.go
```

## Architecture
Our repository is Server (Go)-Client (Javascript) architecture. 
WIP  


### Hub Events 
Some events regarding broadcasting service. Please check out [hub.go](/src/github.com/bic4907/webrtc/web/hub.go) source code.   
   
```Register```
A broadcaster event when a broadcaster joined to the server. The broadcaster offers to begin audio/video track and negotiate with server. When broadcaster begin to send RTC packet, `BroadcastBroadcasterEntered` method will be executed. This method will notify to users that already connected to server via a websocket event. Then, clients and server will re-negotiate to make new media track.  
When the broadcaster starts to send RTC packets, the server will record video/audio using ffmpeg.

```Unregister```
A broadcaster event when a broadcaster exited from the server. This event will execute `BroadcastBroadcasterExited` method, and the server will make user to remove tracks from player.
  
```Subscribe```
A subscriber event when a receiver joined to the server. The hub will find whether the broadcaster is existing and offer video/audio tracks to user via websocket. If the broadcaster not joined yet, the server do not anything and the user will be wait for the broadcaster join.
  
```Unsubscribe```
A subscriber event when a receiver exited from the server. The hub will remove user from a room's user list.
  
```Broadcast```
A broadcaster event when a broadcaster sends a RTC packet. The hub publishes this packet to the users where the broadcaster have opened.
  

## Author
- [In-chang Baek](https://github.com/bic4907)
### Special thanks
- [JooYoung Lim](https://github.com/DevRockstarZ)

## References
- [pion/webrtc](https://github.com/pion/webrtc)
- [pion/example-webrtc-applications](https://github.com/pion/example-webrtc-applications)
- [gorilla/websocket](https://github.com/gorilla/websocket)  

Those are most useful materials for you understanding webrtc sigaling paradism.  
Keep in your mind that websocket communication is just one of lots of signaling methods. 
