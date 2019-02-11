package network

import (
  "context"
  "fmt"
  "os"

  fast "github.com/filecoin-project/go-filecoin/tools/fast"
  iptbfilplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
  iptb "github.com/ipfs/iptb/testbed"
)

const (
  kittyhawkGenesis = "http://user.kittyhawk.wtf:8020/genesis.car"
)

func init() {
  _, err := iptb.RegisterPlugin(iptb.IptbPlugin{
    From:       "<builtin>",
    NewNode:    iptbfilplugin.NewNode,
    PluginName: iptbfilplugin.PluginName,
    BuiltIn:    true,
  }, false)

  if err != nil {
    panic(err)
  }
}

func NewFastFilecoinProc(repoDir string) (*fast.Filecoin, error) {
  ctx := context.TODO()

  // based on github.com/filecoin-project/go-filecoin/tools/fast.EnvironmentMemoryGenesis.NewProcess
  ns := IPTBNodeSpec(repoDir)

  if err := os.MkdirAll(ns.Dir, 0775); err != nil {
    return nil, err
  }

  c, err := ns.Load()
  if err != nil {
    return nil, err
  }

  // We require a slightly more extended core interface
  fc, ok := c.(fast.IPTBCoreExt)
  if !ok {
    return nil, fmt.Errorf("%s does not implement the extended IPTB.Core interface IPTBCoreExt", ns.Type)
  }

  p := fast.NewFilecoinProcess(ctx, fc, FastEnvOpts())
  // todo: init, and register w/ aggregator.
  return p, nil
}

func IPTBNodeSpec(repoDir string) iptb.NodeSpec {
  nodeSpecOpts := map[string]string{}

  return iptb.NodeSpec{
    Type:  iptbfilplugin.PluginName,
    Dir:   repoDir,
    Attrs: nodeSpecOpts,
  }
}

func FastEnvOpts() fast.EnvironmentOpts {
  eo := fast.EnvironmentOpts{}
  eo.InitOpts = append(eo.InitOpts, fast.POGenesisFile(kittyhawkGenesis))
  eo.InitOpts = append(eo.InitOpts, PODevnetUser())
  return eo
}

// PODevnetUser provides the `--devnet-user` option to process at init
func PODevnetUser() fast.ProcessInitOption {
  return func() []string {
    return []string{"--devnet-user"}
  }
}
