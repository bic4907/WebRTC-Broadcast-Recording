package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/at-wat/ebml-go/webm"

	webrtcsignal "github.com/pion/example-webrtc-applications/internal/signal"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media/samplebuilder"
)

func main() {
	saver := newWebmSaver()
	peerConnection := createWebRTCConn(saver)

	closed := make(chan os.Signal, 1)
	signal.Notify(closed, os.Interrupt)
	<-closed

	if err := peerConnection.Close(); err != nil {
		panic(err)
	}
	saver.Close()
}

type webmSaver struct {
	audioWriter, videoWriter       webm.BlockWriteCloser
	audioBuilder, videoBuilder     *samplebuilder.SampleBuilder
	audioTimestamp, videoTimestamp uint32
}

func newWebmSaver() *webmSaver {
	return &webmSaver{
		audioBuilder: samplebuilder.New(10, &codecs.OpusPacket{}),
		videoBuilder: samplebuilder.New(10, &codecs.VP8Packet{}),
	}
}

func (s *webmSaver) Close() {
	fmt.Printf("Finalizing webm...\n")
	if s.audioWriter != nil {
		if err := s.audioWriter.Close(); err != nil {
			panic(err)
		}
	}
	if s.videoWriter != nil {
		if err := s.videoWriter.Close(); err != nil {
			panic(err)
		}
	}
}
func (s *webmSaver) PushOpus(rtpPacket *rtp.Packet) {
	s.audioBuilder.Push(rtpPacket)

	for {
		sample := s.audioBuilder.Pop()
		if sample == nil {
			return
		}
		if s.audioWriter != nil {
			s.audioTimestamp += sample.Samples
			t := s.audioTimestamp / 48
			if _, err := s.audioWriter.Write(true, int64(t), sample.Data); err != nil {
				panic(err)
			}
		}
	}
}
func (s *webmSaver) PushVP8(rtpPacket *rtp.Packet) {
	s.videoBuilder.Push(rtpPacket)

	for {
		sample := s.videoBuilder.Pop()
		if sample == nil {
			return
		}
		// Read VP8 header.
		videoKeyframe := (sample.Data[0]&0x1 == 0)
		if videoKeyframe {
			// Keyframe has frame information.
			raw := uint(sample.Data[6]) | uint(sample.Data[7])<<8 | uint(sample.Data[8])<<16 | uint(sample.Data[9])<<24
			width := int(raw & 0x3FFF)
			height := int((raw >> 16) & 0x3FFF)

			if s.videoWriter == nil || s.audioWriter == nil {
				// Initialize WebM saver using received frame size.
				s.InitWriter(width, height)
			}
		}
		if s.videoWriter != nil {
			s.videoTimestamp += sample.Samples
			t := s.videoTimestamp / 90
			if _, err := s.videoWriter.Write(videoKeyframe, int64(t), sample.Data); err != nil {
				panic(err)
			}
		}
	}
}
func (s *webmSaver) InitWriter(width, height int) {
	w, err := os.OpenFile("test.webm", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}

	ws, err := webm.NewSimpleBlockWriter(w,
		[]webm.TrackEntry{
			{
				Name:            "Audio",
				TrackNumber:     1,
				TrackUID:        12345,
				CodecID:         "A_OPUS",
				TrackType:       2,
				DefaultDuration: 20000000,
				Audio: &webm.Audio{
					SamplingFrequency: 48000.0,
					Channels:          2,
				},
			}, {
				Name:            "Video",
				TrackNumber:     2,
				TrackUID:        67890,
				CodecID:         "V_VP8",
				TrackType:       1,
				DefaultDuration: 33333333,
				Video: &webm.Video{
					PixelWidth:  uint64(width),
					PixelHeight: uint64(height),
				},
			},
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("WebM saver has started with video width=%d, height=%d\n", width, height)
	s.audioWriter = ws[0]
	s.videoWriter = ws[1]
}

func createWebRTCConn(saver *webmSaver) *webrtc.PeerConnection {
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

	if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().Name)
		for {
			// Read RTP packets being sent to Pion
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				if readErr == io.EOF {
					return
				}
				panic(readErr)
			}
			switch track.Kind() {
			case webrtc.RTPCodecTypeAudio:
				saver.PushOpus(rtp)
			case webrtc.RTPCodecTypeVideo:
				saver.PushVP8(rtp)
			}
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	webrtcsignal.Decode("eyJ0eXBlIjoib2ZmZXIiLCJzZHAiOiJ2PTBcclxubz0tIDQ5NTc4MjMwODY3NjIwOTg1NzYgMiBJTiBJUDQgMTI3LjAuMC4xXHJcbnM9LVxyXG50PTAgMFxyXG5hPWdyb3VwOkJVTkRMRSAwIDFcclxuYT1tc2lkLXNlbWFudGljOiBXTVMgRWpaR3pwNlNta0w4M01PZzZIYUFIRW1oYnMwd2FFalp5c1JzXHJcbm09YXVkaW8gNjAzNzggVURQL1RMUy9SVFAvU0FWUEYgMTExIDEwMyAxMDQgOSAwIDggMTA2IDEwNSAxMyAxMTAgMTEyIDExMyAxMjZcclxuYz1JTiBJUDQgNTguMjI2LjE2Ny4xMzVcclxuYT1ydGNwOjkgSU4gSVA0IDAuMC4wLjBcclxuYT1jYW5kaWRhdGU6NDAyMjg2NjQ0NiAxIHVkcCAyMTIyMjYwMjIzIDE5Mi4xNjguMC4xOTcgNjAzNzggdHlwIGhvc3QgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMSBuZXR3b3JrLWNvc3QgMTBcclxuYT1jYW5kaWRhdGU6MTg1Mzg4NzY3NCAxIHVkcCAxNjg2MDUyNjA3IDU4LjIyNi4xNjcuMTM1IDYwMzc4IHR5cCBzcmZseCByYWRkciAxOTIuMTY4LjAuMTk3IHJwb3J0IDYwMzc4IGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDEgbmV0d29yay1jb3N0IDEwXHJcbmE9Y2FuZGlkYXRlOjI3MDYxMDgxNTggMSB0Y3AgMTUxODI4MDQ0NyAxOTIuMTY4LjAuMTk3IDkgdHlwIGhvc3QgdGNwdHlwZSBhY3RpdmUgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMSBuZXR3b3JrLWNvc3QgMTBcclxuYT1pY2UtdWZyYWc6dU9PQlxyXG5hPWljZS1wd2Q6cWJSdFgxQnI0cHJXVmpDOStYSC9CTnR6XHJcbmE9aWNlLW9wdGlvbnM6dHJpY2tsZVxyXG5hPWZpbmdlcnByaW50OnNoYS0yNTYgNzA6MDI6Q0Y6QzU6QTk6Rjk6RkQ6Qzc6MTE6MTg6MkU6NTM6MTM6RDE6N0Y6QzA6MEU6NzE6MTI6RDM6OTg6Q0Q6Mjg6RkI6RTA6MjM6OEU6RDQ6NTU6Qjc6Rjg6NjBcclxuYT1zZXR1cDphY3RwYXNzXHJcbmE9bWlkOjBcclxuYT1leHRtYXA6MSB1cm46aWV0ZjpwYXJhbXM6cnRwLWhkcmV4dDpzc3JjLWF1ZGlvLWxldmVsXHJcbmE9ZXh0bWFwOjIgaHR0cDovL3d3dy53ZWJydGMub3JnL2V4cGVyaW1lbnRzL3J0cC1oZHJleHQvYWJzLXNlbmQtdGltZVxyXG5hPWV4dG1hcDozIGh0dHA6Ly93d3cuaWV0Zi5vcmcvaWQvZHJhZnQtaG9sbWVyLXJtY2F0LXRyYW5zcG9ydC13aWRlLWNjLWV4dGVuc2lvbnMtMDFcclxuYT1leHRtYXA6NCB1cm46aWV0ZjpwYXJhbXM6cnRwLWhkcmV4dDpzZGVzOm1pZFxyXG5hPWV4dG1hcDo1IHVybjppZXRmOnBhcmFtczpydHAtaGRyZXh0OnNkZXM6cnRwLXN0cmVhbS1pZFxyXG5hPWV4dG1hcDo2IHVybjppZXRmOnBhcmFtczpydHAtaGRyZXh0OnNkZXM6cmVwYWlyZWQtcnRwLXN0cmVhbS1pZFxyXG5hPXNlbmRyZWN2XHJcbmE9bXNpZDpFalpHenA2U21rTDgzTU9nNkhhQUhFbWhiczB3YUVqWnlzUnMgNzc4NTIwYmEtNTlmYy00ZjFkLWIwM2EtYTNmNjhjNWZiN2Y2XHJcbmE9cnRjcC1tdXhcclxuYT1ydHBtYXA6MTExIG9wdXMvNDgwMDAvMlxyXG5hPXJ0Y3AtZmI6MTExIHRyYW5zcG9ydC1jY1xyXG5hPWZtdHA6MTExIG1pbnB0aW1lPTEwO3VzZWluYmFuZGZlYz0xXHJcbmE9cnRwbWFwOjEwMyBJU0FDLzE2MDAwXHJcbmE9cnRwbWFwOjEwNCBJU0FDLzMyMDAwXHJcbmE9cnRwbWFwOjkgRzcyMi84MDAwXHJcbmE9cnRwbWFwOjAgUENNVS84MDAwXHJcbmE9cnRwbWFwOjggUENNQS84MDAwXHJcbmE9cnRwbWFwOjEwNiBDTi8zMjAwMFxyXG5hPXJ0cG1hcDoxMDUgQ04vMTYwMDBcclxuYT1ydHBtYXA6MTMgQ04vODAwMFxyXG5hPXJ0cG1hcDoxMTAgdGVsZXBob25lLWV2ZW50LzQ4MDAwXHJcbmE9cnRwbWFwOjExMiB0ZWxlcGhvbmUtZXZlbnQvMzIwMDBcclxuYT1ydHBtYXA6MTEzIHRlbGVwaG9uZS1ldmVudC8xNjAwMFxyXG5hPXJ0cG1hcDoxMjYgdGVsZXBob25lLWV2ZW50LzgwMDBcclxuYT1zc3JjOjQxMTM4MDM2NTAgY25hbWU6cmVJRDVjbmE2Tkk5aDN6ZVxyXG5hPXNzcmM6NDExMzgwMzY1MCBtc2lkOkVqWkd6cDZTbWtMODNNT2c2SGFBSEVtaGJzMHdhRWpaeXNScyA3Nzg1MjBiYS01OWZjLTRmMWQtYjAzYS1hM2Y2OGM1ZmI3ZjZcclxuYT1zc3JjOjQxMTM4MDM2NTAgbXNsYWJlbDpFalpHenA2U21rTDgzTU9nNkhhQUhFbWhiczB3YUVqWnlzUnNcclxuYT1zc3JjOjQxMTM4MDM2NTAgbGFiZWw6Nzc4NTIwYmEtNTlmYy00ZjFkLWIwM2EtYTNmNjhjNWZiN2Y2XHJcbm09dmlkZW8gNTIxMjEgVURQL1RMUy9SVFAvU0FWUEYgOTYgOTcgOTggOTkgMTAwIDEwMSAxMDIgMTIxIDEyNyAxMjAgMTI1IDEwNyAxMDggMTA5IDEyNCAxMTkgMTIzXHJcbmM9SU4gSVA0IDU4LjIyNi4xNjcuMTM1XHJcbmE9cnRjcDo5IElOIElQNCAwLjAuMC4wXHJcbmE9Y2FuZGlkYXRlOjQwMjI4NjY0NDYgMSB1ZHAgMjEyMjI2MDIyMyAxOTIuMTY4LjAuMTk3IDUyMTIxIHR5cCBob3N0IGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDEgbmV0d29yay1jb3N0IDEwXHJcbmE9Y2FuZGlkYXRlOjE4NTM4ODc2NzQgMSB1ZHAgMTY4NjA1MjYwNyA1OC4yMjYuMTY3LjEzNSA1MjEyMSB0eXAgc3JmbHggcmFkZHIgMTkyLjE2OC4wLjE5NyBycG9ydCA1MjEyMSBnZW5lcmF0aW9uIDAgbmV0d29yay1pZCAxIG5ldHdvcmstY29zdCAxMFxyXG5hPWNhbmRpZGF0ZToyNzA2MTA4MTU4IDEgdGNwIDE1MTgyODA0NDcgMTkyLjE2OC4wLjE5NyA5IHR5cCBob3N0IHRjcHR5cGUgYWN0aXZlIGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDEgbmV0d29yay1jb3N0IDEwXHJcbmE9aWNlLXVmcmFnOnVPT0JcclxuYT1pY2UtcHdkOnFiUnRYMUJyNHByV1ZqQzkrWEgvQk50elxyXG5hPWljZS1vcHRpb25zOnRyaWNrbGVcclxuYT1maW5nZXJwcmludDpzaGEtMjU2IDcwOjAyOkNGOkM1OkE5OkY5OkZEOkM3OjExOjE4OjJFOjUzOjEzOkQxOjdGOkMwOjBFOjcxOjEyOkQzOjk4OkNEOjI4OkZCOkUwOjIzOjhFOkQ0OjU1OkI3OkY4OjYwXHJcbmE9c2V0dXA6YWN0cGFzc1xyXG5hPW1pZDoxXHJcbmE9ZXh0bWFwOjE0IHVybjppZXRmOnBhcmFtczpydHAtaGRyZXh0OnRvZmZzZXRcclxuYT1leHRtYXA6MiBodHRwOi8vd3d3LndlYnJ0Yy5vcmcvZXhwZXJpbWVudHMvcnRwLWhkcmV4dC9hYnMtc2VuZC10aW1lXHJcbmE9ZXh0bWFwOjEzIHVybjozZ3BwOnZpZGVvLW9yaWVudGF0aW9uXHJcbmE9ZXh0bWFwOjMgaHR0cDovL3d3dy5pZXRmLm9yZy9pZC9kcmFmdC1ob2xtZXItcm1jYXQtdHJhbnNwb3J0LXdpZGUtY2MtZXh0ZW5zaW9ucy0wMVxyXG5hPWV4dG1hcDoxMiBodHRwOi8vd3d3LndlYnJ0Yy5vcmcvZXhwZXJpbWVudHMvcnRwLWhkcmV4dC9wbGF5b3V0LWRlbGF5XHJcbmE9ZXh0bWFwOjExIGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L3ZpZGVvLWNvbnRlbnQtdHlwZVxyXG5hPWV4dG1hcDo3IGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L3ZpZGVvLXRpbWluZ1xyXG5hPWV4dG1hcDo4IGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L2NvbG9yLXNwYWNlXHJcbmE9ZXh0bWFwOjQgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6c2RlczptaWRcclxuYT1leHRtYXA6NSB1cm46aWV0ZjpwYXJhbXM6cnRwLWhkcmV4dDpzZGVzOnJ0cC1zdHJlYW0taWRcclxuYT1leHRtYXA6NiB1cm46aWV0ZjpwYXJhbXM6cnRwLWhkcmV4dDpzZGVzOnJlcGFpcmVkLXJ0cC1zdHJlYW0taWRcclxuYT1zZW5kcmVjdlxyXG5hPW1zaWQ6RWpaR3pwNlNta0w4M01PZzZIYUFIRW1oYnMwd2FFalp5c1JzIGM0YzRhNTAwLTBkYjgtNDBmZC1hZGI2LTQ3MmM5ZjBlNWQwMlxyXG5hPXJ0Y3AtbXV4XHJcbmE9cnRjcC1yc2l6ZVxyXG5hPXJ0cG1hcDo5NiBWUDgvOTAwMDBcclxuYT1ydGNwLWZiOjk2IGdvb2ctcmVtYlxyXG5hPXJ0Y3AtZmI6OTYgdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjo5NiBjY20gZmlyXHJcbmE9cnRjcC1mYjo5NiBuYWNrXHJcbmE9cnRjcC1mYjo5NiBuYWNrIHBsaVxyXG5hPXJ0cG1hcDo5NyBydHgvOTAwMDBcclxuYT1mbXRwOjk3IGFwdD05NlxyXG5hPXJ0cG1hcDo5OCBWUDkvOTAwMDBcclxuYT1ydGNwLWZiOjk4IGdvb2ctcmVtYlxyXG5hPXJ0Y3AtZmI6OTggdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjo5OCBjY20gZmlyXHJcbmE9cnRjcC1mYjo5OCBuYWNrXHJcbmE9cnRjcC1mYjo5OCBuYWNrIHBsaVxyXG5hPWZtdHA6OTggcHJvZmlsZS1pZD0wXHJcbmE9cnRwbWFwOjk5IHJ0eC85MDAwMFxyXG5hPWZtdHA6OTkgYXB0PTk4XHJcbmE9cnRwbWFwOjEwMCBWUDkvOTAwMDBcclxuYT1ydGNwLWZiOjEwMCBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjEwMCB0cmFuc3BvcnQtY2NcclxuYT1ydGNwLWZiOjEwMCBjY20gZmlyXHJcbmE9cnRjcC1mYjoxMDAgbmFja1xyXG5hPXJ0Y3AtZmI6MTAwIG5hY2sgcGxpXHJcbmE9Zm10cDoxMDAgcHJvZmlsZS1pZD0yXHJcbmE9cnRwbWFwOjEwMSBydHgvOTAwMDBcclxuYT1mbXRwOjEwMSBhcHQ9MTAwXHJcbmE9cnRwbWFwOjEwMiBIMjY0LzkwMDAwXHJcbmE9cnRjcC1mYjoxMDIgZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjoxMDIgdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjoxMDIgY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MTAyIG5hY2tcclxuYT1ydGNwLWZiOjEwMiBuYWNrIHBsaVxyXG5hPWZtdHA6MTAyIGxldmVsLWFzeW1tZXRyeS1hbGxvd2VkPTE7cGFja2V0aXphdGlvbi1tb2RlPTE7cHJvZmlsZS1sZXZlbC1pZD00MjAwMWZcclxuYT1ydHBtYXA6MTIxIHJ0eC85MDAwMFxyXG5hPWZtdHA6MTIxIGFwdD0xMDJcclxuYT1ydHBtYXA6MTI3IEgyNjQvOTAwMDBcclxuYT1ydGNwLWZiOjEyNyBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjEyNyB0cmFuc3BvcnQtY2NcclxuYT1ydGNwLWZiOjEyNyBjY20gZmlyXHJcbmE9cnRjcC1mYjoxMjcgbmFja1xyXG5hPXJ0Y3AtZmI6MTI3IG5hY2sgcGxpXHJcbmE9Zm10cDoxMjcgbGV2ZWwtYXN5bW1ldHJ5LWFsbG93ZWQ9MTtwYWNrZXRpemF0aW9uLW1vZGU9MDtwcm9maWxlLWxldmVsLWlkPTQyMDAxZlxyXG5hPXJ0cG1hcDoxMjAgcnR4LzkwMDAwXHJcbmE9Zm10cDoxMjAgYXB0PTEyN1xyXG5hPXJ0cG1hcDoxMjUgSDI2NC85MDAwMFxyXG5hPXJ0Y3AtZmI6MTI1IGdvb2ctcmVtYlxyXG5hPXJ0Y3AtZmI6MTI1IHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6MTI1IGNjbSBmaXJcclxuYT1ydGNwLWZiOjEyNSBuYWNrXHJcbmE9cnRjcC1mYjoxMjUgbmFjayBwbGlcclxuYT1mbXRwOjEyNSBsZXZlbC1hc3ltbWV0cnktYWxsb3dlZD0xO3BhY2tldGl6YXRpb24tbW9kZT0xO3Byb2ZpbGUtbGV2ZWwtaWQ9NDJlMDFmXHJcbmE9cnRwbWFwOjEwNyBydHgvOTAwMDBcclxuYT1mbXRwOjEwNyBhcHQ9MTI1XHJcbmE9cnRwbWFwOjEwOCBIMjY0LzkwMDAwXHJcbmE9cnRjcC1mYjoxMDggZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjoxMDggdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjoxMDggY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MTA4IG5hY2tcclxuYT1ydGNwLWZiOjEwOCBuYWNrIHBsaVxyXG5hPWZtdHA6MTA4IGxldmVsLWFzeW1tZXRyeS1hbGxvd2VkPTE7cGFja2V0aXphdGlvbi1tb2RlPTA7cHJvZmlsZS1sZXZlbC1pZD00MmUwMWZcclxuYT1ydHBtYXA6MTA5IHJ0eC85MDAwMFxyXG5hPWZtdHA6MTA5IGFwdD0xMDhcclxuYT1ydHBtYXA6MTI0IHJlZC85MDAwMFxyXG5hPXJ0cG1hcDoxMTkgcnR4LzkwMDAwXHJcbmE9Zm10cDoxMTkgYXB0PTEyNFxyXG5hPXJ0cG1hcDoxMjMgdWxwZmVjLzkwMDAwXHJcbmE9c3NyYy1ncm91cDpGSUQgNjA2OTEyMjMxIDMzNTg5MjEwOTlcclxuYT1zc3JjOjYwNjkxMjIzMSBjbmFtZTpyZUlENWNuYTZOSTloM3plXHJcbmE9c3NyYzo2MDY5MTIyMzEgbXNpZDpFalpHenA2U21rTDgzTU9nNkhhQUhFbWhiczB3YUVqWnlzUnMgYzRjNGE1MDAtMGRiOC00MGZkLWFkYjYtNDcyYzlmMGU1ZDAyXHJcbmE9c3NyYzo2MDY5MTIyMzEgbXNsYWJlbDpFalpHenA2U21rTDgzTU9nNkhhQUhFbWhiczB3YUVqWnlzUnNcclxuYT1zc3JjOjYwNjkxMjIzMSBsYWJlbDpjNGM0YTUwMC0wZGI4LTQwZmQtYWRiNi00NzJjOWYwZTVkMDJcclxuYT1zc3JjOjMzNTg5MjEwOTkgY25hbWU6cmVJRDVjbmE2Tkk5aDN6ZVxyXG5hPXNzcmM6MzM1ODkyMTA5OSBtc2lkOkVqWkd6cDZTbWtMODNNT2c2SGFBSEVtaGJzMHdhRWpaeXNScyBjNGM0YTUwMC0wZGI4LTQwZmQtYWRiNi00NzJjOWYwZTVkMDJcclxuYT1zc3JjOjMzNTg5MjEwOTkgbXNsYWJlbDpFalpHenA2U21rTDgzTU9nNkhhQUhFbWhiczB3YUVqWnlzUnNcclxuYT1zc3JjOjMzNTg5MjEwOTkgbGFiZWw6YzRjNGE1MDAtMGRiOC00MGZkLWFkYjYtNDcyYzlmMGU1ZDAyXHJcbiJ9==", &offer)

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

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(webrtcsignal.Encode(answer))

	return peerConnection
}
