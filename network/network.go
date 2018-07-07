package network

import (
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"sync"
	"os"
	"text/template"

	logs "github.com/filecoin-project/filecoin-network-sim/logs"
	daemon "github.com/filecoin-project/go-filecoin/testhelpers"
)

type NodeType string

const (
	MinerNodeType  NodeType = "Miner"
	ClientNodeType NodeType = "Client"
	AnyNodeType    NodeType = "Node"
)

var tmplNodeAdded *template.Template

type tmplNodeAddedData struct {
	WalletAddr string
	MinerAddr  string
	SwarmAddr  string
	ApiAddr    string
	Type       NodeType
}

func init() {
	var err error
	tmplNodeAdded, err = template.New("tmplnodeadded").Parse(`-> Created New Node: {{.Type}}
	main wallet address: {{.WalletAddr}}
	miner actor address: {{.MinerAddr}}
	go-filecoin swarm connect {{.SwarmAddr}}
	go-filecoin --cmdapiaddr={{.ApiAddr}}
`)
	if err != nil {
		panic(err)
	}
}

func RandomNodeType() NodeType {
	switch rand.Intn(2) {
	case 1:
		return MinerNodeType
	default:
		return ClientNodeType
	}
}

// daemon + cached info
type Node struct {
	*daemon.Daemon

	Type       NodeType
	ID         string
	WalletAddr string // ClientAddr
	MinerAddr  string
	SwarmAddr  string
	sl         *logs.SimLogger
}

func NewNode(d *daemon.Daemon, id string, t NodeType) (*Node, error) {
	if t == AnyNodeType {
		t = RandomNodeType()
	}

	addr, err := d.GetMainWalletAddress()
	if err != nil {
		return nil, err
	}

	saddr, err := d.GetAddress()
	if err != nil {
		return nil, err
	}

	n := &Node{
		Daemon:     d,
		ID:         id,
		Type:       t,
		WalletAddr: addr,
		MinerAddr:  "",
		SwarmAddr:  saddr,
	}

	return n, nil
}

func (n *Node) Logs() *logs.SimLogger {
	if n.sl == nil {
		r := n.Daemon.EventLogStream()
		n.sl = logs.NewSimLogger(n.ID, r)
	}
	return n.sl
}

func (n *Node) HasMinerIdentity() bool {
	return n.MinerAddr == ""
}

func (n *Node) CreateOrGetMinerIdentity() (string, error) {
	if n.MinerAddr == "" {
		a, err := n.CreateMinerAddr()
		if err != nil {
			return "", err
		}
		n.MinerAddr = a.String()
	}
	return n.MinerAddr, nil
}

func (n *Node) GetMinerIdentity() string {
	return n.MinerAddr
}

func (n *Node) MatchesType(t NodeType) bool {
	return t == AnyNodeType || n.Type == t
}

type Network struct {
	lk      sync.RWMutex
	nodes   []*Node
	repoNum int
	repoDir string
	logs    *logs.LineAggregator
}

func NewNetwork(repoDir string) (*Network, error) {
	la := logs.NewLineAggregator()

	if _, err := daemon.GetFilecoinBinary(); err != nil {
		return nil, err
	}
	return &Network{repoDir: repoDir, logs: la}, nil
}

func (n *Network) Size() int {
	n.lk.Lock()
	defer n.lk.Unlock()
	return len(n.nodes)
}

func (n *Network) Logs() *logs.LineAggregator {
	return n.logs
}

func (n *Network) tryCreatingNode(t NodeType) (*Node, error) {
	n.lk.Lock()
	repoNum := n.repoNum
	n.repoNum++
	n.lk.Unlock() // unlock to be able to set up the node w/o holding lock.

	d, err := daemon.NewDaemon(
		daemon.RepoDir(filepath.Join(n.repoDir, fmt.Sprintf("node%d", repoNum))),
		daemon.ShouldInit(true),
		daemon.InsecureApi(),
	)

	if err != nil {
		return nil, err
	}

	if _, err := d.Start(); err != nil {
		d.Shutdown()
		return nil, err
	}

	id, err := d.GetID()
	if err != nil {
		d.Shutdown()
		return nil, err
	}

	node, err := NewNode(d, id, t)
	if err != nil {
		d.Shutdown()
		return nil, err
	}

	return node, nil
}

