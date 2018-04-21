package daemon

import (
  "context"
  "encoding/json"
  "errors"
  "strings"
  "fmt"
  "log"
  "strconv"

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
  if td.WaitMining() { // if disabled, skip (for realistic network sim)
    if err := td.MiningOnce(); err != nil {
      return minerAddr, err
    }
  }

  miner, err := td.Run("miner", "create", "1000000", "1000")
  if err != nil {
    return minerAddr, err
  }

  minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
  if err != nil {
    return minerAddr, err
  }

  wait, err := td.MineForMessage(context.TODO(), minerMessageCid.String())
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

func (td *Daemon) MineForMessage(ctx context.Context, msg string) (*Output, error) {

  log.Print("message wait: mining for message ", msg)
  var outErr error
  var out *Output

  wait := make(chan struct{})
  go func() {
    out, outErr = td.WaitForMessage(ctx, msg)
    log.Print("message wait: mined message ", msg)
    close(wait)
  }()

  var mineErr error
  if td.WaitMining() { // if disabled, skip (for realistic network sim)
    mineErr = td.MiningOnce()
  }

  <-wait

  if mineErr != nil {
    return out, mineErr
  }
  return out, outErr
}

func (td *Daemon) WaitForMessage(ctx context.Context, msg string) (out *Output, err error) {
  log.Print("message wait: waiting for message ", msg)

  // do it async to allow "canceling out" via context.
  done := make(chan struct{})

  go func() {
    // sets the return vars
    out, err = td.Run("message", "wait",
      "--return",
      "--message=false",
      "--receipt=false",
      msg,
    )
    close(done)
  }()

  select {
  case <-ctx.Done():
    return nil, ctx.Err()
  case <-done:
    return out, err
  }
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

  addr := strings.Trim(out.ReadStdout(), "\n ")
  return addr, nil
}

func (td *Daemon) SendFilecoin(ctx context.Context, from, to string, amt int) error {
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

  _, err = td.MineForMessage(ctx, cid.String())
  return err
}

func (td *Daemon) WalletBalance(addr string) (int, error) {
  out, err := td.Run("wallet", "balance", addr)
  if err != nil {
    return 0, err
  }

  balance, err := strconv.Atoi(strings.Trim(out.ReadStdout(), "\n"))
  if err != nil {
    return balance, err
  }
  return balance, err
}

func (td *Daemon) MinerAddAsk(ctx context.Context, from string, size, price int) error {
  out, err := td.Run("miner", "add-ask", from,
    strconv.Itoa(size), strconv.Itoa(price))
  if err != nil {
    return err
  }

  cid, err := cid.Parse(strings.Trim(out.ReadStdout(), "\n"))
  if err != nil {
    return err
  }

  _, err = td.MineForMessage(ctx, cid.String())
  return err
}
