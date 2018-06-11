package logs

import (
	"encoding/json"
	"io"
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

	switch op {
	case "AddNewBlock": // NewBlockMined, BroadcastBlock
		e1 := newSimEvent(l.id)
		e1["type"] = "NewBlockMined"
		e1["to"] = "all"
		e1["block"] = getStrSafe(tags, "block")
		e1["reward"] = "1000"

		e2 := newSimEvent(l.id)
		e2["type"] = "BroadcastBlock"
		e2["to"] = "all"
		e2["block"] = getStrSafe(tags, "block")
		return joinSimEvent(e1, e2)

	case "minerAddAskCmd": // AddAsk
		e := newSimEvent(l.id)
		e["type"] = "AddAsk"
		e["to"] = "all"
		e["txid"] = getStrSafe(tags, "msg")
		e["price"] = getStrSafe(tags, "price")
		e["size"] = getStrSafe(tags, "size")
		return joinSimEvent(e)

	case "clientAddBidCmd": // AddBid
		e := newSimEvent(l.id)
		e["type"] = "AddBid"
		e["to"] = "all"
		e["txid"] = getStrSafe(tags, "msg")
		e["price"] = getStrSafe(tags, "price")
		e["size"] = getStrSafe(tags, "size")
		return joinSimEvent(e)

	case "ProposeDealHandler":
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
		e1["to"] = miner
		e1["data"] = data
		e1["price"] = ask["price"]
		e1["size"] = bid["size"]

		e2 := newSimEvent(client) // SendFile
		e1["type"] = "SendFile"
		e2["to"] = miner
		e2["data"] = data
		return joinSimEvent(e1, e2)

	case "swarmConnectCmdTo": // Connected
		e := newSimEvent(l.id)
		e["type"] = "Connected"
		e["to"] = getStrSafe(tags, "peer")
		return joinSimEvent(e)

	case "AddNewMessage":
		method := getStrSafe(tags, "method")
		switch method {
		case "": // SendPayment
			e := newSimEvent(getStrSafe(tags, "from"))
			e["type"] = "SendPayment"
			e["to"] = getStrSafe(tags, "to")
			e["value"] = tags["value"]
			return joinSimEvent(e)

		default:
			return nil // unused
		}

	default:
		return nil // unused.
	}
}

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
