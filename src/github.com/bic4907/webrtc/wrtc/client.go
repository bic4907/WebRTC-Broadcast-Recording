package wrtc

import (
	list "container/list"
	"encoding/json"
	"fmt"
	webrtcsignal "github.com/bic4907/webrtc/internal/signal"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"strings"
	"time"
)

var (
	viewers map[string]*Viewer = map[string]*Viewer{}
)

type Viewer struct {
	pc         *webrtc.PeerConnection
	dc         *webrtc.DataChannel
	dc2        *webrtc.DataChannel
	id         uuid.UUID
	last_hit   int64
	target_uid string
	candidates *list.List

	videoTrack *webrtc.Track
	audioTrack *webrtc.Track
	videoBuf	chan []byte
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
	client, exists := viewers[uid]

	if exists == false {
		panic("Not exists viewer session")
	}

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

			d.OnOpen(func() {
			})

			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				arr := strings.Split(string(msg.Data), "-")
				d.SendText("pong-" + arr[1])

				client.last_hit = makeTimestamp()
			})

		} else if d.Label() == "track" {

			client.dc2 = d

			d.OnOpen(func() {
			})




		}





	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log(client.id, fmt.Sprintf("Connection State has changed %s", connectionState.String()))

		if connectionState.String() == "disconnected" {
			delete(viewers, client.id.String())
			//client.recorder.Close()
		} else if connectionState.String() == "connected" {

			//go func() {
			//	for {
			//
			//		_, exists := viewers[client.id.String()]
			//		if exists == false {
			//			return
			//		}
			//
			//
			//		if (viewers[client.id.String()].videoTrack == nil) || (viewers[client.id.String()].audioTrack == nil) {
			//
			//			//log(client.id, fmt.Sprintf("Track connecting to %s...", client.target_uid))
			//
			//			log(client.id, fmt.Sprintf(fmt.Sprintf("Clients Remote * %s -> %x (%s, %s)", client.id, &clients)))
			//
			//			for _, element := range clients {
			//
			//				log(client.id, fmt.Sprintf(fmt.Sprintf("For %s -> %x (%s, %s)", element.id, &element, element.videoTrack == nil, element.audioTrack == nil)))
			//
			//				_, exists := viewers[client.id.String()]
			//				if exists == false {
			//					return
			//				}
			//
			//
			//				if element.user_id != client.target_uid {
			//					continue
			//				}
			//
			//				if element.videoTrack != nil {
			//					localTrack, _ := client.pc.NewTrack(element.videoTrack.PayloadType(), element.videoTrack.SSRC(), "video", "pion")
			//
			//					//client.pc.AddTrack(localTrack)
			//					viewers[client.id.String()].videoTrack = localTrack
			//					log(client.id, fmt.Sprintf(fmt.Sprintf("VideoTrack %s -> %s", element.id, element.videoTrack)))
			//
			//
			//				}
			//				if element.audioTrack != nil {
			//
			//					//localTrack, _ := client.pc.NewTrack(element.audioTrack.PayloadType(), element.audioTrack.SSRC(), "audio", "pion")
			//					//client.pc.AddTrack(localTrack)
			//					//viewers[client.id.String()].audioTrack = localTrack
			//
			//					//log(client.id, fmt.Sprintf(fmt.Sprintf("AudioTrack %s -> %s", element.id, element.audioTrack)))
			//				}
			//
			//				//client.dc2.SendText(webrtcsignal.Encode(client.pc.LocalDescription()))
			//
			//			}
			//		} else {
			//			//log(client.id, fmt.Sprintf("Track connected to %s", client.target_uid))
			//			//client.dc2.SendText(webrtcsignal.Encode(client.pc.LocalDescription()))
			//		}
			//
			//		time.Sleep(time.Second)
			//	}
			//}()


		}

	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	webrtcsignal.Decode(token, &offer)

	viewers[client.id.String()] = &client

	for {

		if (viewers[client.id.String()].videoTrack == nil) || (viewers[client.id.String()].audioTrack == nil) {

			log(client.id, fmt.Sprintf(fmt.Sprintf("Clients Remote * %s -> %x (%s, %s)", client.id, &clients)))

			for _, element := range clients {

				log(client.id, fmt.Sprintf(fmt.Sprintf("For %s -> %x (%s, %s)", element.id, &element, element.videoTrack == nil, element.audioTrack == nil)))

				_, exists := viewers[client.id.String()]
				if exists == false {
					continue
				}


				if element.user_id != client.target_uid {
					continue
				}

				if element.videoTrack != nil {
					localTrack, _ := client.pc.NewTrack(element.videoTrack.PayloadType(), element.videoTrack.SSRC(), "video", "pion")

					client.pc.AddTrack(localTrack)
					viewers[client.id.String()].videoTrack = localTrack
					log(client.id, fmt.Sprintf(fmt.Sprintf("VideoTrack %s -> %s", element.id, element.videoTrack)))


				}
				if element.audioTrack != nil {

					localTrack, _ := client.pc.NewTrack(element.audioTrack.PayloadType(), element.audioTrack.SSRC(), "audio", "pion")
					client.pc.AddTrack(localTrack)
					viewers[client.id.String()].audioTrack = localTrack

					log(client.id, fmt.Sprintf(fmt.Sprintf("AudioTrack %s -> %s", element.id, element.audioTrack)))
				}

				//client.dc2.SendText(webrtcsignal.Encode(client.pc.LocalDescription()))

			}
		} else {
			break
			//log(client.id, fmt.Sprintf("Track connected to %s", client.target_uid))
			//client.dc2.SendText(webrtcsignal.Encode(client.pc.LocalDescription()))
		}

		time.Sleep(time.Second)
	}




	//localTrack, _ := client.pc.NewTrack(element.videoTrack.PayloadType(), element.videoTrack.SSRC(), "video", "pion")

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
