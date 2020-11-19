# WebRTC Broadcasting Recording

## Features
- Multiple Broadcaster, multiple subscribers architecture (1:N SFU)
- Subscribers listen events `Broadcaster Connected`, `Broadcaster Unconnected`
- Recording Broadcaster's Tracks as `.mp4` using ffmpeg
- Go-lang SFU server, web-based demostration client
- Signaling (Offer, Answer) using gorilla websocket

## How To Run
WIP

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
On your command line, `ffmpeg` command should be available. FFmpeg is needed to recording your video and audio track as a MPEG4 media file.

WIP


## Architecture
Our repository is Server (Go)-Client (Javascript) architecture. 
  
WIP


## Author
- [In-chang Baek](https://github.com/bic4907)

## References
- [pion/webrtc](https://github.com/pion/webrtc)
- [pion/example-webrtc-applications](https://github.com/pion/example-webrtc-applications)
- [gorilla/websocket](https://github.com/gorilla/websocket)  

Those are most useful materials for you understanding webrtc sigaling paradism.  
Keep in your mind that websocket communication is one of lots of signaling methods. 
