# filecoin-network-sim

Server for https://github.com/filecoin-project/filecoin-network-viz

## Setup

```
cd $GOPATH/src/github.com/filecoin-project/
git clone git@github.com:filecoin-project/filecoin-network-sim.git

cd $GOPATH/src/github.com/filecoin-project/filecoin-network-sim
make deps
make
```

## Run

```
make run
```
This will start the server, the network simulation, and open a browser window pointing at the visualization.

## Warnings

- This will spawn a lot of go-filecoin processes, for running daemons and for running cli commands. Many of the commands will hang forever (fail to terminate) -- this is clearly a bug that needs to be fixed (time them out). Currently, your machine may run out of process descriptors if you leave it running indefinitely (don't do that...). (TODO: fix the bug...)
