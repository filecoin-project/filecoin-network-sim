package network

import (
	"context"
	"log"
	"math/rand"
	"time"
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
	go r.addAndRemoveNodes(ctx)
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

func (r *Randomizer) addAndRemoveNodes(ctx context.Context) {
	r.periodic(ctx, r.BlockTime*4, func(ctx context.Context) {
		go func() {
			size := r.Net.Size()
			if size < r.TotalNodes {
				t := AnyNodeType
				if size < 2 {
					t = MinerNodeType
				}
				_, err := r.Net.AddNode(t)
				logErr(err)
			}
		}()
	})
}

func (r *Randomizer) mineBlocks(ctx context.Context) {
	r.periodic(ctx, r.BlockTime, func(ctx context.Context) {
		n := r.Net.GetRandomNode(AnyNodeType)
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
	switch a {
	case ActionPayment:
		r.doActionPayment(ctx)
	case ActionAsk:
		r.doActionAsk(ctx)
	case ActionBid:
		r.doActionBid(ctx)
	case ActionDeal:
	case ActionSendFile:
	}
}

func (r *Randomizer) doActionPayment(ctx context.Context) {
	var amtToSend = 5
	nds := r.Net.GetRandomNodes(AnyNodeType, 2)
	if len(nds) < 2 || nds[0] == nil || nds[1] == nil {
		log.Print("not enough nodes for random actions")
		return
	}

	log.Print("Trying to send payment.")
	a1, err1 := nds[0].Daemon.GetMainWalletAddress()
	a2, err2 := nds[1].Daemon.GetMainWalletAddress()
	logErr(err1)
	logErr(err2)
	if a1 == "" || a2 == "" {
		log.Print("could not get wallet addresses.", a1, a2, err1, err2)
		return
	}

	// ensure source has balance first. if doesn't, it wont work.
	bal, err := nds[0].Daemon.WalletBalance(a1)
	if err != nil {
		log.Print("could not get balance for address: ", a1)
		return
	}
	if bal < amtToSend {
		log.Printf("not enough money in address: %s %d", a1, bal)
		return
	}

	// if does not succeed in 3 block times, it's hung on an error
	ctx, _ = context.WithTimeout(ctx, r.BlockTime*3)
	logErr(nds[0].Daemon.SendFilecoin(ctx, a1, a2, amtToSend))
	return
}

func (r *Randomizer) doActionAsk(ctx context.Context) {
	var size = 2048 + rand.Intn(2048)
	var price = rand.Intn(30)

	nd := r.Net.GetRandomNode(MinerNodeType)
	if nd == nil {
		return
	}

	// ensure they have a miner addrss associated with them.
	from, err := nd.CreateOrGetMinerIdentity()
	if err != nil {
		logErr(err)
		return
	}

	logErr(nd.Daemon.MinerAddAsk(ctx, from, size, price))
	return
}

func (r *Randomizer) doActionBid(ctx context.Context) {
	var size = 2048 + rand.Intn(2048)
	var price = rand.Intn(30)

	nd := r.Net.GetRandomNode(ClientNodeType)
	if nd == nil {
		return
	}

	// ensure they have an addr they can bid from
	from, err := nd.Daemon.GetMainWalletAddress()
	if err != nil {
		logErr(err)
		return
	}

	logErr(nd.Daemon.ClientAddBid(ctx, from, size, price))
	return
}

func logErr(err error) {
	if err != nil {
		log.Print(err)
	}
}
