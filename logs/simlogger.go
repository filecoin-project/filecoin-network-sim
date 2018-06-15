package logs

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
	//gcid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type SimLogger struct {
	id  string
	pr  *io.PipeReader
	pw  *io.PipeWriter
	buf chan map[string]string
}

func NewSimLogger(nodeid string, eventlogs io.Reader) *SimLogger {
	bufch := make(chan map[string]string, 0)
	pr, pw := io.Pipe()
	sl := &SimLogger{nodeid, pr, pw, bufch}
	go sl.transformEventLogs(eventlogs)
	return sl
}

func (l *SimLogger) Logf(format string, a ...interface{}) {
	log.Printf("[SIM]\t %s", fmt.Sprintf(format, a...))
}

func (l *SimLogger) Reader() io.Reader {
	return l.pr
}

func (l *SimLogger) Close() error {
	l.pw.CloseWithError(nil)
	return nil
}

// {"type": "NetworkChurn", "from": "mineraddr1", "role": "[Miner|Client]", "change": "[join|leave]"}
// {"type": "MinerJoins", "from": "mineraddr1"}
// {"type": "MinerLeaves", "from": "mineraddr1"}
// {"type": "ClientJoins", "from": "mineraddr1"}
// {"type": "ClientLeaves", "from": "mineraddr1"}
func (l *SimLogger) WriteEvent(e map[string]interface{}) error {
	return json.NewEncoder(l.pw).Encode(e)
}

func NetworkChurnEvent(id, role string, joins bool) map[string]interface{} {
	m := newSimEvent(id)
	if joins {
		m["type"] = role + "Joins"
	} else {
		m["type"] = role + "Leaves"
	}
	return m
}

func (l *SimLogger) transformEventLogs(r io.Reader) {
	d := json.NewDecoder(r)
	e := json.NewEncoder(l.pw)
	for {
		var im map[string]interface{}
		err := d.Decode(&im)
		if err != nil {
			l.pw.CloseWithError(err)
			return // bail out. failed.
		}

		for _, om := range l.convertEL2SL(im) {
			if err := e.Encode(&om); err != nil {
				l.pw.CloseWithError(err)
				return // bail out. failed.
			}
		}

		// everything went well.
	}
}

