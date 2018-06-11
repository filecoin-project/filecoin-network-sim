package main

import (
	"context"
	"io"
	"log"
	"net/http"
)

const (
	LogBufferSize = 5000 // how many old messages to keep for new writers.
)

type LogHandler struct {
	ctx  context.Context
	logs io.Reader
	wch  chan io.Writer
}

func NewLogHandler(ctx context.Context, logs io.Reader) *LogHandler {
	lh := &LogHandler{ctx, logs, make(chan io.Writer, 20)}
	go lh.PipeLogsToWriters()
	return lh
}

func (l *LogHandler) HandleHttp(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	l.AddWriter(w)
	l.Wait()
}

func (l *LogHandler) Wait() {
	<-l.ctx.Done()
}

func (l *LogHandler) PipeLogsToWriters() {
	// buffer to store last n logs for new clients
	// need to keep full writes, not bytes.
	// n * 300 chars per write = ~300n Bytes of storage
	lastLogs := NewRingWriter(LogBufferSize)

	// wait for the first writer before reading starts.
	w := <-l.wch
	ws := []io.Writer{w}

	buf := make([]byte, 2048)
	for {
		// read from logs. (blocks here)
		n, err := l.logs.Read(buf)
		if err != nil {
			return
		}
		bytesRead := buf[:n]

		select {
		case w := <-l.wch:
			newWs := DrainWriterCh(l.wch)
			newWs = append(newWs, w)

			// first, catch them up.
			log.Println("catching up new writers", len(newWs))
			for i, w := range newWs {
				_, err := lastLogs.WriteTo(w)
				if err != nil {
					log.Println("failed to write logs to new writer.")
					newWs[i] = nil // sad that it failed so soon...
					continue
				}
				tryFlushing(w)
			}

			ws = append(ws, newWs...) // track new writers.
			ws = pruneNils(ws)        // remove dead writers
		case <-l.ctx.Done():
			return // ok bye
		default:
		}

		// write to all writers
		for i, w := range ws {
			if w == nil {
				continue // skip dead writer.
			}

			_, err := w.Write(bytesRead)
			if err != nil {
				log.Println("failed to write logs to writer. removed.")
				ws[i] = nil // delete/gc writer
				continue
			}

			tryFlushing(w)
		}

		// store in buffer
		lastLogs.Write(bytesRead)
	}
}

func pruneNils(l1 []io.Writer) []io.Writer {
	var l2 []io.Writer
	for _, w := range l1 {
		if w != nil {
			l2 = append(l2, w)
		}
	}
	return l2
}

func DrainWriterCh(wch <-chan io.Writer) []io.Writer {
	var ws []io.Writer
	for {
		select {
		case w := <-wch:
			ws = append(ws, w)
		default:
			return ws
		}
	}
}

func (l *LogHandler) AddWriter(w io.Writer) {
	l.wch <- w
}

type Flusher interface {
	io.Writer
	Flush()
}

func tryFlushing(w io.Writer) {
	if w == nil {
		return
	}
	if wf, ok := w.(Flusher); ok {
		wf.Flush()
	}
}
