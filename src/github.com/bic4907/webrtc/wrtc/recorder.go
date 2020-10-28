package wrtc

import (
	"bufio"
	"fmt"
	"github.com/at-wat/ebml-go/webm"
	"os/exec"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
)

var (
	videoPath = "videos/"
)

type VideoRecorder struct {
	audioWriter, videoWriter       webm.BlockWriteCloser
	audioBuilder, videoBuilder     *samplebuilder.SampleBuilder
	audioTimestamp, videoTimestamp uint32
	client                         Client
	path                           string
	name                           string
}

func newVideoRecorder() *VideoRecorder {
	return &VideoRecorder{
		audioBuilder: samplebuilder.New(10, &codecs.OpusPacket{}),
		videoBuilder: samplebuilder.New(10, &codecs.VP8Packet{}),
	}
}

func (s *VideoRecorder) Close() {
	if s.client.recorder.name == "" {
		return
	}

	log(s.client.id, fmt.Sprintf("Recording finished - %s", s.client.recorder.name))
	if s.audioWriter != nil {
		if err := s.audioWriter.Close(); err != nil {
			// panic(err)
		}
	}
	s.audioWriter = nil
	if s.videoWriter != nil {
		if err := s.videoWriter.Close(); err != nil {
			// panic(err)
		}
	}
	s.videoWriter = nil
}
func (s *VideoRecorder) PushOpus(rtpPacket *rtp.Packet) {
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
func (s *VideoRecorder) PushVP8(rtpPacket *rtp.Packet) {
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
func (s *VideoRecorder) InitWriter(width, height int) {

	uid := s.client.id.String()
	now := time.Now().Format("2006-01-02_15:04:05")
	filename := uid + ".mp4"
	filepath := videoPath + now + "_" + filename
	s.path = filepath
	s.name = filename

	// - re : frame input
	// - -i (pipe:0) : input as pipeline 0
	ffmpeg := exec.Command("ffmpeg", "-re", "-i", "pipe:0", "-c:v", "libx264", "-loglevel", "panic", filepath) //nolint
	ffmpegIn, _ := ffmpeg.StdinPipe()
	ffmpegOut, _ := ffmpeg.StderrPipe()
	if err := ffmpeg.Start(); err != nil {
		panic(err)
	}

	go func() {
		scanner := bufio.NewScanner(ffmpegOut)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	ws, err := webm.NewSimpleBlockWriter(ffmpegIn,
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

	log(s.client.id, fmt.Sprintf("Record starting - video width=%d, height=%d", width, height))

	s.audioWriter = ws[0]
	s.videoWriter = ws[1]
}
