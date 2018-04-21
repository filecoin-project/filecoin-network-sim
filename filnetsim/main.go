package main

import (
  "log"
  "context"
  "io"
  "fmt"
  "net/http"
  "io/ioutil"
  "time"

  network "github.com/filecoin-project/filnetsim/network"
)

type Instance struct {
  N *network.Network
  R network.Randomizer
  L io.Reader
}

func SetupInstance() *Instance {
  dir, err := ioutil.TempDir("", "filnetsim")
  if err != nil {
    dir = "/tmp/filnetsim"
  }

  n := network.NewNetwork(dir)
  r := network.Randomizer{
    Net:        n,
    TotalNodes: 10,
    BlockTime:  2 * time.Second,
    ActionTime: 500 * time.Millisecond,
    Actions:    []network.Action{
      network.ActionPayment,
      network.ActionAsk,
      network.ActionBid,
    },
  }

  l := n.Logs().Reader()
  return &Instance{n, r, l}
}

func (i *Instance) Run(ctx context.Context) {
  defer i.N.ShutdownAll()
  ctx, cancel := context.WithCancel(ctx)
  defer cancel()
  i.R.Run(ctx)
  <-ctx.Done()
}


func runService(ctx context.Context) {
  i := SetupInstance()
  // s.logs = i.L
  ctx, cancel := context.WithCancel(ctx)
  defer cancel()
  go i.Run(ctx)

  lh := NewLogHandler(ctx, i.L)

  // setup http
  http.Handle("/", http.FileServer(http.Dir("./filecoin-network-viz/viz-circle")))
  http.HandleFunc("/logs", lh.HandleHttp)
  // http.HandleFunc("/restart", RestartHandler)

  // run http
  fmt.Println("Listening at 127.0.0.1:7002/logs")
  log.Fatal(http.ListenAndServe(":7002", nil))
}

func main() {
  runService(context.Background())
}
