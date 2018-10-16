package main

import (
	"log"
	"net/http"
	"os"

	messagebird "github.com/messagebird/go-rest-api"
)

func main() {
	dbdata := new(RideSharingDB)
	initExampleDB()

	mb := messagebird.New(os.Getenv("MESSAGEBIRD_API_KEY"))

	mux := http.NewServeMux()
	mux.Handle("/", landing(dbdata))
	mux.Handle("/createride", createRideHandler(dbdata, mb))
	mux.Handle("/webhook", messageHookHandler(dbdata, mb))
	mux.Handle("/webhook-voice", voiceHookHandler(dbdata, mb))

	port := ":8080"
	log.Println("Serving on", port)
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
