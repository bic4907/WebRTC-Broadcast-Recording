package web

import (
	"container/list"
	"github.com/bic4907/webrtc/wrtc"
)

type BroadcastChunk struct {
	userId string
	chunk  []byte
}

type Hub struct {
	// Registered clients.
	broadcasters map[string]wrtc.Broadcaster
	subscribers map[string]list.List

	broadcast chan BroadcastChunk

	//register chan *Client
	//unregister chan *Client
}

func newHub() *Hub {
	return &Hub {

		//broadcast:  make(chan []byte),
		//register:   make(chan *Client),
		//unregister: make(chan *Client),
		//clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		//select {
		//case client := <-h.register:
		//	h.clients[client] = true
		//case client := <-h.unregister:
		//	if _, ok := h.clients[client]; ok {
		//		delete(h.clients, client)
		//		close(client.send)
		//	}
		//case message := <-h.broadcast:
		//	for client := range h.clients {
		//		select {
		//		case client.send <- message:
		//		default:
		//			close(client.send)
		//			delete(h.clients, client)
		//		}
		//	}
		//}
	}
}