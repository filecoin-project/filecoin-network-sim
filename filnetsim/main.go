package main

import (
  "log"
  "context"
  "io"
  "os"
  "fmt"
  "net/http"
  "io/ioutil"
  "time"

  network "github.com/filecoin-project/filnetsim/network"
)

// type Service struct {
//   ctx  context.Context
//   i    *Instance
//   logs io.Reader
// }

// func (s *Service) Run() {
//   for {
//     i := SetupNetwork()
//     s.logs := i.L
//     ctx := s.ctx
//     i.Run(ctx)
//   }
// }

// func (s *Service) Logs() io.Reader {

// }

type Instance struct {
  N *network.Network
  R network.Randomizer
  L io.Reader
}

func SetupNetwork() *Instance {
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
  i := SetupNetwork()
  // s.logs = i.L
  ctx, cancel := context.WithCancel(ctx)
  defer cancel()
  go i.Run(ctx)

  // setup http
  http.Handle("/", http.FileServer(http.Dir("./filecoin-network-viz/viz-circle")))
  http.HandleFunc("/logs", LogsHandler(i.L))
  http.HandleFunc("/restart", RestartHandler)

  // run http
  fmt.Println("Listening at 127.0.0.1:7002/logs")
  log.Fatal(http.ListenAndServe(":7002", nil))
}

// hello world, the web server
func RestartHandler(w http.ResponseWriter, req *http.Request) {

}

type HTTPHandler func(w http.ResponseWriter, req *http.Request)

// hello world, the web server
func LogsHandler(logs io.Reader) HTTPHandler {
  return func(w http.ResponseWriter, req *http.Request) {
    w.WriteHeader(http.StatusOK)
    wf := w.(Flusher)
    w2 := io.MultiWriter(wf, os.Stdout)
    buf := make([]byte, 2048)
    for {
      n, err := logs.Read(buf)
      if err != nil {
        return
      }

      w2.Write(buf[:n])
      wf.Flush()
    }
  }
}

type Flusher interface {
  io.Writer

  Flush()
}

func main() {
  runService(context.Background())
}
