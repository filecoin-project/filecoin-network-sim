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
	VizDir      = "./filecoin-network-viz"
	ExplorerDir = "./filecoin-explorer/build"
)

type Args struct {
	Debug   bool
	NetArgs network.Args
}

var argDefaults = Args{
	Debug: false,
	NetArgs: network.Args{
		StartNodes:      3,
		MaxNodes:        15,
		JoinTime:        3 * time.Second * 4, // 4x the block time
		BlockTime:       3 * time.Second,
		ActionTime:      300 * time.Millisecond,
		ForkBranching:   1,
		ForkProbability: 1.0,
		TestfilesDir:    "testfiles",
		Actions: network.ActionArgs{
			Ask:     true,
			Bid:     true,
			Deal:    true,
			Payment: true,
			Mine:    true,
		},
	},
}

func parseArgs() Args {
	a := Args{}
	flag.BoolVar(&a.Debug, "debug", argDefaults.Debug, "turns on debug logging")
	flag.DurationVar(&a.NetArgs.BlockTime, "t-block", argDefaults.NetArgs.BlockTime, "block time")
	flag.DurationVar(&a.NetArgs.ActionTime, "t-action", argDefaults.NetArgs.ActionTime, "how fast actions are taken")
	flag.DurationVar(&a.NetArgs.JoinTime, "t-join", argDefaults.NetArgs.JoinTime, "how fast new nodes are added")
	flag.IntVar(&a.NetArgs.ForkBranching, "fork-branching", argDefaults.NetArgs.ForkBranching, "miners sampling per round")
	flag.Float64Var(&a.NetArgs.ForkProbability, "fork-probability", argDefaults.NetArgs.ForkProbability, "miners sampling probability (-1 = 1/n)")
	flag.IntVar(&a.NetArgs.MaxNodes, "max-nodes", argDefaults.NetArgs.MaxNodes, "max number of nodes")
	flag.IntVar(&a.NetArgs.StartNodes, "start-nodes", argDefaults.NetArgs.StartNodes, "starting number of nodes")
	flag.StringVar(&a.NetArgs.TestfilesDir, "test-files", argDefaults.NetArgs.TestfilesDir, "directory with test files")

	flag.BoolVar(&a.NetArgs.Actions.Ask, "auto-asks", argDefaults.NetArgs.Actions.Ask, "whether to auto generate asks")
	flag.BoolVar(&a.NetArgs.Actions.Bid, "auto-bids", argDefaults.NetArgs.Actions.Bid, "whether to auto generate bids")
	flag.BoolVar(&a.NetArgs.Actions.Deal, "auto-deals", argDefaults.NetArgs.Actions.Deal, "whether to auto generate deals")
	flag.BoolVar(&a.NetArgs.Actions.Payment, "auto-payments", argDefaults.NetArgs.Actions.Payment, "whether to auto generate payments")
	flag.BoolVar(&a.NetArgs.Actions.Mine, "auto-mining", argDefaults.NetArgs.Actions.Mine, "whether to auto ")

	flag.Parse()

	return a
}

type Instance struct {
	N *network.Network
	R *network.Randomizer
	L io.Reader
}

func SetupInstance(args Args) (*Instance, error) {
	dir, err := ioutil.TempDir("", "filnetsim")
	if err != nil {
		dir = "/tmp/filnetsim"
	}

	n, err := network.NewNetwork(dir)
	if err != nil {
		return nil, err
	}

	r := network.NewRandomizer(n, args.NetArgs)
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

func runService(ctx context.Context, args Args) error {
	i, err := SetupInstance(args)
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
	fmt.Println("Logs at http://127.0.0.1:7002/logs")
	fmt.Println("Network Viz at http://127.0.0.1:7002/viz-circle")
	fmt.Println("Chain Viz at http://127.0.0.1:7002/viz-blockchain")
	return http.ListenAndServe(":7002", muxA)
}

func run(args Args) error {

	// handle options
	if args.Debug {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	// check we're being run from a dir with filecoin-network-viz
	if _, err := os.Stat(VizDir); err != nil {
		return fmt.Errorf("must be run from directory with %s\n", VizDir)
	}

	return runService(context.Background(), args)
}

func main() {
	if err := run(parseArgs()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
