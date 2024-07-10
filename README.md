# SFU WebRTC

This is an educational implementation of a WebRTC SFU using Go and Pion.

## Client

The client is a simple SvelteKit app. We need a simple environment variable to
point to the server host:

```sh
cd packages/client
echo 'PUBLIC_WS_SERVER_URL="http://localhost:8080/ws"' > .env.local
yarn install
yarn dev
```

## Server

The Go WebRTC and WS signalling server.

```sh
cd packages/server
make build
make run
```
