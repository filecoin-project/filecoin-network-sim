#!/bin/bash
echo "Init 10 miner nodes"
read -p "."
iptb init --type=filecoin --count 10 --bootstrap=skip --deployment=local

echo "Start 10 miner nodes"
read -p "."
iptb start

echo "Connect 10 miner nodes"
read -p "."
iptb connect [0-9] [0-9]

echo "10 miner nodes start mining"
read -p "."
iptb for-each go-filecoin mining start

echo "Init a Client Node (press enter to overwrite current filecoin repo)"
read -p "."
rm -rf ~/.filecoin
go-filecoin init

echo "Start Client Node"
read -p "."
go-filecoin daemon > /dev/null 2>&1 &

echo "Get addresses of miner nodes to connect to"
read -p "."
addr1=$(iptb run 4 go-filecoin id --format="<addrs>" | tail -n1)
addr2=$(iptb run 5 go-filecoin id --format="<addrs>" | tail -n1)
echo "$addr1"
echo "$addr2"
echo

echo "Connect to miner addresses"
read -p "."
go-filecoin swarm connect $addr1
go-filecoin swarm connect $addr2

echo "Show Connections to Miners"
read -p "."
go-filecoin swarm peers

echo
echo "Watch Chain State Advance"
read -p "."
echo "Chain Head:"
go-filecoin chain head
read -p "."

echo "Wait 1 block time before pressing 'Enter'"
read -p "."
echo "Chain Head:"
go-filecoin chain head
read -p "."

echo
echo show balance change on miner wallets
wallet0=$(iptb run 0 go-filecoin wallet addrs ls)
wallet1=$(iptb run 1 go-filecoin wallet addrs ls)
echo
echo "Wallet0: $wallet0"
echo "Wallet1: $wallet1"
echo

echo
read -p "."
echo "$wallet0" wallet balance
iptb run 0 go-filecoin wallet balance "$wallet0"
read -p "."
sleep 3
echo "$wallet0" wallet balance has increased
iptb run 0 go-filecoin wallet balance "$wallet0"

read -p "."
echo "$wallet1" wallet balance
iptb run 1 go-filecoin wallet balance "$wallet1"
read -p "."
sleep 3
echo "$wallet1" wallet balance has increased
iptb run 1 go-filecoin wallet balance "$wallet1"
read -p "."

echo "Client wallet balance"
read -p "."
walletC=$(go-filecoin wallet addrs ls)
echo Wallet Address: "$walletC"
go-filecoin wallet balance "$walletC"
read -p "."

echo "Miner0 ($wallet0) Sends client ($walletC) 50 filecoin"
read -p "."
echo "Client waits for message to be mined.....(aprox 30sec)"
sendFLMsg=$(iptb run 0 go-filecoin message send --value=50 "$walletC")
go-filecoin message wait "$sendFLMsg"
read -p "."

echo "Client wallet balance"
read -p "."
walletC=$(go-filecoin wallet addrs ls)
echo Wallet Address: "$walletC"
go-filecoin wallet balance "$walletC"

echo Press enter to kill the miners and client
read -p "."
iptb kill
killall go-filecoin