// {"type": "NewBlockMined", "from": "mineraddr1", "to": "all", "block": "<blockCID>"}
// {"type": "BroadcastBlock", "from": "mineraddr1", "to": "all", "block": "<blockCID>"}
// {"type": "AddAsk", "from": "mineraddr1", "to": "all", "tx": "<askTxCID>", "price": "<priceInFIL>", "size": "<sizeInBytes>"}
// {"type": "AddBid", "from": "mineraddr1", "to": "all", "tx": "<bidTxCID>", "price": "<priceInFIL>", "size": "<sizeInBytes>"}
// {"type": "MakeDeal", "from": "mineraddr1", "to": "mineraddr2", "tx": "<dealTxCID>", "price": "<priceInFIL>", "size": "<sizeInBytes>"}
// {"type": "SendFile", "from": "mineraddr1", "to": "mineraddr2", "size": "<sizeInBytes>"}
// {"type": "SendPayment", "from": "mineraddr1", "to": "mineraddr2", "tx": "<txCID>", "value": "<valueInFIL>"}
// {"type": "Connected", "from": "mineraddr1", "to": "mineraddr2"}
func (l *SimLogger) convertEL2SL(el map[string]interface{}) []map[string]interface{} {

	op, ok := el["Operation"].(string)
	if !ok {
		return nil // who knows what it is
	}

	tags, ok := el["Tags"].(map[string]interface{})
	if !ok {
		return nil // everything we use has tags.
	}

	_, ok = tags["error"]
	if ok {
		// Skip the invalid operations
		return nil
	}

	switch op {

	case "sm.AddBid":
		e := newSimEvent(l.id)
		e["type"] = "AddBid"
		e["bid"] = tags["bid"]
		return joinSimEvent(e)

	case "sm.AddAsk":
		e := newSimEvent(l.id)
		e["type"] = "AddAsk"
		e["ask"] = tags["ask"]
		return joinSimEvent(e)

	case "AddNewBlock": // NewBlockMined, BroadcastBlock
		block := getBlockFromTags(tags)

		e1 := newSimEvent(l.id)
		e1["type"] = "NewBlockMined"
		e1["to"] = "all"
		e1["reward"] = "20000"
		e1["block"] = block.Cid().String()
		e1["from"] = block.Miner.String()

		e2 := newSimEvent(l.id)
		e2["type"] = "BroadcastBlock"
		e2["to"] = "all"
		e2["block"] = block.Cid().String()
		e2["from"] = block.Miner.String()

		return joinSimEvent(e1, e2)

	case "ProcessNewBlock": //SawBlock
		block := getBlockFromTags(tags)

		e := newSimEvent(l.id)
		e["type"] = "SawBlock"
		e["block"] = block.Cid().String()
		e["from"] = block.Miner.String()
		//TODO a "to" field doesn't make sense here does it? I don't tell peers about all the blocks I see
		//e["to"] = "all"

		return joinSimEvent(e)

	case "acceptNewBestBlock": //PickedChain
		block := getBlockFromTags(tags)

		e := newSimEvent(l.id)
		e["type"] = "PickedChain"
		e["block"] = block.Cid().String()
		e["from"] = block.Miner.String()
		//TODO a "to" field doesn't make sense here does it either..
		//e["to"] = "all"

		return joinSimEvent(e)

	case "minerCreateCmd":
		e := newSimEvent(getStrSafe(tags, "from-address"))
		e["pledge"] = tags["pledge"]
		e["collateral"] = tags["collateral"]
		e["miner-addr"] = tags["addr"]
		e["type"] = "CreateMiner"
		e["to"] = "all"
		return joinSimEvent(e)

	case "finishDeal": //MakeDeal
		e := newSimEvent(l.id)
		e["type"] = "FinishDeal"
		e["from"] = tags["miner"]
		e["deal"] = tags["deal"]
		e["txid"] = tags["msgCid"]
		return joinSimEvent(e)

	case "fetchData": //SendPieces
		e := newSimEvent(l.id)
		e["type"] = "SendPieces"
		e["data"] = tags["data"]
		return joinSimEvent(e)

		/*
			case "minerAddAskCmd": // AddAsk
				message := getMsgFromTags(tags)
				msgID, err := message.Cid()
				if err != nil {
					panic(err) // developer error
				}

				e := newSimEvent(l.id)
				e["type"] = "AddAsk"
				e["to"] = "all"
				e["price"] = getStrSafe(tags, "price")
				e["size"] = getStrSafe(tags, "size")

				e["from"] = message.From.String()
				e["txid"] = msgID.String()
				return joinSimEvent(e)

			case "clientAddBidCmd": // AddBid
				message := getMsgFromTags(tags)
				msgID, err := message.Cid()
				if err != nil {
					panic(err)
				}

				e := newSimEvent(l.id)
				e["type"] = "AddBid"
				e["to"] = "all"
				e["price"] = getStrSafe(tags, "price")
				e["size"] = getStrSafe(tags, "size")
				e["from"] = message.From.String()
				e["txid"] = msgID.String()
				return joinSimEvent(e)
		*/

	case "ProposeDeal":
		ask, ok1 := tags["ask"].(map[string]interface{})
		bid, ok2 := tags["bid"].(map[string]interface{})
		deal, ok3 := tags["deal"].(map[string]interface{})
		if !(ok1 && ok2 && ok3) {
			return nil // broken.
		}
		dataRef, ok4 := deal["dataRef"].(map[string]interface{})
		if !ok4 {
			return nil // broken
		}

		miner := getStrSafe(ask, "owner")
		client := getStrSafe(bid, "owner")
		data := getStrSafe(dataRef, "/")

		e1 := newSimEvent(client) // MakeDeal
		e1["type"] = "MakeDeal"
		// TODO this address is wrong in the browser console
		e1["to"] = tags["miner-owner"]
		e1["data"] = data
		e1["price"] = ask["price"]
		e1["size"] = bid["size"]
		e1["ask"] = ask
		e1["bid"] = bid
		e1["deal"] = deal

		e2 := newSimEvent(client) // SendFile
		e2["type"] = "SendFile"
		e2["to"] = miner
		e2["data"] = data
		return joinSimEvent(e1, e2)

	case "swarmConnectCmdTo": // Connected
		e := newSimEvent(l.id)
		e["type"] = "Connected"
		e["to"] = getStrSafe(tags, "peer")
		return joinSimEvent(e)

	case "AddNewMessage":
		message := getMsgFromTags(tags)
		switch message.Method {

		/*
			case "addAsk":
				// WOW this actually works holy shit
				t := []abi.Type{abi.BytesAmount, abi.BytesAmount}
				v, err := abi.DecodeValues(message.Params, t)
				if err != nil {
					panic(err)
				}
				price := v[0].String()
				size := v[1].String()

				e := newSimEvent(getStrSafe(tags, "from"))
				e["type"] = "AddAsk"
				e["to"] = message.To.String()
				e["from"] = message.From.String()
				e["value"] = message.Value.String()
				e["size"] = size
				e["price"] = price
				e["txid"] = cid.String()
				return joinSimEvent(e)

			case "addBid":
				t := []abi.Type{abi.BytesAmount, abi.BytesAmount}
				v, err := abi.DecodeValues(message.Params, t)
				if err != nil {
					panic(err)
				}
				price := v[0].String()
				size := v[1].String()

				e := newSimEvent(getStrSafe(tags, "from"))
				e["type"] = "AddBid"
				e["to"] = message.To.String()
				e["from"] = message.From.String()
				e["value"] = message.Value.String()
				e["size"] = size
				e["price"] = price
				e["txid"] = cid.String()
				return joinSimEvent(e)
		*/

		case "addDeal":
			//              askID       bidID          sig       data
			t := []abi.Type{abi.Integer, abi.Integer, abi.Bytes, abi.Bytes}
			v, err := abi.DecodeValues(message.Params, t)
			if err != nil {
				panic(err)
			}
			askID := v[0].String() // askID
			bidID := v[1].String() // bidID

			sig, err := v[2].Serialize() // sig
			if err != nil {
				panic(err)
			}

			data, err := v[2].Serialize() // data
			if err != nil {
				panic(err)
			}

			e := newSimEvent(l.id)
			e["type"] = "AddDeal"
			e["to"] = "all" // message.To is StorageMarketAddress
			e["from"] = message.From.String()
			e["askID"] = askID
			e["bidID"] = bidID
			e["sig"] = string(sig)   // probs empty
			e["data"] = string(data) //cid
			return joinSimEvent(e)

		case "sendMessage":
			e := newSimEvent(getStrSafe(tags, "from"))
			e["type"] = "SendPayment"
			e["to"] = message.To.String()
			e["from"] = message.From.String()
			e["value"] = message.Value.String()
			return joinSimEvent(e)

		default:
			fmt.Printf("UNKNOWN MESSAGE: %v", message)
			return nil // unused
		}
	case "HeartBeat":
		e := newSimEvent(l.id)
		e["type"] = "HeartBeat"
		e["peer-id"] = tags["peer-id"]
		e["peers"] = tags["peers"]
		e["asks"] = tags["ask-list"]
		e["bids"] = tags["bid-list"]
		e["deals"] = tags["deal-list"]
		e["best-block"] = tags["best-block"]
		e["pending"] = tags["pending-messages"]
		e["wallet-addrs"] = tags["wallet-address"]
		return joinSimEvent(e)

	default:
		return nil // unused.
	}
}

func getBlockFromTags(tags map[string]interface{}) types.Block {
	var block types.Block
	blk, err := json.Marshal(tags["block"])
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(blk, &block); err != nil {
		panic(err)
	}
	return block
}

func getMsgFromTags(tags map[string]interface{}) types.Message {
	var message types.Message
	msg, err := json.Marshal(tags["message"])
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(msg, &message); err != nil {
		panic(err)
	}
	return message
}

// NONE OF THIS IS SAFE OMG

func getStrSafe(m map[string]interface{}, k string) string {
	v, _ := m[k].(string)
	return v
}

func getIntSafe(m map[string]interface{}, k string) int {
	v, _ := m[k].(int)
	return v
}

func newSimEvent(id string) map[string]interface{} {
	return map[string]interface{}{"from": id}
}

func joinSimEvent(e ...map[string]interface{}) []map[string]interface{} {
	l := make([]map[string]interface{}, len(e))
	for i, m := range e {
		l[i] = m
	}
	return l
}
