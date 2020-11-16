package web

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/bic4907/webrtc/common"
	"github.com/bic4907/webrtc/wrtc"
	"log"
)



type Hub struct {
	broadcasters map[string]*wrtc.Broadcaster
	Register     chan *wrtc.Broadcaster
	Unregister   chan *wrtc.Broadcaster

	subscribers map[string]*list.List
	Subscribe   chan *wrtc.Subscriber
	Unsubscribe chan *wrtc.Subscriber

	Broadcast 	chan common.BroadcastChunk
}



func NewHub() *Hub {
	return &Hub{
		broadcasters: map[string]*wrtc.Broadcaster{},
		Register:     make(chan *wrtc.Broadcaster),
		Unregister:   make(chan *wrtc.Broadcaster),

		subscribers: map[string]*list.List{},
		Subscribe:   make(chan *wrtc.Subscriber),
		Unsubscribe: make(chan *wrtc.Subscriber),

		Broadcast: 	make(chan common.BroadcastChunk),
	}
}

func (h *Hub) Run() {


	for {

		defer func() {
			recover()
		}()

		select {
		case broadcaster := <-h.Register:
			log.Print("Broadcast registered - " + broadcaster.BroadcastId)
			h.broadcasters[broadcaster.BroadcastId] = broadcaster

		case broadcaster := <-h.Unregister:
			if _, ok := h.broadcasters[broadcaster.BroadcastId]; ok {
				log.Print("Broadcast unregistered - " + broadcaster.BroadcastId)
				h.BroadcastBroadcasterExited(broadcaster)
				delete(h.broadcasters, broadcaster.BroadcastId)
			}

		case subscriber := <-h.Subscribe:
			l, exist := h.subscribers[subscriber.BroadcastId]

			if exist == false {
				h.subscribers[subscriber.BroadcastId] = list.New()
				l, _ = h.subscribers[subscriber.BroadcastId]
			}

			l.PushBack(subscriber)

			log.Print("Subscriber registered - " + subscriber.BroadcastId)
			fmt.Println(l.Len())

			// Already broadcasting
			broadcaster, exist := h.broadcasters[subscriber.BroadcastId]
			if exist {
				if broadcaster.AudioTrack != nil && broadcaster.VideoTrack != nil {
					AttachBroadcaster(subscriber, broadcaster)
				}
			}


		case subscriber := <-h.Unsubscribe:
			l, exist := h.subscribers[subscriber.BroadcastId]

			if exist == false {
				break
			}

			var next *list.Element
			for e := l.Front(); e != nil; e = next {
				next = e.Next()

				if e.Value == subscriber {
					l.Remove(e)
				}
			}
			log.Print("Subscriber unregistered - " + subscriber.BroadcastId)
			fmt.Println(l.Len())

			case chunk := <-h.Broadcast:

				// Broadcast if not broadcasted
				br, exist := h.broadcasters[chunk.BroadcastId]
				if exist {
					if br.VideoTrack != nil && br.AudioTrack != nil {
						if br.IsBroadcasted == false {
							h.BroadcastBroadcasterEntered(br)
							br.IsBroadcasted = true
						}
					}
				}

				l, exist := h.subscribers[chunk.BroadcastId]

				if exist == false {
					break
				}

				var next *list.Element
				for e := l.Front(); e != nil; e = next {
					next = e.Next()

					sub := e.Value.(*wrtc.Subscriber)
					sub.Receiver <- chunk
				}

		}

	}
}

func (h *Hub) GetBroadcaster(broadcastId string) *wrtc.Broadcaster {
	broadcaster, _ := h.broadcasters[broadcastId]
	return broadcaster
}


func AttachBroadcaster(subscriber *wrtc.Subscriber, broadcaster *wrtc.Broadcaster) {
	audioTrack, _ := subscriber.Pc.NewTrack(broadcaster.AudioTrack.PayloadType(), broadcaster.AudioTrack.SSRC(), "audio", "pion")
	subscriber.Pc.AddTrack(audioTrack)
	subscriber.AudioTrack = audioTrack

	videoTrack, _ := subscriber.Pc.NewTrack(broadcaster.VideoTrack.PayloadType(), broadcaster.VideoTrack.SSRC(), "video", "pion")
	subscriber.Pc.AddTrack(videoTrack)
	subscriber.VideoTrack = videoTrack


	offer, err := subscriber.Pc.CreateOffer(nil)
	subscriber.Pc.SetLocalDescription(offer)

	if err != nil {
		panic(err)
	}

	payload := make(map[string]interface{})
	payload["type"] = "remoteOffer"
	payload["message"] = offer
	message, _ := json.Marshal(payload)

	subscriber.Ws.WriteMessage(1, message)

}

func DeAttachBroadcaster(subscriber *wrtc.Subscriber, broadcaster *wrtc.Broadcaster) {


	senders := subscriber.Pc.GetSenders()
	for _, sender := range senders {
		subscriber.Pc.RemoveTrack(sender)
	}

	offer, err := subscriber.Pc.CreateOffer(nil)
	subscriber.Pc.SetLocalDescription(offer)

	if err != nil {
		panic(err)
	}

	payload := make(map[string]interface{})
	payload["type"] = "broadcasterExited"
	payload["message"] = offer
	message, _ := json.Marshal(payload)

	subscriber.Ws.WriteMessage(1, message)

}




func (h*Hub) BroadcastBroadcasterExited(broadcaster *wrtc.Broadcaster) {
	l, exist := h.subscribers[broadcaster.BroadcastId]

	if exist == false {
		return
	}

	var next *list.Element
	for e := l.Front(); e != nil; e = next {
		next = e.Next()

		sub := e.Value.(*wrtc.Subscriber)

		DeAttachBroadcaster(sub, broadcaster)
	}
}

func (h*Hub) BroadcastBroadcasterEntered(broadcaster *wrtc.Broadcaster) {
	l, exist := h.subscribers[broadcaster.BroadcastId]

	if exist == false {
		return
	}

	var next *list.Element
	for e := l.Front(); e != nil; e = next {
		next = e.Next()

		sub := e.Value.(*wrtc.Subscriber)

		AttachBroadcaster(sub, broadcaster)
	}
}