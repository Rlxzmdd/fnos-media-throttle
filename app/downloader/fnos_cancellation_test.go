package downloader

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestFNOSCancellationInterruptsStalledHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	requestRead := make(chan struct{})
	peerClosed := make(chan struct{})
	go func() {
		defer close(peerClosed)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		close(requestRead)
		// Receive the upgrade but never answer it. Cancellation must close TCP.
		_, _ = io.Copy(io.Discard, conn)
	}()
	result := make(chan error, 1)
	go func() {
		gateway, err := dialFNOSGateway(ctx, Downloader{BaseURL: "ws://" + listener.Addr().String() + "/websocket"})
		if gateway != nil {
			_ = gateway.Close()
		}
		result <- err
	}()
	select {
	case <-requestRead:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake did not reach peer")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled handshake succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("handshake ignored cancellation")
	}
	select {
	case <-peerClosed:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked TCP connection")
	}
}
