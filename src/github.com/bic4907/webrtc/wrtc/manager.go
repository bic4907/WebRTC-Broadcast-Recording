package wrtc

import (
	"fmt"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/uuid"
	"io"
	"strings"
	"time"

	webrtcsignal "github.com/bic4907/webrtc/internal/signal"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	pc       *webrtc.PeerConnection
	dc       *webrtc.DataChannel
	id       uuid.UUID
	last_hit *timestamp.Timestamp
	recorder *VideoRecorder
}

func CreatePeerConnection(token string) []byte {
	recorder := newVideoRecorder()
	_, answer := createWebRTCConn(recorder, token)

	return []byte(answer)
}

func createWebRTCConn(recorder *VideoRecorder, token string) (Client, string) {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a MediaEngine object to configure the supported codec
	m := webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// Only support VP8 and OPUS, this makes our WebM muxer code simpler
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	uuid4, _ := uuid.NewRandom()

	var client = Client{
		pc:       peerConnection,
		dc:       nil,
		id:       uuid4,
		last_hit: nil,
		recorder: recorder,
	}
	recorder.client = client
	client.recorder = recorder

	if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {

		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					break
				}
			}
		}()

		log(client.id, fmt.Sprintf("Track has started, of type %d: %s", track.PayloadType(), track.Codec().Name))

		if track.Kind() == webrtc.RTPCodecTypeVideo {
			go func() {
				ticker := time.NewTicker(time.Second * 1)
				for range ticker.C {
					if client.dc != nil && client.pc.ConnectionState().String() == "connected" {
						client.dc.SendText("video-ok")
					}
				}
			}()
		}

		for {
			rtp, readErr := track.ReadRTP()

			//log(client.id, "HI")

			if readErr != nil {
				if readErr == io.EOF {
					return
				}
				panic(readErr)
			}
			switch track.Kind() {
			case webrtc.RTPCodecTypeAudio:
				recorder.PushOpus(rtp)

			case webrtc.RTPCodecTypeVideo:
				recorder.PushVP8(rtp)
			}
		}
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		if d.Label() == "health-check" {
			client.dc = d
		}

		d.OnOpen(func() {
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			arr := strings.Split(string(msg.Data), "-")
			d.SendText("pong-" + arr[1])
		})

	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log(client.id, fmt.Sprintf("Connection State has changed %s", connectionState.String()))

		if connectionState.String() == "disconnected" {
			client.recorder.Close()
		}

	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	webrtcsignal.Decode(token, &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete
	// Output the answer in base64 so we can paste it in browser
	//fmt.Println(webrtcsignal.Encode(answer))

	return client, webrtcsignal.Encode(answer)
}
