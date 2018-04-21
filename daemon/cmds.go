package daemon

import (
  "encoding/json"
  "errors"
  "strings"
  "fmt"

  "github.com/filecoin-project/go-filecoin/types"

  "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func (td *Daemon) GetID() (string, error) {
  out, err := td.Run("id")
  if err != nil {
    return "", err
  }


  var parsed map[string]interface{}
  err = json.Unmarshal([]byte(out.ReadStdout()), &parsed)
  if err != nil {
    return "", err
  }

  s, ok := parsed["ID"].(string)
  if !ok {
    return "", errors.New("id format incorrect")
  }
  return s, nil
}

func (td *Daemon) GetAddress() (string, error) {
  out, err := td.Run("id")
  if err != nil {
    return "", err
  }

  var parsed map[string]interface{}
  err = json.Unmarshal([]byte(out.ReadStdout()), &parsed)
  if err != nil {
    return "", err
  }

  adders, ok := parsed["Addresses"].([]interface{})
  if !ok {
    return "", errors.New("address format incorrect")
  }

  s, ok := adders[0].(string)
  if !ok {
    return "", errors.New("address format incorrect")
  }
  return s, nil
}

func (td *Daemon) Connect(remote *Daemon) (*Output, error) {
  // Connect the nodes
  addr, err := remote.GetAddress()
  if err != nil {
    return nil, err
  }

  out, err := td.Run("swarm", "connect", addr)
  if err != nil {
    return out, err
  }
  peers1, err := td.Run("swarm", "peers")
  if err != nil {
    return out, err
  }
  peers2, err := remote.Run("swarm", "peers")
  if err != nil {
    return out, err
  }

  rid, err := remote.GetID()
  if err != nil {
    return out, err
  }
  lid, err := td.GetID()
  if err != nil {
    return out, err
  }

  if !strings.Contains(peers1.ReadStdout(), rid) {
    return out, errors.New("failed to connect (2->1)")
  }
  if !strings.Contains(peers2.ReadStdout(), lid) {
    return out, errors.New("failed to connect (1->2)")
  }

  return out, nil
}

func (td *Daemon) MiningOnce() error {
  _, err := td.Run("mining", "once")
  return err
}

// CreateMinerAddr issues a new message to the network, mines the message
// and returns the address of the new miner
// equivalent to:
//     `go-filecoin miner create --from $TEST_ACCOUNT 100000 20`
func (td *Daemon) CreateMinerAddr() (types.Address, error) {
  var minerAddr types.Address

  // need money
  if err := td.MiningOnce(); err != nil {
    return minerAddr, err
  }

  miner, err := td.Run("miner", "create", "1000000", "1000")
  if err != nil {
    return minerAddr, err
  }

  minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
  if err != nil {
    return minerAddr, err
  }

  wait, err := td.MineForMessage(minerMessageCid.String())
  if err != nil {
    return minerAddr, err
  }

  addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
  if err != nil {
    return addr, err
  }

  emptyAddr := types.Address{}
  if emptyAddr == addr {
    return addr, errors.New("got empty address")
  }

  return addr, nil
}

func (td *Daemon) MineForMessage(msg string) (*Output, error) {

  var outErr error
  var out *Output

  wait := make(chan struct{})
  go func() {
    out, outErr = td.WaitForMessage(msg)
    close(wait)
  }()

  _, mineErr := td.Run("mining", "once")

  <-wait

  if mineErr != nil {
    return out, mineErr
  }
  return out, outErr
}

func (td *Daemon) WaitForMessage(msg string) (*Output, error) {
  return td.Run("message", "wait",
    "--return",
    "--message=false",
    "--receipt=false",
    msg,
  )
}

// CreateWalletAddr adds a new address to the daemons wallet and
// returns it.
// equivalent to:
//     `go-filecoin wallet addrs new`
func (td *Daemon) CreateWalletAddr() (string, error) {
  outNew, err := td.Run("wallet", "addrs", "new")
  if err != nil {
    return "", err
  }

  addr := strings.Trim(outNew.ReadStdout(), "\n")
  if addr == "" {
    return "", errors.New("got empty address")
  }
  return addr, nil
}


func (td *Daemon) GetMainWalletAddress() (string, error) {
  out, err := td.Run("address", "ls")
  if err != nil {
    return "", err
  }

  var o struct {
    Address string
  }

  err = json.Unmarshal([]byte(out.ReadStdout()), &o)
  if err != nil {
    return "", err
  }
  if o.Address == "" {
    return "", errors.New("output format incorrect")
  }
  return o.Address, nil
}

func (td *Daemon) SendFilecoin(from, to string, amt int) error {
  out, err := td.Run("message", "send",
    fmt.Sprintf("--value=%d", amt),
    fmt.Sprintf("--from=%s", from),
    to)
  if err != nil {
    return err
  }

  cid, err := cid.Parse(strings.Trim(out.ReadStdout(), "\n"))
  if err != nil {
    return err
  }

  _, err = td.MineForMessage(cid.String())
  return err
}
