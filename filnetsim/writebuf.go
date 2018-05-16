package main

import (
  "io"
  "container/ring"
)

type RingWriter struct {
  ring *ring.Ring
}

func NewRingWriter(size int) *RingWriter {
  return &RingWriter{ring.New(size)}
}

func (b *RingWriter) Write(buf []byte) (int, error) {
  // copy it because we cant keep buf
  buf2 := make([]byte, len(buf))
  copy(buf2, buf)

  b.ring.Value = buf2
  b.ring = b.ring.Next() // advance the ring. points to the oldest (or unused) entry
  return len(buf2), nil
}

func (b *RingWriter) WriteTo(w io.Writer) (int, error) {
  total := 0
  hasFailed := false
  var outErr error

  // printing the ring (from the handle we keep) starts from the oldest entry.
  b.ring.Do(func(v interface{}) {
    if hasFailed {
      return
    }

    if v == nil {
      return
    }

    buf := v.([]byte)
    if len(buf) < 1 {
      return
    }

    n, err := w.Write(buf)
    total += n
    if err != nil {
      outErr = err
      hasFailed = true // short circuit from now on.
    }
  })

  return total, outErr
}
