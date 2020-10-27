package main

import (
	"github.com/bic4907/webrtc/web"
	"os"
	"os/signal"
)

func main() {
	web.StartWebService()

	closed := make(chan os.Signal, 1)
	signal.Notify(closed, os.Interrupt)
	<-closed

	// if err := peerConnection.Close(); err != nil {
	//	panic(err)
	//}
	// saver.Close()
}
