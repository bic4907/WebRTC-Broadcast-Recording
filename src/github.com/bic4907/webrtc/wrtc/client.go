package wrtc

import (
	list "container/list"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"

	webrtcsignal "github.com/bic4907/webrtc/internal/signal"
	"github.com/pion/webrtc/v3"
)

var (
	viewers map[string]*Viewer = map[string]*Viewer{}
)

type Viewer struct {
	pc         *webrtc.PeerConnection
	dc         *webrtc.DataChannel
	id         uuid.UUID
	last_hit   int64
	target_uid string
	candidates *list.List

	videoTrack chan *webrtc.Track
	audioTrack chan *webrtc.Track
}

func CreateViewerConnection(token string, target_uid string) (string, string) {

	client, answer := createViewerConn(token, target_uid)
	viewers[client.id.String()] = &client

	return client.id.String(), answer
}

func AddCandidateToViewerConnection(uid string, candidate string) (string, string) {
	client, _ := viewers[uid]

	log(client.id, candidate)

	var actual webrtc.ICECandidateInit
	json.Unmarshal([]byte(candidate), &actual)

	client.pc.AddICECandidate(actual)

	return client.id.String(), webrtcsignal.Encode(client.pc.LocalDescription())
}

func GetCandidateToViewerConnection(uid string) (string, string, string) {
	client, _ := viewers[uid]

	var output []string = []string{}
	for {
		if client.candidates.Len() == 0 {
			break
		}

		var str = (client.candidates.Remove(client.candidates.Front())).(string)
		output = append(output, str)
	}

	outputStr, err := json.Marshal(output)
	if err != nil {
		fmt.Println(err)
	}

	return client.id.String(), webrtcsignal.Encode(client.pc.LocalDescription()), string(outputStr)
}

func createViewerConn(token string, target_uid string) (Viewer, string) {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
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

	var client = Viewer{
		pc:         peerConnection,
		dc:         nil,
		id:         uuid4,
		last_hit:   -1,
		candidates: list.New(),
		target_uid: target_uid,
	}

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		b, err := json.Marshal(c.ToJSON())
		if err != nil {
		}
		actualSerialized := string(b)

		log(client.id, actualSerialized)

		client.candidates.PushBack(actualSerialized)
	})

	//go func() {
	//	ticker := time.NewTicker(time.Second * 3)
	//	for range ticker.C {
	//		errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
	//		if errSend != nil {
	//			break
	//		}
	//	}
	//}()
	//
	//

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		if d.Label() == "health-check" {
			client.dc = d
		}

		d.OnOpen(func() {
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			arr := strings.Split(string(msg.Data), "-")
			d.SendText("pong-" + arr[1])

			client.last_hit = makeTimestamp()
		})

	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log(client.id, fmt.Sprintf("Connection State has changed %s", connectionState.String()))

		if connectionState.String() == "disconnected" {
			//client.recorder.Close()
		} else if connectionState.String() == "connected" {
			for {
				if (client.videoTrack == nil) || (client.audioTrack == nil) {

					//log(client.id, fmt.Sprintf("Track connecting to %s...", client.target_uid))

					for _, element := range clients {

						//log(element.id, fmt.Sprintf(fmt.Sprintf("%s", element.videoTrack)))

						if element.user_id != client.target_uid {
							continue
						}

						if element.videoTrack != nil {
							localTrack, _ := peerConnection.NewTrack(element.videoTrack.PayloadType(), element.videoTrack.SSRC(), "video", "pion")
							client.videoTrack <- localTrack
						}
						if element.audioTrack != nil {
							localTrack, _ := peerConnection.NewTrack(element.audioTrack.PayloadType(), element.audioTrack.SSRC(), "audio", "pion-")
							client.audioTrack <- localTrack
						}
					}
				} else {
					log(client.id, fmt.Sprintf("Track connected to %s", client.target_uid))
				}

				time.Sleep(time.Second)
			}
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
