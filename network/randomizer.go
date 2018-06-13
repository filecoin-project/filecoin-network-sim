package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	sm "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
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
			if size < 6 {
				_, err := r.Net.AddNode(AnyNodeType)
				logErr(err)
			}
		}()
	})
}

// Only miners should mine block
func (r *Randomizer) mineBlocks(ctx context.Context) {
	r.periodic(ctx, r.BlockTime, func(ctx context.Context) {
		n := r.Net.GetRandomNode(MinerNodeType)
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
		r.doActionDeal(ctx)
	case ActionSendFile:
	}
}

func (r *Randomizer) doActionPayment(ctx context.Context) {
	var amtToSend = 5
	nds := r.Net.GetRandomNodes(ClientNodeType, 2)
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

func (r *Randomizer) doActionDeal(ctx context.Context) {
	psudoData := "QmTz3oc4gdpRMKP2sdGUPZTAGRngqjsi99BPoztyP53JMM"
	nd := r.Net.GetRandomNode(ClientNodeType)
	if nd == nil {
		return
	}

	/*
		from, err := nd.Daemon.GetMainWalletAddress()
		if err != nil {
			logErr(err)
			return
		}
	*/

	out, err := nd.Daemon.OrderbookGetAsks(ctx)
	if err != nil {
		logErr(err)
		return
	}
	asks := extractAsks(out.ReadStdout())

	out, err = nd.Daemon.OrderbookGetBids(ctx)
	if err != nil {
		logErr(err)
		return
	}
	bids := extractUnusedBids(out.ReadStdout())

	out, err = nd.Daemon.ProposeDeal(asks[0].ID, bids[0].ID, psudoData)
	if err != nil {
		logErr(err)
		return
	}

	/*
		askID, err := nil, nil   // get AskID
		bidID, err := nil, nil   // get BidID
		dataRef, err := nil, nil // get a CID of the data to use for the deal
	*/
}

func extractAsks(input string) []sm.Ask {

	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	as := strings.Split(o, "\n")
	fmt.Println(as)
	fmt.Println(len(as))

	var asks []sm.Ask
	for _, a := range as {
		var ask sm.Ask
		fmt.Println(a)
		err := json.Unmarshal([]byte(a), &ask)
		if err != nil {
			panic(err)
		}
		asks = append(asks, ask)
	}
	return asks
}

func extractUnusedBids(input string) []sm.Bid {
	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	bs := strings.Split(o, "\n")
	fmt.Println(bs)
	fmt.Println(len(bs))

	var bids []sm.Bid
	for _, b := range bs {
		var bid sm.Bid
		fmt.Println(b)
		err := json.Unmarshal([]byte(b), &bid)
		if err != nil {
			panic(err)
		}
		if bid.Used {
			continue
		}
		bids = append(bids, bid)
	}
	return bids
}

func extractDeals(input string) []sm.Deal {

	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	ds := strings.Split(o, "\n")
	fmt.Println(ds)
	fmt.Println(len(ds))

	var deals []sm.Deal
	for _, d := range ds {
		var deal sm.Deal
		fmt.Println(d)
		err := json.Unmarshal([]byte(d), &deal)
		if err != nil {
			panic(err)
		}
		deals = append(deals, deal)
	}
	return deals
}

func logErr(err error) {
	if err != nil {
		fmt.Errorf("[RAND]\t ERROR: %s\n", err.Error())
	}
}