func (n *Network) AddNode(t NodeType) (*Node, error) {
	node, err := n.tryCreatingNode(t)
	if err != nil {
		return nil, err
	}
	// ok from here, we have a node, and it should work out.

	// connect to other miners?
	// TODO
	n.ConnectNodeToAll(node)

	// frrist: we want realistic sim. lots of actions gated by 1-at-atime consesnus
	node.Daemon.SetWaitMining(false)

	// add miner to our list.
	n.lk.Lock()
	n.nodes = append(n.nodes, node)
	n.lk.Unlock()

	n.logs.MixReader(node.Logs().Reader())

	// announce the miner to logs
	eventMap := logs.NetworkChurnEvent(node.WalletAddr, string(node.Type), true)
	eventMap["cmdAddr"] = node.CmdAddr

	node.Logs().WriteEvent(eventMap)

	// need some $ ...
	if err := node.MiningOnce(); err != nil {
		return nil, err
	}
	if node.Type == MinerNodeType {
		node.CreateOrGetMinerIdentity() // sets n.MinerAddr
	}

	tmplNodeAdded.Execute(os.Stdout, tmplNodeAddedData{
		WalletAddr: node.WalletAddr,
		MinerAddr: node.MinerAddr,
		SwarmAddr: node.SwarmAddr,
		ApiAddr: node.Daemon.CmdAddr,
		Type: node.Type,
	})

	log.Printf("[NET]\t added a new node to the network: %s Address: %s\n", node.ID, node.WalletAddr)
	return node, nil
}

func (n *Network) AddNodes(t NodeType, num int) error {
	errs := AsyncErrs(num, func(i int) error {
		_, err := n.AddNode(t)
		return err
	})

	if len(errs) > 0 {
		return fmt.Errorf("[NET]\t adding %d/%d failed\n", len(errs), num)
	}
	return nil
}

func (n *Network) ConnectNodeToAll(node *Node) error {
	n.lk.Lock()
	conn := make([]*Node, len(n.nodes))
	for i, n2 := range n.nodes {
		if n2 != node {
			conn[i] = n2
		}
	}
	n.lk.Unlock()

	failed := 0
	for _, n2 := range conn {
		_, err := node.Connect(n2.Daemon)
		logErr(err)
		if err != nil {
			panic(err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("[NET]\t failed to connect %d/%d\n", failed, len(conn))
	}
	return nil
}

func (n *Network) GetNode(index int) *Node {
	n.lk.Lock()
	defer n.lk.Unlock()

	if index >= len(n.nodes) {
		return nil
	}

	return n.nodes[index]
}

func (n *Network) GetNodeByID(id string) *Node {
	n.lk.Lock()
	defer n.lk.Unlock()

	for _, nd := range n.nodes {
		if nd.ID == id {
			return nd
		}
	}
	return nil
}

// should be called with lock held
func (n *Network) GetNodesOfType(t NodeType) []*Node {
	n.lk.Lock()
	defer n.lk.Unlock()

	var nodes []*Node
	for _, node := range n.nodes {
		if node.MatchesType(t) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *Network) GetNodeCounts() map[NodeType]int {
	n.lk.Lock()
	defer n.lk.Unlock()

	m := make(map[NodeType]int)
	for _, node := range n.nodes {
		m[node.Type] = m[node.Type] + 1
	}
	return m
}

func (n *Network) GetRandomNode(t NodeType) *Node {
	nodes := n.GetNodesOfType(t)

	l := len(nodes)
	if l == 0 {
		return nil
	}

	return nodes[rand.Intn(l)]
}

func (n *Network) GetRandomNodes(t NodeType, num int) []*Node {
	nodes := n.GetNodesOfType(t)
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) < num {
		num = len(nodes)
	}

	// shuffle first, then output.
	rand.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
	return nodes[:num]
}

func (n *Network) ShutdownAll() error {
	n.lk.Lock()
	defer n.lk.Unlock()

	errs := AsyncErrs(len(n.nodes), func(i int) error {
		return n.nodes[i].Shutdown()
	})

	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("[NET]\t shutting down %d/%d failed\n", len(errs), len(n.nodes))
	}
	return err
}
