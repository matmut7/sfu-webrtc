<script lang="ts">
	import { onMount } from "svelte";
	import PeerComponent from "../components/Peer.svelte";
	import { PUBLIC_WS_SERVER_URL } from "$env/static/public";
	import { logger } from "$lib/logger";
	import { initWebSocket } from "./signalling";
	import {
		initPeerConnection,
		type Peer,
		type peerId,
		type Peers
	} from "./webrtc";
	import { writable } from "svelte/store";

	let peers: Peers = {};
	let ws: WebSocket;
	let localStream: MediaStream;
	let roomContainer: HTMLDivElement;
	let size = 0;
	let peerConnection: RTCPeerConnection;
	const makingOffer = writable(false);

	function addPeer(peer: Peer) {
		peers[peer.id] = peer;
		peers = peers;
	}

	function removePeer(peerId: peerId) {
		delete peers[peerId];
		peers = peers;
	}

	function updateSize() {
		if (roomContainer && roomContainer.parentElement) {
			if (
				roomContainer.parentElement.clientWidth >
				roomContainer.parentElement.clientHeight
			) {
				size = roomContainer.parentElement.clientHeight;
			} else {
				size = roomContainer.parentElement.clientWidth;
			}
		}
	}
	$: gridSize = Math.ceil(Math.sqrt(Object.keys(peers).length + 1));

	onMount(() => {
		(async () => {
			const resizeObserver = new ResizeObserver(updateSize);
			if (roomContainer && roomContainer.parentElement) {
				resizeObserver.observe(roomContainer.parentElement);
			}

			peerConnection = new RTCPeerConnection();
			localStream = await navigator.mediaDevices.getUserMedia({ video: true });

			ws = initWebSocket(PUBLIC_WS_SERVER_URL, peerConnection);
			while (ws.readyState !== ws.OPEN) {
				logger("waiting for WS...");
				await new Promise((r) => setTimeout(r, 500));
			}

			peerConnection = initPeerConnection(
				peerConnection,
				ws,
				addPeer,
				removePeer,
				makingOffer
			);
			while (peerConnection.connectionState !== "connected") {
				logger("waiting for PeerConnection...");
				await new Promise((r) => setTimeout(r, 500));
			}

			localStream
				.getTracks()
				.forEach((track) => peerConnection.addTrack(track, localStream));

			return () => {
				resizeObserver.disconnect();
				ws.close();
			};
		})();
	});
</script>

<main>
	<div
		bind:this={roomContainer}
		id="room-container"
		style="width: {size}px; height: {size}px;"
	>
		<div style="width: {size / gridSize}px; height: {size / gridSize}px">
			<PeerComponent mirrorVideo={true} mediaStream={localStream} />
		</div>
		{#each Object.values(peers) as peer}
			<div style="width: {size / gridSize}px; height: {size / gridSize}px">
				<PeerComponent mediaStream={peer.stream} />
			</div>
		{/each}
	</div>
</main>

<style>
	main {
		width: 95vw;
		height: 95vh;
		overflow: hidden;
		margin: auto;
		display: flex;
		align-content: center;
		align-items: center;
	}

	#room-container {
		margin: auto;
		display: flex;
		flex-direction: row;
		flex-wrap: wrap;
		align-items: center;
		align-content: center;
		justify-content: center;
	}
</style>
