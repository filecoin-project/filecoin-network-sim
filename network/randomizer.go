package network

import (
  "context"
  "time"
  "math/rand"
  "log"
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

func (r *Randomizer) periodic(ctx context.Context, t time.Duration, periodicFunc func(ctx context.Context)) {
  for {
    time.Sleep(t)

    select {
    case <-ctx.Done():
      return
    default:
    }

    periodicFunc(ctx)
  }
}

func (r *Randomizer) addAndRemoveMiners(ctx context.Context) {
  r.periodic(ctx, 2 * r.BlockTime, func(ctx context.Context) {
    size := r.Net.Size()
    if size < r.TotalNodes {
      _, err := r.Net.AddNode()
      logErr(err)
    }
  })
}

func (r *Randomizer) mineBlocks(ctx context.Context) {
  r.periodic(ctx, r.BlockTime, func(ctx context.Context) {
    n := r.Net.GetRandomNode()
    if n == nil {
      return
    }
    go func() {
      logErr(n.Daemon.MiningOnce())
    }()
  })
}

func (r *Randomizer) randomActions(ctx context.Context) {
  r.periodic(ctx, r.ActionTime, func(ctx context.Context) {

    action := r.Actions[rand.Intn(len(r.Actions))]
    go r.doRandomAction(ctx, action)
  })
}

func (r *Randomizer) doRandomAction(ctx context.Context, a Action) {
  nds := r.Net.GetRandomNodes(2)
  if len(nds) < 2 || nds[0] == nil {
    log.Print("not enough nodes for random actions")
    return
  }

  switch a {
  case ActionPayment:
    var amtToSend = 5

    log.Print("Trying to send payment.")
    if nds[1] == nil {
      log.Print("Nil node.")
      return
    }
    a1, err1 := nds[0].Daemon.GetMainWalletAddress()
    a2, err2 := nds[1].Daemon.GetMainWalletAddress()
    logErr(err1)
    logErr(err2)
    if a1 == "" || a2 == "" {
      log.Print("could not get wallet addresses.", a1, a2, err1, err2)
      return
    }

    // ensure source has balance first. if doesn't, it wont work.
    bal, err := nds[0].Daemon.GetBalance(a1)
    if err != nil {
      log.Print("could not get balance for address: ", a1)
      return
    }
    if bal < amtToSend {
      log.Printf("not enough money in address: %s %d", a1, bal)
      return
    }

    // if does not succeed in 3 block times, it's hung on an error
    // ctx, _ := context.WithTimeout(ctx, r.BlockTime * 3)
    logErr(nds[0].Daemon.SendFilecoin(ctx, a1, a2, amtToSend))
    return

  case ActionAsk:
  case ActionBid:
  case ActionDeal:
  case ActionSendFile:
  }
}

func logErr(err error) {
  if err != nil {
    log.Print(err)
  }
}
