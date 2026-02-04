package webadmin

import (
	"testing"
	"time"
)

func TestWebSocketHub_RegisterUnregister(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Close()

	// Create a mock client
	client := &WSClient{
		send:    make(chan []byte, 256),
		channel: "test",
		hub:     hub,
	}

	// Register
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount("test") != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount("test"))
	}

	// Unregister
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount("test") != 0 {
		t.Errorf("Expected 0 clients after unregister, got %d", hub.ClientCount("test"))
	}
}

func TestWebSocketHub_Broadcast(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Close()

	// Create two clients on the same channel
	client1 := &WSClient{
		send:    make(chan []byte, 256),
		channel: "test",
		hub:     hub,
	}
	client2 := &WSClient{
		send:    make(chan []byte, 256),
		channel: "test",
		hub:     hub,
	}

	// Create a client on a different channel
	client3 := &WSClient{
		send:    make(chan []byte, 256),
		channel: "other",
		hub:     hub,
	}

	hub.register <- client1
	hub.register <- client2
	hub.register <- client3
	time.Sleep(10 * time.Millisecond)

	// Broadcast to "test" channel
	hub.Broadcast("test", []byte("hello"))
	time.Sleep(10 * time.Millisecond)

	// client1 and client2 should receive the message
	select {
	case msg := <-client1.send:
		if string(msg) != "hello" {
			t.Errorf("client1 received %q, want %q", string(msg), "hello")
		}
	default:
		t.Error("client1 should have received a message")
	}

	select {
	case msg := <-client2.send:
		if string(msg) != "hello" {
			t.Errorf("client2 received %q, want %q", string(msg), "hello")
		}
	default:
		t.Error("client2 should have received a message")
	}

	// client3 should not receive the message (different channel)
	select {
	case <-client3.send:
		t.Error("client3 should not have received a message")
	default:
		// Expected
	}
}

func TestWebSocketHub_TotalClientCount(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Close()

	// Create clients on different channels
	for i := 0; i < 3; i++ {
		client := &WSClient{
			send:    make(chan []byte, 256),
			channel: "channel1",
			hub:     hub,
		}
		hub.register <- client
	}

	for i := 0; i < 2; i++ {
		client := &WSClient{
			send:    make(chan []byte, 256),
			channel: "channel2",
			hub:     hub,
		}
		hub.register <- client
	}

	time.Sleep(10 * time.Millisecond)

	if hub.TotalClientCount() != 5 {
		t.Errorf("Expected 5 total clients, got %d", hub.TotalClientCount())
	}

	if hub.ClientCount("channel1") != 3 {
		t.Errorf("Expected 3 clients in channel1, got %d", hub.ClientCount("channel1"))
	}

	if hub.ClientCount("channel2") != 2 {
		t.Errorf("Expected 2 clients in channel2, got %d", hub.ClientCount("channel2"))
	}
}

func TestWebSocketHub_Close(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	client := &WSClient{
		send:    make(chan []byte, 256),
		channel: "test",
		hub:     hub,
	}

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Close hub
	hub.Close()
	time.Sleep(10 * time.Millisecond)

	// Hub should have no clients
	if hub.TotalClientCount() != 0 {
		t.Errorf("Expected 0 clients after close, got %d", hub.TotalClientCount())
	}
}
