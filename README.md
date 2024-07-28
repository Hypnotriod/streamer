## Streamer 
Streamer is a Go package that implements a Fan Out like pattern to distribute a real-time data stream to dynamically subscribed consumers.

# Usage
Typical usage would be a real-time asynchronous video, audio, telemetry, e.t.c. data streaming distribution. Where the number of the consumers can vary over time.  
Designed with [circular data buffer](https://en.wikipedia.org/wiki/Circular_buffer) approach in mind. Where the data buffer can be alocated beforehand. And devided into reusable chunks/packets.  
The idea of buffer `overrun` is introduced. If some of the `consumers` can no longer keep up with the `streamer`, data packets are being dropped for that particular `consumer` and `overrun` counter is incremented.  

# Example
```Go
import (
  ...
  strmr "github.com/Hypnotriod/streamer"
)

const CHUNKS_BUFFER_SIZE = 1024
const CHUNK_SIZE = 4096

type Chunk struct {
  Data [CHUNK_SIZE]byte
  Size int
}

func serveStreamer(conn net.Conn, streamer *strmr.Streamer[Chunk]) {
  buffer := [CHUNKS_BUFFER_SIZE]Chunk{}
  index := 0
  for {
    chunk := &buffer[index]
    index = (index + 1) % CHUNKS_BUFFER_SIZE
    size, _ := conn.Read(chunk.Data[:])
    ...
    buffer[index].Size = size
    if !strmr.Broadcast(chunk) {
	break
    }
  }
}

func serveConsumer(conn net.Conn, consumer *strmr.Consumer[Chunk]) {
  defer consumer.Close()
  for {
    chunk, ok = <-consumer.C
    if !ok {
      break
    }
    conn.Write(chunk.Data[:chunk.Size])
  }
}

...
streamer := strmr.NewStreamer[Chunk](strmr.BufferSizeFromTotal(CHUNKS_BUFFER_SIZE)).Run()
go serveStreamer(streamConn, streamer)
...
consumer := streamer.NewConsumer(strmr.BufferSizeFromTotal(CHUNKS_BUFFER_SIZE))
go serveConsumer(conn, consumer)
...
streamer.Stop()
```
