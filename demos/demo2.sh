#!/bin/bash
echo spawn network with iptb 10 fully connected nodes mining every 2 sec
sleep 2

iptb init --type=filecoin --count 10 --bootstrap=skip --deployment=local
iptb start
iptb connect [0-9] [0-9]
iptb for-each go-filecoin mining start

sleep 1
echo connect node1 to two nodes in new network
echo get an address to dial
sleep 1
addr1=$(iptb run 4 go-filecoin id --format="<addrs>" | tail -n1)
addr2=$(iptb run 5 go-filecoin id --format="<addrs>" | tail -n1)
sleep 1
echo Init a daemon
go-filecoin init
sleep 1
echo Start the daemon
sleep 1
go-filecoin daemon > /dev/null 2>&1 &
echo Connect to addresses
sleep 1
go-filecoin swarm connect $addr1
go-filecoin swarm connect $addr2

echo show connections
sleep 1
go-filecoin swarm peers

echo show chain state has advanced
echo
sleep 1
go-filecoin chain head
sleep 5
echo
echo ...advancing
echo
go-filecoin chain head

sleep 1
echo
echo show balance has changed on some accounts we saw before
wallet0=$(iptb run 0 go-filecoin wallet addrs ls)
wallet1=$(iptb run 1 go-filecoin wallet addrs ls)

sleep 1
echo node0 wallet balance
iptb run 0 go-filecoin wallet balance "$wallet0"
sleep 3
echo node0 wallet balance has increased
iptb run 0 go-filecoin wallet balance "$wallet0"

sleep 1
echo node1 wallet balance
iptb run 1 go-filecoin wallet balance "$wallet1"
sleep 3
echo node1 wallet balance has increased
iptb run 1 go-filecoin wallet balance "$wallet1"
