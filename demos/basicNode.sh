#!/bin/bash
echo "Init 1 node"
read -p "."
iptb init --type=filecoin --count 1 --bootstrap=skip --deployment=local

echo "Start 1 node"
read -p "."
iptb start

echo "Nodes PeerID"
iptb run 0 go-filecoin id

echo "Swarm peers to show no connections"
read -p "."
iptb run 0 go-filecoin swarm peers
echo

# show the chain state
## chain explorer
echo "View The Chain Explorer"
read -p "."

echo look ma, a chain
iptb run 0 go-filecoin chain ls --enc=json | jq
read -p "."

echo a block data structure
genCid=$(iptb run 0 go-filecoin chain head --enc=json | jq '."/"' | tr -d '"')
iptb run 0 go-filecoin dag get "$genCid" --enc=json | jq
read -p "."

echo Pull out nonce from block "$genCid"
iptb run 0 go-filecoin dag get "$genCid"/nonce | jq '.' # If you want to get messages create some then mine, else messages is niil
read -p "."
# show the coinbase
# this is just the zero-th tx

echo check wallet balance
wallet0=$(iptb run 0 go-filecoin wallet addrs ls | tail -n1)
iptb run 0 go-filecoin wallet balance $wallet0
read -p "."

echo Press enter to kill the demo
read -p "."
iptb kill
