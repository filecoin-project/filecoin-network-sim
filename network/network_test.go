package network

import (
  "testing"
  // "io"
  "io/ioutil"
  // "bytes"
  // "encoding/json"

  "github.com/stretchr/testify/assert"
)

func TempDir(t *testing.T) string {
  dir, err := ioutil.TempDir("", "network test")
  assert.NoError(t, err)
  return dir
}


// func TestNetworkAddNode(t *testing.T) {
//   net := NewNetwork(TempDir(t))
//   defer net.ShutdownAll()

//   n1, err := net.AddNode()
//   assert.NoError(t, err)

//   n2, err := net.AddNode()
//   assert.NoError(t, err)

//   n3, err := net.AddNode()
//   assert.NoError(t, err)

//   assert.True(t, n1 == net.GetNode(0))
//   assert.True(t, n2 == net.GetNode(1))
//   assert.True(t, n3 == net.GetNode(2))
// }


// func TestNetworkAddNodes(t *testing.T) {
//   net := NewNetwork(TempDir(t))
//   defer net.ShutdownAll()

//   err := net.AddNodes(10)
//   assert.NoError(t, err)
// }

// func TestLogging(t *testing.T) {
//   net := NewNetwork(TempDir(t))
//   defer net.ShutdownAll()

//   buf := bytes.NewBuffer(nil)
//   go io.Copy(buf, net.Logs().Reader())

//   n1, err := net.AddNode()
//   assert.NoError(t, err)
//   n2, err := net.AddNode()
//   assert.NoError(t, err)
//   n3, err := net.AddNode()
//   assert.NoError(t, err)

//   _, err = n1.Connect(n3.Daemon)
//   assert.NoError(t, err)
//   assert.NoError(t, n1.Daemon.MiningOnce())
//   _, err = n1.Connect(n2.Daemon)
//   assert.NoError(t, err)
//   assert.NoError(t, n2.Daemon.MiningOnce())
//   assert.NoError(t, n3.Daemon.MiningOnce())

//   // check logs

//   check := []map[string]string{
//     {"type": "Connected"},
//     {"type": "NewBlockMined"},
//     {"type": "BroadcastBlock"},
//     {"type": "Connected"},
//     {"type": "NewBlockMined"},
//     {"type": "BroadcastBlock"},
//     {"type": "NewBlockMined"},
//     {"type": "BroadcastBlock"},
//   }

//   d := json.NewDecoder(buf)
//   for _, m1 := range check {
//     var m2 map[string]interface{}
//     assert.NoError(t, d.Decode(&m2))

//     for k, v := range m1 {
//       assert.Equal(t, m2[k], v)
//     }
//     t.Log(m2)
//   }
// }
