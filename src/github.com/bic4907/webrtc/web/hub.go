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

		select {
		case broadcaster := <-h.Register:
			log.Print("Broadcast registered - " + broadcaster.BroadcastId)
			h.broadcasters[broadcaster.BroadcastId] = broadcaster

		case broadcaster := <-h.Unregister:
			if _, ok := h.broadcasters[broadcaster.BroadcastId]; ok {
				log.Print("Broadcast unregistered - " + broadcaster.BroadcastId)
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

				if broadcaster.AudioTrack != nil {
					track, _ := subscriber.Pc.NewTrack(broadcaster.AudioTrack.PayloadType(), broadcaster.AudioTrack.SSRC(), "audio", "pion")
					subscriber.Pc.AddTrack(track)
					subscriber.AudioTrack = track
				}
				if broadcaster.VideoTrack != nil {
					track, _ := subscriber.Pc.NewTrack(broadcaster.VideoTrack.PayloadType(), broadcaster.VideoTrack.SSRC(), "video", "pion")
					subscriber.Pc.AddTrack(track)
					subscriber.VideoTrack = track


					offer, err := subscriber.Pc.CreateOffer(nil)
					subscriber.Pc.SetLocalDescription(offer)

					if err != nil {
						panic(err)
					}

					payload := make(map[string]interface{})
					payload["type"] = "remoteOffer"
					payload["message"] = offer
					message, _ := json.Marshal(payload)
					err = subscriber.Ws.WriteMessage(1, message)
					if err != nil {

					}
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