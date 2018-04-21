package logs

import (
  "testing"
  "io"
  "bufio"
  "time"
  "bytes"
  "math/rand"

  "github.com/stretchr/testify/assert"
)

func FilledBuf(c byte, n int) []byte {
  buf := make([]byte, n)
  for i := 0; i < n; i++ {
    buf[i] = c
  }
  return buf
}

func randSleep(usmax int) {
  time.Sleep(time.Duration(rand.Intn(usmax)) * time.Microsecond)
}

func WriteErratically(w io.WriteCloser, c byte, epochs int) {
  hunk := FilledBuf(c, 2048)

  for e := 0; e < epochs; e++ {
    randSleep(15)
    for k := 0; k < int(rand.Intn(10)); k++ {
      randSleep(5)
      r := rand.Intn(len(hunk))
      w.Write(hunk[0:r])
    }
    w.Write([]byte{'\n'})
  }

  w.Close()
}

func TestAggregatorBasic(t *testing.T) {

  l := NewLineAggregator()

  msg := []byte("hello there!\n")
  b1 := bytes.NewBuffer(nil)
  b1.Write(msg)
  l.MixReader(b1)

  buf := make([]byte, 2048)
  n, err := l.Reader().Read(buf)
  assert.NoError(t, err)
  assert.Equal(t, n, len(msg))
}

func TestAggregatorThree(t *testing.T) {

  l := NewLineAggregator()

  msg := []byte("hello there! -- General Kenobi\n")
  for i := 0; i < 3; i++ {
    b := bytes.NewBuffer(nil)
    b.Write(msg)
    l.MixReader(b)
  }

  for i := 0; i < 3; i++ {
    msg2 := make([]byte, len(msg))
    n, err := l.Reader().Read(msg2)
    assert.NoError(t, err)
    assert.Equal(t, n, len(msg))
    assert.Equal(t, msg, msg2)
  }
}


func TestAggregatorStress(t *testing.T) {
  t.Skip()

  epochs := 10 // 50
  writers := 20 // 100
  alpha := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

  l := NewLineAggregator()

  for i := 0; i < writers; i++ {
    go func(i int) {
      c := alpha[i % len(alpha)]
      r, w := io.Pipe()
      t.Log("WriteErratically", i, c, epochs)
      l.MixReader(r)
      WriteErratically(w, byte(c), epochs)
    }(i)
  }

  // consume all lines.
  s := bufio.NewReader(l.Reader())
  linesRead := 0
  bytesRead := 0
  defer func() {
    t.Logf("Read %d lines, %d bytes", linesRead, bytesRead)
  }()

  for linesRead < epochs * writers {
    l, err := s.ReadBytes('\n')
    if err == io.EOF {
      t.Log("Got EOF")
      break // keep going...
    }
    if err != nil {
      assert.NoError(t, err)
      t.Error(err)
      break // what's the error
    }
    bytesRead += len(l)
    linesRead++

    if len(l) <= 1 {
      continue // end newline
    }

    // trim newline
    l = l[0:len(l) - 1]

    // check line is all the same.
    c := l[0]
    for i := 0; i < len(l); i++ {
      assert.Equal(t, l[i], c)
    }
  }
}
