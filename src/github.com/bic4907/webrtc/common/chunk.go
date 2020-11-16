package common

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type BroadcastChunk struct {
	BroadcastId 	string
	CodecType       webrtc.RTPCodecType
	Chunk  			*rtp.Packet
}
