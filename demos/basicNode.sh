#!/bin/bash

pretty() {
  while read line; do
     echo "$line" | underscore print --color --wrapwidth 200
  done
}

pause() {
  read
}

_step=0
step() {
  _step=$(($_step + 1))
  printf "\n# Step $_step: $@"
  pause
}

step "Init 1 node"
iptb init --type=filecoin --count 1 --bootstrap=skip --deployment=local -f

step "Start 1 node"
iptb start

step "Nodes PeerID"
iptb run 0 go-filecoin id

step "Swarm peers to show no connections"
iptb run 0 go-filecoin swarm peers
step

# show the chain state
## chain explorer
step "View The Chain Explorer"
echo "http://localhost:..."

step "look ma, a chain"
iptb run 0 go-filecoin mining once
iptb run 0 go-filecoin mining once
iptb run 0 go-filecoin mining once
iptb run 0 go-filecoin chain ls --enc=json | pretty

step "mine a single block"
iptb run 0 go-filecoin mining once

step "a block data structure"
genCid=$(iptb run 0 go-filecoin chain head)
iptb run 0 go-filecoin dag get "$genCid" --enc=json | pretty

step "Pull out nonce from block $genCid"
iptb run 0 go-filecoin dag get "zDPWYqFCx2A53BRSx8G26hns41QWnzWvPD64djNAfsgyDG7jnpWs/nonce"
iptb run 0 go-filecoin dag get "$genCid"/nonce | jq '.' | pretty
# If you want to get messages create some then mine, else messages is niil
# show the coinbase
# this is just the zero-th tx

step "check wallet balance"
wallet0=$(iptb run 0 go-filecoin wallet addrs ls | tail -n1)
iptb run 0 go-filecoin wallet balance $wallet0

step "Press enter to kill the demo"
iptb kill
echo ""
