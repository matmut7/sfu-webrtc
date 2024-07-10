<script lang="ts">
	export let mediaStream: MediaStream | undefined = undefined;
	export let mirrorVideo: boolean | undefined = undefined;
	let video: HTMLVideoElement;

	$: if (mediaStream && video) {
		video.srcObject = mediaStream;
		video.play();
	}
</script>

<div class="video-container">
	<video class={mirrorVideo ? "mirrored" : ""} bind:this={video} autoplay>
		<track kind="captions" />
	</video>
</div>

<style>
	.video-container {
		width: 100%;
		height: 100%;
		padding: 0.1rem;
		box-sizing: border-box;
		overflow: hidden;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	video {
		object-fit: cover;
		min-width: 100%;
		min-height: 100%;
		width: auto;
		height: auto;
	}

	.mirrored {
		-webkit-transform: scaleX(-1);
		transform: scaleX(-1);
	}
</style>
