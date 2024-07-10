package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type webrtcContext struct {
	api    *webrtc.API
	config webrtc.Configuration
}

type Peers struct {
	peers map[string]Peer
	mutex sync.Mutex
}

func (p *Peers) GetPeer(peerId string) (Peer, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	peer, ok := p.peers[peerId]
	return peer, ok
}

func (p *Peers) SetPeer(peerId string, peer Peer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.peers[peerId] = peer
}

func (p *Peers) AddTrack(peerId string, track *webrtc.TrackLocalStaticRTP) {
	peer, ok := p.GetPeer(peerId)
	if !ok {
		log.Println("peer does not exist", peerId)
	}

	peer.localTracks = append(peer.localTracks, track)
	p.SetPeer(peerId, peer)

	// Add track to all other peers
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for k, v := range (*p).peers {
		p.mutex.Unlock()
		if k == peerId {
			p.mutex.Lock()
			continue
		}
		v.peerConnection.AddTrack(track)
		p.mutex.Lock()
	}
}

func (p *Peers) AddAllTracks(peerId string) {
	peer, ok := p.GetPeer(peerId)
	if !ok {
		log.Println("peer does not exist", peerId)
	}

	p.mutex.Lock()
	defer func() {
		p.mutex.Unlock()
		p.RequestKeyFrames()
	}()
	for k, v := range (*p).peers {
		p.mutex.Unlock()
		if k == peerId {
			p.mutex.Lock()
			continue
		}
		for _, track := range v.localTracks {
			p.mutex.Lock()
			peer.peerConnection.AddTrack(track)
			p.mutex.Unlock()
		}
		p.mutex.Lock()
	}
}

func (p *Peers) RemoveTrack(peerId string, trackId string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for k, v := range (*p).peers {
		p.mutex.Unlock()
		if peerId == k {
			// remove the track from the parent peer
			for index, track := range v.localTracks {
				if track.ID() == trackId {
					newLocalTracks := v.localTracks
					newLocalTracks[index] = newLocalTracks[len(newLocalTracks)-1]
					v.localTracks = newLocalTracks[:len(newLocalTracks)-1]
					p.SetPeer(k, v)
					break
				}
			}
		} else {
			// remove the track from all other peers' PeerConnections
			for _, sender := range v.peerConnection.GetSenders() {
				if sender.Track() != nil && sender.Track().ID() == trackId {
					v.peerConnection.RemoveTrack(sender)
				}
			}
		}
		p.mutex.Lock()
	}
}

func (p *Peers) RemovePeer(peerId string) {
	peer, ok := p.GetPeer(peerId)
	if !ok {
		log.Println("peer does not exist", peerId)
	}

	for _, track := range peer.localTracks {
		p.RemoveTrack(peerId, track.ID())
	}
}

func (p *Peers) RequestKeyFrames() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, peer := range (*p).peers {
		p.mutex.Unlock()
		for _, receiver := range peer.peerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			err := peer.peerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
			if err != nil {
				log.Println("error requesting key frame", err)
			}
		}
		p.mutex.Lock()
	}
}

func (p *Peers) OfferPeer(peerId string, conn *threadSafeWriter) {
	peer, ok := p.GetPeer(peerId)
	if !ok {
		log.Println("peer does not exist", peerId)
	}

	offer, err := peer.peerConnection.CreateOffer(nil)
	if err != nil {
		log.Println("error creating offer", err)
	}
	err = peer.peerConnection.SetLocalDescription(offer)
	if err != nil {
		log.Println("error setting local description", err)
	}
	log.Println("sending offer")
	err = sendSessionDescription(&offer, Offer, conn)
	if err != nil {
		log.Println("error sending offer", err)
	}
}

func (p *Peers) InitPeerConnection(webrtcContext *webrtcContext, conn *threadSafeWriter) (Peer, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtcContext.config)
	if err != nil {
		return Peer{}, err
	}

	peer := Peer{
		peerConnection: peerConnection,
		id:             uuid.NewString(),
	}

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Println("event: icecandidate")
		if candidate == nil {
			return
		}
		err = sendIceCandidate(candidate, conn)
		if err != nil {
			log.Println("error sending ICE candidate", err)
		}
	})

	peerConnection.OnSignalingStateChange(func(state webrtc.SignalingState) {
		log.Println("event: signalingstate", state)
	})

	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Println("event: track")
		localTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.ID(), remoteTrack.StreamID())
		if err != nil {
			log.Println("error creating local track", err)
		}

		p.AddTrack(peer.id, localTrack)
		p.RequestKeyFrames()

		// when we lose the track
		defer func() {
			p.RemoveTrack(peer.id, localTrack.ID())
		}()

		buf := make([]byte, 1500)
		for {
			i, _, err := remoteTrack.Read(buf)
			if err != nil {
				log.Println("error reading from remote track", err)
				return
			}

			if _, err = localTrack.Write(buf[:i]); err != nil {
				log.Println("error writing to local track", err)
				return
			}
		}
	})

	peerConnection.OnNegotiationNeeded(func() {
		log.Println("event: negotiationneeded")
		if peerConnection.SignalingState() != webrtc.SignalingStateStable {
			log.Println("need to negotiate but signaling is not stable")
			return
		}

		p.OfferPeer(peer.id, conn)
	})

	// fake transceiver
	peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv})
	if err != nil {
		return Peer{}, err
	}

	p.SetPeer(peer.id, peer)

	p.OfferPeer(peer.id, conn)

	go func() {
		for peerConnection.ConnectionState() != webrtc.PeerConnectionStateConnected {
			log.Println("waiting for PeerConnection...")
			time.Sleep(500 * time.Millisecond)
		}
		p.AddAllTracks(peer.id)
	}()

	return peer, nil
}

type Peer struct {
	peerConnection *webrtc.PeerConnection
	id             string
	localTracks    []*webrtc.TrackLocalStaticRTP
}

func InitWebRTC() (*webrtcContext, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, err
	}
	i.Add(intervalPliFactory)
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	config := webrtc.Configuration{
		ICEServers:   []webrtc.ICEServer{},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	return &webrtcContext{
		api,
		config,
	}, nil
}

func HandleIceCandidate(message *SignallingMessage, peer *Peer) error {
	candidate := webrtc.ICECandidateInit{}
	if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
		return err
	}

	peer.peerConnection.AddICECandidate(candidate)

	return nil
}

func HandleAnswer(message *SignallingMessage, peer *Peer, peers *Peers, conn *threadSafeWriter) error {
	answer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
		return err
	}

	err := peer.peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	return nil
}

func HandleOffer(message *SignallingMessage, peer *Peer, peers *Peers, conn *threadSafeWriter) error {
	if peer.peerConnection.SignalingState() != webrtc.SignalingStateStable {
		log.Println("need to negotiate but signaling is not stable")
		return nil
	}

	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(message.Data), &offer); err != nil {
		return err
	}

	err := peer.peerConnection.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	answer, err := peer.peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	err = peer.peerConnection.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	err = sendSessionDescription(&answer, Answer, conn)
	if err != nil {
		return err
	}

	return nil
}
