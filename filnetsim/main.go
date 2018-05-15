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
  "flag"

  network "github.com/filecoin-project/filecoin-network-sim/network"
  daemon "github.com/filecoin-project/filecoin-network-sim/daemon"
)

const (
  VizDir = "./filecoin-network-viz/viz-circle"
)

var opts = struct {
  Debug bool
}{}

func init() {
  flag.BoolVar(&opts.Debug, "debug", false, "turns on debug logging")
  flag.Parse()
}

type Instance struct {
  N *network.Network
  R network.Randomizer
  L io.Reader
}

func SetupInstance() (*Instance, error) {
  dir, err := ioutil.TempDir("", "filnetsim")
  if err != nil {
    dir = "/tmp/filnetsim"
  }

  n, err := network.NewNetwork(dir)
  if err != nil {
    return nil, err
  }

  r := network.Randomizer{
    Net:        n,
    TotalNodes: 30,
    BlockTime:  2 * time.Second,
    ActionTime: 1000 * time.Millisecond,
    Actions:    []network.Action{
      network.ActionPayment,
      network.ActionAsk,
      network.ActionBid,
    },
  }

  l := n.Logs().Reader()
  return &Instance{n, r, l}, nil
}

func (i *Instance) Run(ctx context.Context) {
  defer i.N.ShutdownAll()
  ctx, cancel := context.WithCancel(ctx)
  defer cancel()
  i.R.Run(ctx)
  <-ctx.Done()
}


func runService(ctx context.Context) error {
  i, err := SetupInstance()
  if err != nil {
    return err
  }

  // s.logs = i.L
  ctx, cancel := context.WithCancel(ctx)
  defer cancel()
  go i.Run(ctx)

  lh := NewLogHandler(ctx, i.L)

  // setup http
  http.Handle("/", http.FileServer(http.Dir(VizDir)))
  http.HandleFunc("/logs", lh.HandleHttp)
  // http.HandleFunc("/restart", RestartHandler)

  // run http
  fmt.Println("Listening at 127.0.0.1:7002/logs")
  return http.ListenAndServe(":7002", nil)
}

func run() error {

  // handle options
  if opts.Debug {
    log.SetOutput(os.Stderr)
  } else {
    log.SetOutput(ioutil.Discard)
  }

  // check we're being run from a dir with filecoin-network-viz
  if _, err := os.Stat(VizDir); err != nil {
    return fmt.Errorf("must be run from directory with %s\n", VizDir)
  }

  return runService(context.Background())
}

func main() {
  if err := run(); err != nil {
    fmt.Fprintf(os.Stderr, "error: %s\n", err)
    os.Exit(1)
  }
}
