package wrtc

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"strings"
)

type Subscriber struct {
	Pc          *webrtc.PeerConnection
	Ws 			*websocket.Conn

	Uid         uuid.UUID

	LastHit   	int64

	UserId      string
	RoomId      string
	BroadcastId string
}




func MakeSubscriberPeerConnection(description webrtc.SessionDescription, subscriber *Subscriber) *webrtc.PeerConnection {

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

	subscriber.Pc = pc

	var videoCheckerChannel *webrtc.DataChannel = nil


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

		subscriber.Ws.WriteMessage(1, message)

	})


	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		if d.Label() == "health-check" {
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				arr := strings.Split(string(msg.Data), "-")
				d.SendText("pong-" + arr[1])
				subscriber.LastHit = makeTimestamp()
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



