package network

import (
  "context"
  "errors"
  "fmt"
  "io"
  "io/ioutil"
  "net/http"

  address "github.com/filecoin-project/go-filecoin/address"
  commands "github.com/filecoin-project/go-filecoin/commands"
  fast "github.com/filecoin-project/go-filecoin/tools/fast"
  types "github.com/filecoin-project/go-filecoin/types"
  cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
)

const (
  userDevnetFaucetUrl = "https://user.kittyhawk.wtf:9797/tap?target=%s"
  faucetSuccessFmt    = "Success! Message CID: %s"
)

func FilecoinGetMainWalletAddress(ctx context.Context, f *fast.Filecoin) (address.Address, error) {
  as, err := f.AddressLs(ctx)
  if err != nil {
    return address.Address{}, err
  }

  if len(as) < 1 {
    return address.Address{}, errors.New("no addresses")
  }

  return as[0], nil
}

// ID runs the `id` command against the filecoin process
func FilecoinLogTail(ctx context.Context, f *fast.Filecoin) (io.ReadCloser, error) {
  args := []string{"go-filecoin", "log", "tail"}
  out, err := f.RunCmdWithStdin(ctx, nil, args...)
  if err != nil {
    return nil, err
  }
  return out.Stdout(), nil
}

// CreateMinerAddr issues a new message to the network, mines the message
// and returns the address of the new miner
// equivalent to:
//     `go-filecoin miner create --from $TEST_ACCOUNT 100000 20`
// TODO don't panic be happy
func FilecoinCreateMinerAddr(ctx context.Context, n *Node) (address.Address, error) {
  collateralAmt := 1
  pledgeAmt := 10
  totalAmt := collateralAmt + pledgeAmt
  na := address.Address{}

  err := BalanceGreaterEqualWithFaucet(n.Daemon, n.WalletAddr, totalAmt)
  if err != nil {
    return na, err
  }

  return n.Daemon.MinerCreate(ctx, uint64(pledgeAmt), types.NewAttoFILFromFIL(uint64(collateralAmt)).BigInt())
}

func GetFilFromFaucet(addr address.Address, faucetUrl string) (msg cid.Cid, err error) {
  var c cid.Cid

  url := fmt.Sprintf(userDevnetFaucetUrl, addr.String())
  res, err := http.Get(url)
  if err != nil {
    return c, err
  }

  defer res.Body.Close()
  bodyB, err := ioutil.ReadAll(res.Body)
  if err != nil {
    return c, err
  }

  var cs string
  _, err = fmt.Sscanf(string(bodyB), cs)
  if err != nil {
    return c, err
  }

  return cid.Decode(cs)
}

func WaitForMsg(f *fast.Filecoin, msg cid.Cid) (commands.WaitResult, error) {
  return f.MessageWait(context.TODO(), msg)
}

func BalanceGreaterEqual(f *fast.Filecoin, addr address.Address, amt int) (bool, error) {
  balance, err := f.WalletBalance(context.TODO(), addr)
  if err != nil {
    return false, err
  }

  amtaf := types.NewAttoFILFromFIL(uint64(amt))
  return balance.GreaterEqual(amtaf), nil
}

func BalanceGreaterEqualWithFaucet(f *fast.Filecoin, addr address.Address, amt int) error {
  ok, err := BalanceGreaterEqual(f, addr, amt)
  if err != nil {
    return err
  }
  if ok {
    return nil // we're all set.
  }

  // otherwise, try the faucet.
  cid, err := GetFilFromFaucet(addr, userDevnetFaucetUrl)
  if err != nil {
    return err
  }

  _, err = WaitForMsg(f, cid)
  if err != nil {
    return err
  }

  ok, err = BalanceGreaterEqual(f, addr, amt)
  if err != nil {
    return err
  }
  if ok {
    return nil // we're all set.
  }
  return errors.New("not enough money")
}

func SendFilecoin(ctx context.Context, f *fast.Filecoin, from, to address.Address, amt int) error {
  _, err := f.MessageSend(ctx, to, "", fast.AOFromAddr(from), fast.AOValue(amt))
  return err
}
