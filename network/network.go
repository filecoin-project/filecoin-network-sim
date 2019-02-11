package network

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"text/template"

	logs "github.com/filecoin-project/filecoin-network-sim/logs"
	address "github.com/filecoin-project/go-filecoin/address"
	filapi "github.com/filecoin-project/go-filecoin/api"
	fast "github.com/filecoin-project/go-filecoin/tools/fast"
	fastseries "github.com/filecoin-project/go-filecoin/tools/fast/series"
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
	RepoDir    string
	Type       NodeType
}

func init() {
	var err error
	tmplNodeAdded, err = template.New("tmplnodeadded").Parse(`-> Created New Node: {{.Type}}
	repo dir: {{.RepoDir}}
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
	Daemon *fast.Filecoin

	Type       NodeType
	RepoDir    string
	IDDetails  *filapi.IDDetails
	WalletAddr address.Address // ClientAddr
	MinerAddr  address.Address
	ApiAddr    string
	sl         *logs.SimLogger
}

func NewNode(d *fast.Filecoin, repoDir string, idDetails *filapi.IDDetails, t NodeType) (*Node, error) {
	if t == AnyNodeType {
		t = RandomNodeType()
	}

	addr, err := FilecoinGetMainWalletAddress(context.TODO(), d)
	if err != nil {
		return nil, err
	}

	apiAddr, err := d.APIAddr()
	if err != nil {
		return nil, err
	}

	n := &Node{
		Daemon:     d,
		RepoDir:    repoDir,
		IDDetails:  idDetails,
		Type:       t,
		WalletAddr: addr,
		MinerAddr:  address.Address{},
		ApiAddr:    apiAddr,
	}

	return n, nil
}

func (n *Node) Logs() *logs.SimLogger {
	if n.sl == nil {
		r, err := FilecoinLogTail(context.TODO(), n.Daemon)
		if err != nil {
			panic(err)
		}
		n.sl = logs.NewSimLogger(n.IDDetails.ID.String(), r)
	}
	return n.sl
}

func (n *Node) HasMinerIdentity() bool {
	return n.MinerAddr == address.Address{}
}

func (n *Node) CreateOrGetMinerIdentity() (address.Address, error) {
	if (n.MinerAddr == address.Address{}) {
		a, err := n.CreateMinerAddr()
		if err != nil {
			return a, err
		}
		n.MinerAddr = a
	}
	return n.MinerAddr, nil
}

func (n *Node) CreateMinerAddr() (address.Address, error) {
	return FilecoinCreateMinerAddr(context.TODO(), n)
}

func (n *Node) GetMinerIdentity() string {
	return n.MinerAddr.String()
}

func (n *Node) MatchesType(t NodeType) bool {
	return t == AnyNodeType || n.Type == t
}

func (n *Node) Connect(n2 *Node) error {
	return fastseries.Connect(context.TODO(), n.Daemon, n2.Daemon)
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

	// likely do not need this anymore. (TODO)
	// if _, err := daemon.GetFilecoinBinary(); err != nil {
	// 	return nil, err
	// }

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
	ctx := context.TODO()

	n.lk.Lock()
	repoNum := n.repoNum
	n.repoNum++
	n.lk.Unlock() // unlock to be able to set up the node w/o holding lock.

	repoDir := filepath.Join(n.repoDir, fmt.Sprintf("node%d", repoNum))
	d, err := NewFastFilecoinProc(repoDir)
	if err != nil {
		return nil, err
	}

	wait := true
	if _, err := d.StartDaemon(context.TODO(), wait); err != nil {
		d.StopDaemon(ctx)
		return nil, err
	}

	idDetails, err := d.ID(ctx)
	if err != nil {
		d.StopDaemon(ctx)
		return nil, err
	}

	node, err := NewNode(d, repoDir, idDetails, t)
	if err != nil {
		d.StopDaemon(ctx)
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
	n.ConnectNodeToAll(node)

	// frrist: we want realistic sim. lots of actions gated by 1-at-atime consesnus
	// node.Daemon.SetWaitMining(false)

	// add miner to our list.
	n.lk.Lock()
	n.nodes = append(n.nodes, node)
	n.lk.Unlock()

	n.logs.MixReader(node.Logs().Reader())

	// announce the miner to logs
	eventMap := logs.NetworkChurnEvent(node.WalletAddr.String(), string(node.Type), true)
	eventMap["cmdAddr"] = node.ApiAddr

	node.Logs().WriteEvent(eventMap)

	if node.Type == MinerNodeType {
		node.CreateOrGetMinerIdentity() // sets n.MinerAddr
	}

	tmplNodeAdded.Execute(os.Stdout, tmplNodeAddedData{
		WalletAddr: node.WalletAddr.String(),
		MinerAddr:  node.MinerAddr.String(),
		SwarmAddr:  node.IDDetails.Addresses[0].String(),
		ApiAddr:    node.ApiAddr,
		RepoDir:    node.RepoDir,
		Type:       node.Type,
	})

	log.Printf("[NET]\t added a new node to the network: %s Address: %s\n", node.IDDetails.ID, node.WalletAddr)
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
		if n2 == nil {
			continue // one of them will be nil
		}

		err := node.Connect(n2)
		if err != nil {
			logErr(err)
			failed++
			return err
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
		if nd.IDDetails.ID.String() == id {
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
		return n.nodes[i].Daemon.StopDaemon(context.TODO())
	})

	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("[NET]\t shutting down %d/%d failed\n", len(errs), len(n.nodes))
	}
	return err
}
