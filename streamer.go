// Copyright 2024, Illia Pikin a.k.a. Hypnotriod. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package streamer

import (
	"sync"
	"sync/atomic"
)

// Client structure represents the consumer of the data stream.
// Exposes receive-only channel C of the data type *T. May be closed by the Client by calling the Close method, or
// by the Streamer in case of the Stop call or overrun.
type Client[T any] struct {
	streamer *Streamer[T]
	input    chan<- *T
	overrun  atomic.Uintptr
	C        <-chan *T
}

// Close method closes the receive-only channel C and removes the Client from the Streamer.
// Should be called if the Client is no longer consuming the data stream.
func (c *Client[T]) Close() {
	for {
		select {
		case _, ok := <-c.C:
			if !ok {
				return
			}
		case c.streamer.remove <- c:
			return
		}
	}
}

// Overrun function returns the number of data packets missed by the Client,
// what happens when the Client can't keep up with the Streamer.
// In any time the Streamer tries to broadcast next data packet and the Client
// channel is already full - overrun occurres, and the data packet will be discarded for this Client.
func (c *Client[T]) Overrun() uint {
	return uint(c.overrun.Load())
}

// Streamer structure represents the producer of the data stream.
type Streamer[T any] struct {
	mu        sync.Mutex
	isRunning bool
	clients   map[*Client[T]]bool
	add       chan *Client[T]
	remove    chan *Client[T]
	broadcast chan *T
	stop      chan bool
}

// BufferSizeFromTotal function returns the buffer size for the Streamer, as well as for the Client
// when using the circular buffer approach. Accepts the total size of the circular buffer,
// which must be at least 6.
// In the case of the Streamer, it is half the size of the circular buffer minus one unprepared
// packet and minus one extra carry over packet.
// In the case of the Client, it is half the size of the circular buffer minus one unhandled
// packet and minus one extra carry over packet.
func BufferSizeFromTotal(total int) int {
	if total < 6 {
		return 0
	}
	return total/2 - 2
}

// NewStreamer creates the new Streamer of the data type T with the broadcast channel buffer size of buffSize.
// buffSize is recommended to be at least 1 to prevent blocking when broadcasting data.
func NewStreamer[T any](buffSize int) *Streamer[T] {
	return &Streamer[T]{
		clients:   make(map[*Client[T]]bool),
		add:       make(chan *Client[T]),
		remove:    make(chan *Client[T]),
		broadcast: make(chan *T, buffSize),
		stop:      make(chan bool),
	}
}

// NewClient creates the new subscribed Client of the data type T with the receive channel buffer size of buffSize.
// buffSize should be at least 1, otherwise, no data packet will ever be received by the Client.
func (s *Streamer[T]) NewClient(buffSize int) *Client[T] {
	ch := make(chan *T, buffSize)
	c := &Client[T]{
		streamer: s,
		input:    ch,
		C:        ch,
	}
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		close(ch)
		return c
	}
	c.streamer.add <- c
	s.mu.Unlock()
	return c
}

// IsRunning function returns true if the Streamer is still running.
func (s *Streamer[T]) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// Broadcast function transmits the next data packet of type *T to all subscribed Clients.
// Returns true if the Streamer is still running, false if the Streamer routine
// has not been started or the Stop method has been called.
// Will block if broadcast channel is full, till The Streamer routine pulls the data.
func (s *Streamer[T]) Broadcast(data *T) bool {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return false
	}
	s.broadcast <- data
	s.mu.Unlock()
	return true
}

// Run function starts the Streamer routine.
// Returns the Streamer pointer to be able to chain with the NewStreamer function call.
func (s *Streamer[T]) Run() *Streamer[T] {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return s
	}
	s.isRunning = true
	s.mu.Unlock()
	go s.run()
	return s
}

// The Streamer routine method.
func (s *Streamer[T]) run() {
	for {
		select {
		case <-s.stop:
			for client := range s.clients {
				close(client.input)
			}
			clear(s.clients)
			for _, ok := <-s.broadcast; ok; {
			}
			return
		case client := <-s.add:
			s.clients[client] = true
		case client := <-s.remove:
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.input)
			}
		case packet := <-s.broadcast:
			for client := range s.clients {
				select {
				case client.input <- packet:
					client.overrun.Store(0)
				default:
					overrun := client.overrun.Add(1)
					if int(overrun) > cap(s.broadcast) {
						delete(s.clients, client)
						close(client.input)
					}
				}
			}
		}
	}
}

// Stop stops the Streamer and all subscribed Clients.
// Returns true if the Streamer was closed successfully, false if the Streamer routine
// has not been started or the Stop method has already been called.
func (s *Streamer[T]) Stop() bool {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return false
	}
	s.isRunning = false
	s.stop <- true
	s.mu.Unlock()
	return true
}
