**Note: THE FILECOIN PROJECT IS STILL EXTREMELY CONFIDENTIAL. Do not share or discuss the project outside of designated preview channels  (chat channels, Discourse forum, GitHub, emails to Filecoin team), not even with partners/spouses/family members. If you have any questions, please email [legal@protocol.ai](mailto:legal@protocol.ai).**

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

## License

The Filecoin Project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/filecoin-network-sim/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/filecoin-network-sim/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
