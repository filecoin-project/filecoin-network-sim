package network

import (
  "sync"
  "fmt"
  "path/filepath"
  "math/rand"

  daemon "github.com/filecoin-project/filnetsim/daemon"
  logs "github.com/filecoin-project/filnetsim/logs"
)


// daemon + cached info
type Node struct {
  *daemon.Daemon

  ID string
  sl *logs.SimLogger
}

func (n *Node) Logs() *logs.SimLogger {
  if n.sl == nil {
    r := n.Daemon.EventLogStream()
    n.sl = logs.NewSimLogger(n.ID, r)
  }
  return n.sl
}

type Network struct {
  lk      sync.RWMutex
  nodes   []*Node
  repoNum int
  repoDir string
  logs    *logs.LineAggregator
}

func NewNetwork(repoDir string) *Network {
  la := logs.NewLineAggregator()
  return &Network{repoDir: repoDir, logs: la}
}

func (n *Network) Size() int {
  n.lk.Lock()
  defer n.lk.Unlock()
  return len(n.nodes)
}

func (n *Network) Logs() *logs.LineAggregator {
  return n.logs
}

func (n *Network) AddNode() (*Node, error) {
  n.lk.Lock()
  repoNum := n.repoNum
  n.repoNum++
  n.lk.Unlock()

  d, err := daemon.NewDaemon(
    daemon.RepoDir(filepath.Join(n.repoDir, fmt.Sprintf("node%d", repoNum))),
    daemon.ShouldInit(true))
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

  // Connect?

  node := &Node{
    Daemon: d,
    ID:     id,
  }

  n.lk.Lock()
  n.nodes = append(n.nodes, node)
  n.lk.Unlock()

  n.logs.MixReader(node.Logs().Reader())

  addr, err := d.GetMainWalletAddress()
  if err == nil {
    node.sl.WriteEvent(logs.NetworkChurnEvent(addr, "Miner", true))
  }

  fmt.Println("added a new node to the network:", node.ID)
  return node, nil
}

func (n *Network) AddNodes(num int) error {
  errs := AsyncErrs(num, func(i int) error {
    _, err := n.AddNode()
    return err
  })

  if len(errs) > 0 {
    return fmt.Errorf("adding %d/%d failed", len(errs), num)
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
      failed++
    }
  }

  if failed > 0 {
    return fmt.Errorf("failed to connect %d/%d", failed, len(conn))
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

func (n *Network) GetRandomNode() *Node {
  n.lk.Lock()
  defer n.lk.Unlock()

  l := len(n.nodes)
  if l == 0 {
    return nil
  }

  return n.nodes[rand.Intn(l)]
}

func (n *Network) GetRandomNodes(num int) []*Node {
  n.lk.Lock()
  defer n.lk.Unlock()

  if len(n.nodes) < num {
    return nil
  }

  // use a set to sample different nodes.
  nodeSet := map[*Node]struct{}{}
  for len(nodeSet) < num {
    nd := n.nodes[rand.Intn(len(n.nodes))]
    nodeSet[nd] = struct{}{}
  }

  var nodeList []*Node
  for k, _ := range nodeSet {
    nodeList = append(nodeList, k)
  }

  return nodeList
}

func (n *Network) ShutdownAll() error {
  n.lk.Lock()
  defer n.lk.Unlock()

  errs := AsyncErrs(len(n.nodes), func(i int) error {
    return n.nodes[i].Shutdown()
  })

  var err error
  if len(errs) > 0 {
    err = fmt.Errorf("shutting down %d/%d failed", len(errs), len(n.nodes))
  }
  return err
}
