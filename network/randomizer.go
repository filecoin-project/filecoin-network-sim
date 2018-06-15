package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"sort"
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

// Only miners should mine block
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
		r.doActionDeal(ctx)
	case ActionSendFile:
	}
}

func (r *Randomizer) doActionPayment(ctx context.Context) {
	var amtToSend = 5
	nds := r.Net.GetRandomNodes(ClientNodeType, 2)
	if len(nds) < 2 || nds[0] == nil || nds[1] == nil {
		log.Print("[RAND]\t not enough nodes for random actions")
		return
	}

	log.Print("[RAND]\t Trying to send payment.")
	a1, err1 := nds[0].Daemon.GetMainWalletAddress()
	a2, err2 := nds[1].Daemon.GetMainWalletAddress()
	logErr(err1)
	logErr(err2)
	if a1 == "" || a2 == "" {
		log.Print("[RAND]\t could not get wallet addresses.", a1, a2, err1, err2)
		return
	}

	// ensure source has balance first. if doesn't, it wont work.
	bal, err := nds[0].Daemon.WalletBalance(a1)
	if err != nil {
		log.Print("[RAND]\t could not get balance for address: ", a1)
		return
	}
	if bal < amtToSend {
		log.Printf("[RAND]\t not enough money in address: %s %d", a1, bal)
		return
	}

	// if does not succeed in 3 block times, it's hung on an error
	ctx, _ = context.WithTimeout(ctx, r.BlockTime*3)
	logErr(nds[0].Daemon.SendFilecoin(ctx, a1, a2, amtToSend))
	return
}

func (r *Randomizer) doActionAsk(ctx context.Context) {
	var size = 32 + rand.Intn(16)
	var price = rand.Intn(13) + 15

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
	var size = 32 + rand.Intn(16)
	var price = rand.Intn(17) + 1

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
	asks, err := extractAsks(out.ReadStdout())
	if err != nil {
		logErr(err)
		return
	}

	out, err = nd.Daemon.OrderbookGetBids(ctx)
	if err != nil {
		logErr(err)
		return
	}

	bids, err := extractUnusedBids(out.ReadStdout())
	if err != nil {
		logErr(err)
		return
	}

	wallet := nd.WalletAddr

	log.Printf("Wallet Address: %s\n", wallet)

	ask, bid, err := getBestDealPair(asks, bids, wallet)

	if err != nil {
		logErr(err)
		return
	}

	log.Printf("[RAND] deal found ask and bid\n")
	log.Printf("[RAND] ask %d %s %s\n", ask.ID, ask.Price.String(), ask.Size.String())
	log.Printf("[RAND] bid %d %s %s\n", bid.ID, bid.Price.String(), bid.Size.String())

	// TODO put interesting info in the file like the bid and ask id
	importFile, err := ioutil.TempFile(nd.RepoDir, "dealFile")
	if err != nil {
		panic(err)
	}

	defer importFile.Close()
	enc := json.NewEncoder(importFile)

	enc.Encode(ask)
	enc.Encode(bid)

	// Write stuff to the file
	out, err = nd.Daemon.ClientImport(importFile.Name())
	if err != nil {
		panic(err)
	}

	out, err = nd.Daemon.ProposeDeal(ask.ID, bid.ID, out.ReadStdoutTrimNewlines())
	if err != nil {
		panic(err)
		logErr(err)
		return
	}

	log.Printf("[RAND] deal proposal: %s\n", out)
	importFile.Write([]byte(out.ReadStdout()))

	/*
		askID, err := nil, nil   // get AskID
		bidID, err := nil, nil   // get BidID
		dataRef, err := nil, nil // get a CID of the data to use for the deal
	*/
}

func getBestDealPair(asks []sm.Ask, bids []sm.Bid, wallet string) (sm.Ask, sm.Bid, error) {
	// Sort bids by ID, FIFO
	sort.Slice(bids[:], func(i, j int) bool {
		return bids[i].ID < bids[j].ID
	})

	// Sort asks by Price, as we will always want the best price
	sort.Slice(asks[:], func(i, j int) bool {
		return asks[i].Price.LessThan(asks[j].Price)
	})

	// All bids for the given wallet
	var walletBids []sm.Bid

	// Find all bids for the provided wallet
	for _, b := range bids {
		if b.Owner.String() == wallet {
			log.Printf("[RAND] getBestDealPair found bid for wallet %s id, %d, price %s, size, %s\n", wallet, b.ID, b.Price.String(), b.Size.String())
			walletBids = append(walletBids, b)
		}
	}

	if len(walletBids) != 0 {
		for _, b := range walletBids {
			// Check to see if the bid fits within an ask
			for _, a := range asks {
				if b.Size.LessEqual(a.Size) && b.Price.GreaterEqual(a.Price) {
					// Valid bid for ask
					return a, b, nil
				}
			}
		}
	} else {
		log.Printf("Could not find any bids for wallet %s\n", wallet)
	}

	err := fmt.Errorf("Could not find matching ask / bid for wallet %s", wallet)
	return sm.Ask{}, sm.Bid{}, err

}

func extractAsks(input string) ([]sm.Ask, error) {

	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	as := strings.Split(o, "\n")
	log.Printf("[RAND] extractAsks: asks of length %d: %v\n", len(as), as)
	if len(as) <= 1 {
		return nil, fmt.Errorf("No Asks yes")
	}

	var asks []sm.Ask
	for _, a := range as {
		var ask sm.Ask
		log.Printf("[RAND] extractAsks: ask %v\n", a)
		err := json.Unmarshal([]byte(a), &ask)
		if err != nil {
			panic(err)
		}
		asks = append(asks, ask)
	}
	return asks, nil
}

func extractUnusedBids(input string) ([]sm.Bid, error) {
	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	bs := strings.Split(o, "\n")
	log.Printf("[RAND] extractUnusedBids: bids of length %d: %v\n", len(bs), bs)
	if len(bs) <= 1 {
		return nil, fmt.Errorf("No Bids yet")
	}

	var bids []sm.Bid
	for _, b := range bs {
		var bid sm.Bid
		log.Printf("[RAND] extractUnusedBids: bid %v\n", b)
		err := json.Unmarshal([]byte(b), &bid)
		if err != nil {
			panic(err)
		}
		if bid.Used {
			continue
		}
		bids = append(bids, bid)
	}
	return bids, nil
}

func extractDeals(input string) []sm.Deal {

	// remove last new line
	o := strings.Trim(input, "\n")
	// separate ndjson on new lines
	ds := strings.Split(o, "\n")
	log.Printf("[RAND] extractDeals: deals of length %d: %v\n", len(ds), ds)

	var deals []sm.Deal
	for _, d := range ds {
		var deal sm.Deal
		log.Printf("[RAND] extractDeals: deal %v\n", d)
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
		log.Printf("[RAND]\t ERROR: %s\n", err.Error())
	}
}
