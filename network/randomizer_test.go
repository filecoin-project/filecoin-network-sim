package network

import (
  "testing"
  "io"
  "bytes"
  "encoding/json"
  "time"
  "context"

  "github.com/stretchr/testify/assert"
)



func TestRandomizer(t *testing.T) {
  net := NewNetwork(TempDir(t))
  defer net.ShutdownAll()

  buf := bytes.NewBuffer(nil)
  go io.Copy(buf, net.Logs().Reader())

  r := Randomizer{
    Net:        net,
    TotalNodes: 10,
    BlockTime:  100 * time.Millisecond,
    ActionTime: 300 * time.Millisecond,
    Actions:    []Action{ActionPayment},
  }

  ctx, cancel := context.WithCancel(context.Background())
  r.Run(ctx)

  runDuration := 5 * time.Second
  time.Sleep(runDuration)
  cancel()
  time.Sleep(time.Second)
  // wait till done. want goprocess.

  // s := string(buf.Bytes())
  // rd := bytes.NewBuffer([]byte(s))
  counts := CountLogs(t, buf)
  t.Log(counts)
  // assert.True(t, counts["NewBlockMined"] > int(runDuration / r.BlockTime))
}

func CountLogs(t *testing.T, r io.Reader) map[string]int {
  counts := map[string]int{}
  d := json.NewDecoder(r)
  for {
    var m map[string]interface{}
    err := d.Decode(&m)
    if err != nil {
      break
    }

    t := m["type"].(string)
    counts[t] = counts[t] + 1
  }
  return counts
}
