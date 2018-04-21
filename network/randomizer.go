package network

import (
  "context"
  "time"
  "math/rand"
)

type Action int

const (
  ActionPayment Action = iota
  ActionAsk
  ActionBid
  ActionDeal
  ActionSendFile
)


type Randomizer struct {
  Net *Network

  TotalNodes int
  BlockTime  time.Duration
  ActionTime time.Duration
  Actions    []Action
}

func (r *Randomizer) Run(ctx context.Context) {
  go r.addAndRemoveMiners(ctx)
  go r.mineBlocks(ctx)
  go r.randomActions(ctx)
}

func (r *Randomizer) periodic(ctx context.Context, t time.Duration, periodicFunc func()) {
  for {
    time.Sleep(t)

    select {
    case <-ctx.Done():
      return
    default:
    }

    periodicFunc()
  }
}

func (r *Randomizer) addAndRemoveMiners(ctx context.Context) {
  r.periodic(ctx, 2 * r.BlockTime, func() {
    size := r.Net.Size()
    if size < r.TotalNodes {
      _, err := r.Net.AddNode()
      logErr(err)
    }
  })
}

func (r *Randomizer) mineBlocks(ctx context.Context) {
  r.periodic(ctx, r.BlockTime, func() {
    n := r.Net.GetRandomNode()
    if n == nil {
      return
    }
    logErr(n.Daemon.MiningOnce())
  })
}

func (r *Randomizer) randomActions(ctx context.Context) {
  r.periodic(ctx, r.ActionTime, func() {

    nds := r.Net.GetRandomNodes(2)
    if len(nds) < 2 || nds[0] == nil {
      return
    }

    switch r.Actions[rand.Intn(len(r.Actions))] {
    case ActionPayment:
      if nds[1] == nil {
        return
      }
      a1, err := nds[0].Daemon.GetMainWalletAddress()
      logErr(err)
      a2, err := nds[1].Daemon.GetMainWalletAddress()
      logErr(err)
      if a1 == "" || a2 == "" {
        return
      }

      logErr(nds[0].Daemon.SendFilecoin(a1, a2, 5))
      return

    case ActionAsk:
    case ActionBid:
    case ActionDeal:
    case ActionSendFile:
    }
  })
}

func logErr(err error) {
  if err != nil {
    // log.Error(err)
  }
}
