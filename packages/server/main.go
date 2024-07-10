package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	webRTC, err := InitWebRTC()
	if err != nil {
		panic(err)
	}

	peers := Peers{
		peers: map[string]Peer{},
		mutex: sync.Mutex{},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(webRTC, &peers, w, r)
	})
	go func() {
		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Println(err)
		}
	}()

	go func() {
		for {
			time.Sleep(3 * time.Second)
			peers.RequestKeyFrames()
		}
	}()

	select {}
}
