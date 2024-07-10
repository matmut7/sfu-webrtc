import { logger } from "$lib/logger";

export enum SignallingMessageType {
	IceCandidate,
	Offer,
	Answer
}

export interface SignallingMessage {
	type: SignallingMessageType;
	data: any;
}

export function initWebSocket(url: string, peerConnection: RTCPeerConnection) {
	const ws = new WebSocket(url);
	ws.addEventListener("open", () => {
		logger("WS open");
	});
	ws.addEventListener("close", () => {
		logger("WS close");
	});
	ws.addEventListener("message", (event) =>
		handleMessage(event, peerConnection, ws)
	);
	return ws;
}

export function sendSignallingMessage(
	signallingMessage: SignallingMessage,
	ws: WebSocket
) {
	const encodedMessage = {
		...signallingMessage,
		data: JSON.stringify(signallingMessage.data)
	};
	ws.send(JSON.stringify(encodedMessage));
}

async function handleMessage(
	messageEvent: MessageEvent,
	peerConnection: RTCPeerConnection,
	ws: WebSocket
) {
	const message: SignallingMessage = JSON.parse(messageEvent.data);

	try {
		if (
			message.type === SignallingMessageType.Offer ||
			message.type === SignallingMessageType.Answer
		) {
			const sessionDescription = new RTCSessionDescription(
				JSON.parse(message.data)
			);
			await peerConnection.setRemoteDescription(sessionDescription);
			if (sessionDescription.type === "offer") {
				await peerConnection.setLocalDescription();
				sendSignallingMessage(
					{
						type: SignallingMessageType.Answer,
						data: peerConnection.localDescription
					},
					ws
				);
			}
		} else if (message.type === SignallingMessageType.IceCandidate) {
			await peerConnection.addIceCandidate(
				new RTCIceCandidate(JSON.parse(message.data))
			);
		}
	} catch (err) {
		logger("error handling message", err);
	}
}
