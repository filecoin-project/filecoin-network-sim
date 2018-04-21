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
    TotalNodes: 20,
    BlockTime:  500 * time.Millisecond,
    ActionTime: 100 * time.Millisecond,
    Actions:    []Action{
      ActionPayment,
      ActionAsk,
      ActionBid,
    },
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
  assert.True(t, counts["NewBlockMined"] > 1)
  assert.True(t, counts["MinerJoins"] > 1)
  assert.True(t, counts["BroadcastBlock"] > 1)
  assert.True(t, counts["AddAsk"] > 1)
  assert.True(t, counts["SendPayment"] > 1)
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
