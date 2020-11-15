package wrtc

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"io"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Broadcaster struct {
	Pc *webrtc.PeerConnection
	Ws *websocket.Conn

	Uid 	   uuid.UUID
	Recorder   *VideoRecorder

	LastHit   int64

	UserId      string
	RoomId      string
	BroadcastId string
}

func MakeBroadcasterPeerConnection(description webrtc.SessionDescription, broadcaster *Broadcaster) *webrtc.PeerConnection {

	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))


	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	pc, err := api.NewPeerConnection(WebRTCConfig)
	if err != nil {
		panic(err)
	}

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	recorder := newVideoRecorder()
	broadcaster.Recorder = recorder
	broadcaster.Pc = pc
	recorder.Broadcaster = broadcaster

	var videoCheckerChannel *webrtc.DataChannel = nil

	pc.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {

		if track.Kind() == webrtc.RTPCodecTypeVideo {

			go func() {
				ticker := time.NewTicker(time.Second * 1)
				for range ticker.C {
					errSend := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
					if errSend != nil {
						break
					}
				}
			}()

			go func() {
				ticker := time.NewTicker(time.Second * 1)
				for range ticker.C {
					if broadcaster.LastHit != -1 && makeTimestamp()-broadcaster.LastHit > 3000 {
						log(broadcaster.Uid, fmt.Sprintf("Closed with Time-out"))

						broadcaster.Recorder.Close()
						broadcaster.Pc.Close()
						return
					}

					if videoCheckerChannel != nil {
						videoCheckerChannel.SendText("video-ok")
					}
				}
			}()
		}

		for {

			rtp, err := track.ReadRTP()
			if err != nil {
				if err == io.EOF {
					return
				}
				panic(err)
			}


			switch track.Kind() {
			case webrtc.RTPCodecTypeAudio:
				broadcaster.Recorder.PushOpus(rtp)

			case webrtc.RTPCodecTypeVideo:
				broadcaster.Recorder.PushVP8(rtp)
			}

		}
	})


	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		b, err := json.Marshal(c.ToJSON())
		if err != nil {
			panic(err)
		}
		actualSerialized := string(b)

		payload := make(map[string]interface{})
		payload["type"] = "iceCandidate"
		payload["message"] = actualSerialized
		message, _ := json.Marshal(payload)

		broadcaster.Ws.WriteMessage(1, message)

	})


	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		if d.Label() == "health-check" {
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				arr := strings.Split(string(msg.Data), "-")
				d.SendText("pong-" + arr[1])
				broadcaster.LastHit = makeTimestamp()
			})
		}

		if d.Label() == "video-check" {
			videoCheckerChannel = d
		}
	})


	err = pc.SetRemoteDescription(description)
	if err != nil {
		panic(err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	return pc
}







