package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bic4907/webrtc/common"
	"github.com/bic4907/webrtc/wrtc"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
)

var upgrader = websocket.Upgrader{} // use default options

func StartWebService() {
	var address = ":10001"


	http.HandleFunc("/", indexHandler)

	pwd, _ := os.Getwd()
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir(pwd + "/web/src/static"))))


	hub := NewHub()
	go hub.Run()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocketHandler(hub, w, r)
	})



	var err error
	if fileExists("certs/cert.pem") && fileExists("certs/privkey.pem") {
		fmt.Println(fmt.Sprintf("Server opened as HTTPS (%s, %s)", "https://127.0.0.1" + address, "https://0.0.0.0" + address))
		err = http.ListenAndServeTLS(address, "certs/cert.pem", "certs/privkey.pem", nil)
	} else {
		fmt.Println(fmt.Sprintf("Server opened as HTTP (%s, %s)", "http://127.0.0.1" + address, "http://0.0.0.0" + address))
		err = http.ListenAndServe(address, nil)
	}

	closed := make(chan os.Signal, 1)
	signal.Notify(closed, os.Interrupt)
	<-closed

	if err != nil {
		fmt.Println(err)
	}

}

func indexHandler(w http.ResponseWriter, r *http.Request) {

	pwd, _ := os.Getwd()
	data, err := ioutil.ReadFile(pwd + "/web/src/index.html")

	if err != nil {
		fmt.Println(err)
	}
	w.Write(data)
}

func websocketHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	var pc *webrtc.PeerConnection = nil
	var br *wrtc.Broadcaster = nil
	var sub *wrtc.Subscriber = nil


	defer func() {
		c.Close()
		if br != nil {
			hub.Unregister <- br
			br.Ws = nil
			br = nil
		}
		if sub != nil {
			hub.Unsubscribe <- sub
			sub.Ws = nil
			sub = nil
		}
		return
	}()

	go func() {
		defer func() {
			recover()

			if br != nil {
				hub.Unregister <- br
				br.Ws = nil
				br = nil
			}
			if sub != nil {
				hub.Unsubscribe <- sub
				sub.Ws = nil
				sub = nil
			}
			c.Close()


		}()
		for {
			if br != nil {
				message := <-br.MessageChannel

				err = c.WriteMessage(1, message)
				if err != nil {
					break
				}
			}
			if sub != nil {

				message := <-sub.MessageChannel

				err = c.WriteMessage(1, message)
				if err != nil {
					break
				}
			}

		}
	}()

	for {

		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}

		var data map[string]string
		err = json.Unmarshal(message, &data)
		if err != nil {
			fmt.Println(err)
		}

		switch data["type"] {
		case "broadcastRequest":


			b, _ := base64.StdEncoding.DecodeString(data["message"])

			offer := webrtc.SessionDescription{}
			err = json.Unmarshal(b, &offer)
			if err != nil {
				panic(err)
			}


			uuid4, _ := uuid.NewRandom()
			broadcaster := wrtc.Broadcaster{
				Ws:          c,
				Uid:		 uuid4,
				UserId:      data["userId"],
				RoomId:      data["roomId"],
				BroadcastId: data["roomId"] + "_" + data["userId"],
				BroadcastChannel: make(chan common.BroadcastChunk),
				IsBroadcasted: false,
				MessageChannel: make(chan []byte),
			}

			go func() {
				for {
					chunk := <- broadcaster.BroadcastChannel
					hub.Broadcast <- chunk
				}
			}()

			prevBroadcaster := hub.GetBroadcaster(broadcaster.BroadcastId)
			if prevBroadcaster != nil {

				prevBroadcaster.Ws.Close()
				prevBroadcaster.Pc.Close()

				payload := make(map[string]interface{})
				payload["type"] = "duplicatedSession"
				message, _ = json.Marshal(payload)

				err = c.WriteMessage(mt, message)

				hub.Unregister <- prevBroadcaster
			} else {

				pc = wrtc.MakeBroadcasterPeerConnection(offer, &broadcaster)

				payload := make(map[string]interface{})
				payload["type"] = "remoteDescription"
				payload["message"] = pc.LocalDescription()
				message, _ = json.Marshal(payload)

				err = c.WriteMessage(mt, message)

				br = &broadcaster
				hub.Register <- &broadcaster

			}
		case "subscribeRequest":

			b, _ := base64.StdEncoding.DecodeString(data["message"])

			offer := webrtc.SessionDescription{}
			err = json.Unmarshal(b, &offer)
			if err != nil {
				panic(err)
			}

			uuid4, _ := uuid.NewRandom()
			subscriber := wrtc.Subscriber{
				Ws:          c,
				Uid:		 uuid4,
				UserId:      data["userId"],
				RoomId:      data["roomId"],
				BroadcastId: data["roomId"] + "_" + data["userId"],
				Receiver: make(chan common.BroadcastChunk),
				MessageChannel: make(chan []byte),
			}

			pc = wrtc.MakeSubscriberPeerConnection(offer, &subscriber)


			payload := make(map[string]interface{})
			payload["type"] = "remoteDescription"
			payload["message"] = pc.LocalDescription()
			message, _ = json.Marshal(payload)

			c.WriteMessage(mt, message)


			sub = &subscriber
			hub.Subscribe <- &subscriber

		case "iceCandidate":

			var actual webrtc.ICECandidateInit
			json.Unmarshal([]byte(data["message"]), &actual)
			if pc != nil {
				pc.AddICECandidate(actual)
			}
			break
		case "remoteAnswer":
			b, _ := base64.StdEncoding.DecodeString(data["message"])

			answer := webrtc.SessionDescription{}
			err = json.Unmarshal(b, &answer)
			pc.SetRemoteDescription(answer)

			break
		}

	}

}




