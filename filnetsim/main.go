package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	network "github.com/filecoin-project/filecoin-network-sim/network"
)

const (
	VizDir      = "./filecoin-network-viz/viz-circle"
	ExplorerDir = "./filecoin-explorer/build"
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
		TotalNodes: 25,
		BlockTime:  3 * time.Second,
		ActionTime: 1000 * time.Millisecond,
		Actions: []network.Action{
			network.ActionPayment,
			network.ActionAsk,
			network.ActionBid,
			network.ActionDeal,
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

	muxA := http.NewServeMux()
	muxB := http.NewServeMux()

	muxA.Handle("/", http.FileServer(http.Dir(VizDir)))
	muxA.HandleFunc("/logs", lh.HandleHttp)
	muxB.Handle("/", http.FileServer(http.Dir(ExplorerDir)))

	go func() {
		http.ListenAndServe(":7003", muxB)
	}()

	// run http
	fmt.Println("Listening at 127.0.0.1:7002/logs")
	return http.ListenAndServe(":7002", muxA)
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
