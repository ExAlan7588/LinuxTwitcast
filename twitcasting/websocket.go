package twitcasting

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sacOO7/gowebsocket"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

var (
	wsReconnectGracePeriod   = 2 * time.Minute
	wsReconnectRetryInterval = 2 * time.Second
	wsReconnectURLFetcher    = func(streamer, password string) (string, error) {
		lookup, err := GetWSStreamUrlWithPassword(streamer, password)
		return lookup.StreamURL, err
	}
)

func RecordWS(recordCtx record.RecordContext, sinkChan chan<- []byte) {
	defer close(sinkChan)

	streamer := recordCtx.GetStreamer()
	streamURL := recordCtx.GetStreamUrl()

	for {
		if recordCtx.Err() != nil {
			return
		}

		socket, disconnectedCh := newRecordingSocket(recordCtx, streamURL, sinkChan)
		socket.Connect()

		select {
		case <-recordCtx.Done():
			if socket.IsConnected {
				socket.Close()
			}
			return
		case err := <-disconnectedCh:
			if recordCtx.Err() != nil {
				return
			}

			// TwitCasting 的 WebSocket 會偶發短暫斷線；給更長的寬限期重抓 stream URL，
			// 避免把同一場直播切成多段並重複發 Discord/Telegram 通知。
			nextURL, reconnectErr := waitForReconnectURL(recordCtx, streamer, err)
			if reconnectErr != nil {
				log.Printf("Stream [%s] reconnect grace period expired: %v\n", streamer, reconnectErr)
				recordCtx.Cancel()
				return
			}
			streamURL = nextURL
		}
	}
}

func newRecordingSocket(recordCtx record.RecordContext, streamURL string, sinkChan chan<- []byte) (*gowebsocket.Socket, <-chan error) {
	socket := gowebsocket.New(streamURL)
	streamer := recordCtx.GetStreamer()
	disconnectedCh := make(chan error, 1)
	var disconnectOnce sync.Once

	socket.ConnectionOptions = gowebsocket.ConnectionOptions{
		UseSSL:         true,
		UseCompression: false,
	}

	socket.RequestHeader.Set("Origin", baseDomain)
	socket.RequestHeader.Set("User-Agent", userAgent)

	signalDisconnect := func(err error) {
		if err == nil {
			err = fmt.Errorf("websocket disconnected")
		}
		disconnectOnce.Do(func() {
			select {
			case disconnectedCh <- err:
			default:
			}
		})
	}

	socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Println("Error connecting to stream URL: ", err)
		signalDisconnect(err)
	}
	socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Printf("Connected to live stream for [%s], recording start \n", streamer)
	}
	socket.OnTextMessage = func(message string, socket gowebsocket.Socket) {
		log.Println("Received message", message)
	}
	socket.OnBinaryMessage = func(data []byte, socket gowebsocket.Socket) {
		select {
		case <-recordCtx.Done():
			return
		case sinkChan <- data:
		}
	}
	socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		log.Printf("Disconnected from live stream of [%s] \n", streamer)
		signalDisconnect(err)
	}

	return &socket, disconnectedCh
}

func waitForReconnectURL(recordCtx record.RecordContext, streamer string, disconnectErr error) (string, error) {
	deadline := time.Now().Add(wsReconnectGracePeriod)
	password := recordCtx.GetPassword()
	log.Printf("Stream [%s] disconnected, waiting up to %s for reconnect: %v\n", streamer, wsReconnectGracePeriod, disconnectErr)

	for {
		if recordCtx.Err() != nil {
			return "", recordCtx.Err()
		}

		streamURL, err := wsReconnectURLFetcher(streamer, password)
		if err == nil {
			log.Printf("Stream [%s] reconnected within grace period\n", streamer)
			return streamURL, nil
		}

		if time.Now().After(deadline) {
			return "", err
		}

		timer := time.NewTimer(wsReconnectRetryInterval)
		select {
		case <-recordCtx.Done():
			timer.Stop()
			return "", recordCtx.Err()
		case <-timer.C:
		}
	}
}
