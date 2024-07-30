## Streamer 
Streamer is a Go package that implements a Fan Out like pattern to distribute a real-time data stream to dynamically subscribed consumers.

# Usage
A typical use would be the distribution of an isochronous data stream (video, audio, telemetry, etc.) in real time. Where the number of the consumers can vary over time.  
Designed with [circular buffer](https://en.wikipedia.org/wiki/Circular_buffer) approach in mind. The data buffer can be allocated once and reuse, offloading the garbage collector.  
To avoid situations where the `Streamer` broadcast may be blocked by some `Consumers` because their channel is full, the idea of ​​buffer `overrun` is introduced. If some of the `Consumers` can no longer keep up with the `Streamer`, data packets are being dropped for that particular `Consumer` and `overrun` counter is incremented. If the number of dropped packets exceeds the `Streamer` buffer size, the `Consumer` will be closed by the `Streamer`.  

# Example
```Go
import (
  ...
  strmr "github.com/Hypnotriod/streamer"
)

const CHUNKS_BUFFER_SIZE = 1024
const CHUNK_SIZE = 4096

// Example of data chunk broadcasted by the Streamer
type Chunk struct {
  Data [CHUNK_SIZE]byte
  Size int
}

func serveStreamer(conn net.Conn, streamer *strmr.Streamer[Chunk]) {
  // Allocate contiguous memory buffer
  buffer := [CHUNKS_BUFFER_SIZE]Chunk{}
  index := 0
  for {
    // Take the pointer to the next chunk of data to fill
    chunk := &buffer[index]
    // Increment and wrap around the next chunk index 
    index = (index + 1) % CHUNKS_BUFFER_SIZE
    // Fill the chunk data array
    size, _ := conn.Read(chunk.Data[:])
    buffer[index].Size = size
    // Broadcast the next data chunk pointer
    if !streamer.Broadcast(chunk) {
      // Streamer was stopped
      break
    }
  }
}

func serveConsumer(conn net.Conn, consumer *strmr.Consumer[Chunk]) {
  defer consumer.Close()
  for {
    // Read the next data chunk pointer
    chunk, ok := <-consumer.C
    if !ok {
      // Consumer was closed
      break
    }
    conn.Write(chunk.Data[:chunk.Size])
  }
}

...
// Create a new Streamer of the Chunk data with (CHUNKS_BUFFER_SIZE / 2 - 2) buffer size
// Use the BufferSizeFromTotal function to calculate Streamer and Consumer buffer size in case of circular buffer
streamer := strmr.NewStreamer[Chunk](strmr.BufferSizeFromTotal(CHUNKS_BUFFER_SIZE)).Run()
go serveStreamer(streamConn, streamer)
...
// Create a new Consumer of the Chunk data with (CHUNKS_BUFFER_SIZE / 2 - 2) buffer size
consumer := streamer.NewConsumer(strmr.BufferSizeFromTotal(CHUNKS_BUFFER_SIZE))
go serveConsumer(consumerConn, consumer)
...
// Stop the Streamer and all subscribed Consumers
streamer.Stop()
```
