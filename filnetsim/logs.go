package main

import (
  "context"
  "io"
  "net/http"
)

type LogHandler struct {
  ctx  context.Context
  logs io.Reader
  wch  chan io.Writer
}

func NewLogHandler(ctx context.Context, logs io.Reader) *LogHandler {
  lh := &LogHandler{ctx, logs, make(chan io.Writer)}
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
  // wait for the first writer.
  w := <-l.wch
  ws := []io.Writer{&OptimisticWriter{w}}
  mw := io.MultiWriter(ws...)

  buf := make([]byte, 2048)
  for {
    select {
    case w := <-l.wch:
      ws = append(ws, &OptimisticWriter{w})
      ws = append(ws, DrainWriterCh(l.wch)...)
      mw = io.MultiWriter(ws...)
    case <-l.ctx.Done():
      return // ok bye
    default:
    }

    n, err := l.logs.Read(buf)
    if err != nil {
      return
    }

    mw.Write(buf[:n])
  }
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
  l.wch<- w
}

type Flusher interface {
  io.Writer
  Flush()
}

type OptimisticWriter struct {
  W io.Writer
}

func (w *OptimisticWriter) Write(data []byte) (n int, err error) {
  if w.W == nil {
    return len(data), nil // discard behavior.
  }

  n, err = w.W.Write(data)
  if err != nil {
    w.W = nil // good times are over
    return len(data), nil
  }
  if f, ok := w.W.(Flusher); ok {
    f.Flush()
  }
  return n, err
}