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
	"text/template"
	"time"

	network "github.com/filecoin-project/filecoin-network-sim/network"
)

const (
	VizDir      = "./filecoin-network-viz"
	ExplorerDir = "./filecoin-explorer/build"
)

type Args struct {
	Debug   bool
	Port    int
	NetArgs network.Args
}

var argDefaults = Args{
	Debug: false,
	Port:  7002,
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

var Usage = `SYNOPSIS
	filnetsim - filecoin network simulator & visualizer

	filnetsim is a tool to simulate a filecoin network locally.
	It spawns go-filecoin daemons randomly, and issues commands to them via the cli tool.
	Then, the sim issues actions ramdomly according to the program options.
	The sim supports connecting to individual filecoin nodes to issue commands manually.
	The sim consumes all eventlogs and transforms them into a input logs for a visualization
	The sim serves webapp visualizations at an http server.
	In the future, this simulator may run across many machines.

ACTIONS
	SendPayment   sends a payment message, from one node to another (miner and client)
	StorageAsk    send a msg to add an Ask to the Storage Market (miner only)
	StorageBid    send a msg to add a Bid to the Storage Market (client only)
	Deal          matches one Ask and Bid, makes a Deal between their miner and client
	SendFiles     send deal file from client to miner (storage), or client to miner (retrieval)
	MineBlock     advance an epoch: sample leaders, each mine a block & propagate it (miners only)

OPTIONS
  NETWORK
	--max-nodes int            maximum number of nodes to spawn (default: {{.NetArgs.MaxNodes}})
	--start-nodes int          number of nodes to spawn at once in the beginning (default: {{.NetArgs.StartNodes}})

  TIME
	--t-join duration          how fast new nodes are spawned (default: {{.NetArgs.JoinTime}})
	--t-action duration        how fast to issue actions (default: {{.NetArgs.ActionTime}})
	--t-block duration         automatic mining block time (default: {{.NetArgs.BlockTime}})

  ACTIONS
	--auto-asks bool           automatically issue StorageAsk action (default: {{.NetArgs.Actions.Ask}})
	--auto-bids bool           automatically issue StorageBid action (default: {{.NetArgs.Actions.Bid}})
	--auto-deals bool          automatically issue StorageDeal action (default: {{.NetArgs.Actions.Deal}})
	--auto-mining bool         automatically mine blocks (default: {{.NetArgs.Actions.Mine}})
	--auto-payments bool       automatically issue StorageBid action (default: {{.NetArgs.Actions.Payment}})

  MINING
	--fork-branching int       number of leaders (branches) to consider per consensus epoch (default: {{.NetArgs.ForkBranching}})
	--fork-probability float   probability individual leaders mine a block (not power) (default: {{.NetArgs.ForkProbability}})

  FILES
	--test-files dir           directory with test files to use with SendFiles (default: {{.NetArgs.TestfilesDir}})

  OTHER
	-h, --help                 print this help text
	--debug                    output verbose debugging logs
	--port port                port at which to serve /logs and visualizations
	--httptest.serve addr      if non-empty, httptest.NewServer serves on this address and blocks
`

func parseArgs() Args {
	a := Args{}

	usageT, err := template.New("tmplusage").Parse(Usage)
	if err != nil {
		panic(err)
	}

	flag.Usage = func() {
		usageT.Execute(flag.CommandLine.Output(), argDefaults)
	}

	flag.BoolVar(&a.Debug, "debug", argDefaults.Debug, "")
	flag.IntVar(&a.Port, "port", argDefaults.Port, "")

	flag.DurationVar(&a.NetArgs.BlockTime, "t-block", argDefaults.NetArgs.BlockTime, "")
	flag.DurationVar(&a.NetArgs.ActionTime, "t-action", argDefaults.NetArgs.ActionTime, "")
	flag.DurationVar(&a.NetArgs.JoinTime, "t-join", argDefaults.NetArgs.JoinTime, "")
	flag.IntVar(&a.NetArgs.ForkBranching, "fork-branching", argDefaults.NetArgs.ForkBranching, "")
	flag.Float64Var(&a.NetArgs.ForkProbability, "fork-probability", argDefaults.NetArgs.ForkProbability, "")
	flag.IntVar(&a.NetArgs.MaxNodes, "max-nodes", argDefaults.NetArgs.MaxNodes, "")
	flag.IntVar(&a.NetArgs.StartNodes, "start-nodes", argDefaults.NetArgs.StartNodes, "")
	flag.StringVar(&a.NetArgs.TestfilesDir, "test-files", argDefaults.NetArgs.TestfilesDir, "")

	flag.BoolVar(&a.NetArgs.Actions.Ask, "auto-asks", argDefaults.NetArgs.Actions.Ask, "")
	flag.BoolVar(&a.NetArgs.Actions.Bid, "auto-bids", argDefaults.NetArgs.Actions.Bid, "")
	flag.BoolVar(&a.NetArgs.Actions.Deal, "auto-deals", argDefaults.NetArgs.Actions.Deal, "")
	flag.BoolVar(&a.NetArgs.Actions.Payment, "auto-payments", argDefaults.NetArgs.Actions.Payment, "")
	flag.BoolVar(&a.NetArgs.Actions.Mine, "auto-mining", argDefaults.NetArgs.Actions.Mine, "")

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
	addr := fmt.Sprintf("127.0.0.1:%d", args.Port)
	fmt.Printf("Logs at http://%s/logs\n", addr)
	fmt.Printf("Network Viz at http://%s/viz-circle\n", addr)
	fmt.Printf("Chain Viz at http://%s/viz-blockchain\n", addr)
	return http.ListenAndServe(addr, muxA)
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
