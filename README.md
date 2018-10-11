**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share anything outside of Protocol Labs. Do not discuss anything related to Filecoin outside of Protocol Labs, not even with your partners/spouses/other family members. If you have any questions about what can be discussed, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

# filecoin-network-sim

Server for https://github.com/filecoin-project/filecoin-network-viz

## Setup

### go-filecoin

go-filecoin is required

```sh
cd $GOPATH/src/github.com/filecoin-project/
git clone git@github.com:filecoin-project/go-filecoin.git
cd $GOPATH/src/github.com/filecoin-project/go-filecoin
git checkout feat/extractTestDaemon
go run ./build/*.go deps
go run ./build/*.go build
```

### filecoin-network-sim

```
cd $GOPATH/src/github.com/filecoin-project/
git clone git@github.com:filecoin-project/filecoin-network-sim.git

cd $GOPATH/src/github.com/filecoin-project/filecoin-network-sim
make deps
make
make runDebug
```

## Run

```
make run
```
This will start the server, the network simulation, and open a browser window pointing at the visualization.

## Warnings

- This will spawn a lot of go-filecoin processes, for running daemons and for running cli commands. Many of the commands will hang forever (fail to terminate) -- this is clearly a bug that needs to be fixed (time them out). Currently, your machine may run out of process descriptors if you leave it running indefinitely (don't do that...). (TODO: fix the bug...)
- Update wrt to the above comment, we think this is mosly fixed now, we recomend not going over 50 nodes in the simulator.
