#!/bin/bash
echo "Init 3 nodes"
read -p "."
iptb init --type=filecoin --count 3 --bootstrap=skip --deployment=local

echo "Start 3 nodes"
read -p "."
iptb start

echo "Connect 3 nodes"
read -p "."
iptb connect [0-2] [0-2]

echo "Each node mines a block"
read -p "."
iptb run 0 go-filecoin mining once
sleep 1
iptb run 1 go-filecoin mining once
sleep 1
iptb run 2 go-filecoin mining once
sleep 1

echo
echo "node 2 starts mining"
read -p "."
iptb run 2 go-filecoin mining start

echo "Node 0 makes a bid size: 10 price: 10"
read -p "."
iptb run 0 go-filecoin client add-bid 10 10

echo "node 1 checks the orderbook bids"
read -p "."
iptb run 1 go-filecoin orderbook bids | jq


echo "nodes 1 creates a miner pledge: 10000 collateral: 500"
read -p "."
node1MinerAddr=$(iptb run 1 go-filecoin miner create 10000 500)
echo "$node1MinerAddr"

echo "node 1 makes an ask size 10000 price 500"
read -p "."
iptb run 1 go-filecoin miner add-ask "$node1MinerAddr" 10000 500

echo "node 0 checks the orderbook asks"
read -p "."
iptb run 0 go-filecoin orderbook asks | jq

echo
echo "node 0 imports some data"
read -p "."
dataRef=$(iptb run 0 go-filecoin client import DATA_FILE)
echo "$dataRef"
echo

echo "node 0 proposes a deal to store data"
read -p "."
iptb run 0 go-filecoin client propose-deal --ask=0 --bid=0 "$dataRef"

echo "node 1 checks the orderbook deals"
read -p "."
iptb run 1 go-filecoin orderbook deals | jq

echo "this concludes the demo"
read -p "Press enter to shutdown, or open a new terminal and play with this state"
iptb kill
