package wrtc

import (
	"fmt"
	"github.com/pion/webrtc/v3"
	"time"

	"github.com/google/uuid"
)

func log(id uuid.UUID, str string) {
	uid, _ := id.Value()
	t := time.Now().Format("2006-01-02 15:04:05")

	fmt.Println(fmt.Sprintf("[%s] %s %s", uid, t, str))
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

var WebRTCConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
}