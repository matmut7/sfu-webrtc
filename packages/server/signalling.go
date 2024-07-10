package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

const (
	IceCandidate = iota
	Offer
	Answer
	NegotiationNeeded
)

type SignallingMessage struct {
	Data string `json:"data"`
	Type int    `json:"type"`
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWebSocket(webrtcContext *webrtcContext, peers *Peers, w http.ResponseWriter, r *http.Request) {
	unsafeConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	conn := &threadSafeWriter{unsafeConn, sync.Mutex{}}
	defer conn.Close()

	peer, err := peers.InitPeerConnection(webrtcContext, conn)
	if err != nil {
		log.Println("error creating new peer", err)
		return
	}
	defer peers.RemovePeer(peer.id)

	message := SignallingMessage{}
	for {

		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Println("error reading from WS", err)
			return
		}

		if err := json.Unmarshal(raw, &message); err != nil {
			log.Println("error unmarshaling message", err)
			return
		}

		switch message.Type {
		case IceCandidate:
			log.Println("received ICE candidate")
			err = HandleIceCandidate(&message, &peer)
			if err != nil {
				log.Println("error handling ICE candidate", err)
				return
			}

		case Answer:
			err = HandleAnswer(&message, &peer, peers, conn)
			if err != nil {
				log.Println("error handling answer", err)
				return
			}

		case Offer:
			log.Println("received offer")
			err = HandleOffer(&message, &peer, peers, conn)
			if err != nil {
				log.Println("error handling offer", err)
				return
			}

		case NegotiationNeeded:
			for peer.peerConnection.SignalingState() != webrtc.SignalingStateStable {
				log.Println("need to negotiate but signaling is not stable")
				time.Sleep(500 * time.Millisecond)
			}

			peers.OfferPeer(peer.id, conn)

		default:
			log.Println("unknown message type", message.Type)
			return
		}
	}
}

func sendSessionDescription(sessionDescription *webrtc.SessionDescription, messageType int, conn *threadSafeWriter) error {
	sessionDescriptionBytes, err := json.Marshal(sessionDescription)
	if err != nil {
		return err
	}

	err = sendSignallingMessage(&SignallingMessage{
		Type: messageType,
		Data: string(sessionDescriptionBytes),
	}, conn)
	if err != nil {
		return err
	}

	return nil
}

func sendIceCandidate(candidate *webrtc.ICECandidate, conn *threadSafeWriter) error {
	candidateBytes, err := json.Marshal(candidate.ToJSON())
	if err != nil {
		return err
	}

	err = sendSignallingMessage(&SignallingMessage{
		Type: IceCandidate,
		Data: string(candidateBytes),
	}, conn)
	if err != nil {
		return err
	}

	return nil
}

func sendSignallingMessage(signallingMessage *SignallingMessage, conn *threadSafeWriter) error {
	err := conn.WriteJSON(&signallingMessage)
	if err != nil {
		return err
	}

	return nil
}

type threadSafeWriter struct {
	*websocket.Conn
	sync.Mutex
}

func (t *threadSafeWriter) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()

	return t.Conn.WriteJSON(v)
}
