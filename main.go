package main

import (
	web "./web"
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
