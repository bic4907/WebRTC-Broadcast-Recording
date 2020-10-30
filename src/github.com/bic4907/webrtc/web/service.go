package web

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"

	"github.com/bic4907/webrtc/wrtc"
)

func StartWebService() {
	var address = ":10001"

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/connect", connectHandler)
	http.HandleFunc("/add-candidate", addCandidateHandler)
	http.HandleFunc("/get-candidate", getCandidateHandler)
	http.HandleFunc("/client.js", scriptHandler)

	var err error
	// Check SSL Certification
	if fileExists("certs/cert.pem") && fileExists("certs/privkey.pem") {
		fmt.Println("Server opened as HTTPS @", address)
		err = http.ListenAndServeTLS(address, "certs/cert.pem", "certs/privkey.pem", nil)
	} else {
		fmt.Println("Server opened as HTTP @", address)
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
	data, err := ioutil.ReadFile("index.html")

	if err != nil {
		fmt.Println(err)
	}
	w.Write(data)
}

func scriptHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("script.js")

	if err != nil {
		fmt.Println(err)
	}
	w.Write(data)
}

func addCandidateHandler(w http.ResponseWriter, r *http.Request) {

	clientId, resp := wrtc.AddCandidateToPeerConnnection(r.FormValue("uid"), r.FormValue("candidate"))
	w.Write([]byte(clientId + "\t" + resp))
}

func getCandidateHandler(w http.ResponseWriter, r *http.Request) {

	clientId, resp, output := wrtc.GetCandidateToPeerConnnection(r.FormValue("uid"))
	w.Write([]byte(clientId + "\t" + resp + "\t" + output))
}

func connectHandler(w http.ResponseWriter, r *http.Request) {
	clientId, resp := wrtc.CreatePeerConnection(r.FormValue("localDescription"))
	w.Write([]byte(clientId + "\t" + resp))
}
