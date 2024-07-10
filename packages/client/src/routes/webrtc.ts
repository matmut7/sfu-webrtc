import { logger } from "$lib/logger";
import type { Writable } from "svelte/store";
import { sendSignallingMessage, SignallingMessageType } from "./signalling";

export type peerId = `${string}-${string}-${string}-${string}-${string}`;

export interface Peer {
	id: peerId;
	stream: MediaStream;
}

export type Peers = Record<peerId, Peer>;

export function initPeerConnection(
	peerConnection: RTCPeerConnection,
	ws: WebSocket,
	addPeer: (peer: Peer) => void,
	removePeer: (peerId: peerId) => void,
	makingOffer: Writable<boolean>
) {
	peerConnection.onicecandidate = (event) => {
		logger("event: icecandidate");
		if (event.candidate) {
			sendSignallingMessage(
				{
					type: SignallingMessageType.IceCandidate,
					data: event.candidate
				},
				ws
			);
		}
	};

	peerConnection.onsignalingstatechange = () => {
		logger("event: signalingstate", peerConnection.signalingState);
	};

	peerConnection.onconnectionstatechange = () => {
		logger("event: connectionstate", peerConnection.connectionState);
	};

	peerConnection.ontrack = (event) => {
		logger("event: track");
		const stream = event.streams[0];
		addPeer({ id: stream.id as peerId, stream });

		stream.onremovetrack = (_) => {
			removePeer(stream.id as peerId);
		};
	};

	peerConnection.oniceconnectionstatechange = () => {
		if (peerConnection.iceConnectionState === "failed") {
			peerConnection.restartIce();
		}
	};

	peerConnection.onicegatheringstatechange = () => {
		logger("event: icegatheringstatechange", peerConnection.iceGatheringState);
	};

	peerConnection.onnegotiationneeded = async () => {
		logger("event: negotiationneeded");

		try {
			makingOffer.set(true);
			await peerConnection.setLocalDescription();
			sendSignallingMessage(
				{
					type: SignallingMessageType.Offer,
					data: peerConnection.localDescription
				},
				ws
			);
		} catch (err) {
			logger("error negitiating", err);
		} finally {
			makingOffer.set(false);
		}
	};

	return peerConnection;
}
