build:
	go build

deps: filecoin-network-viz

filecoin-network-viz:
	git clone git@github.com:filecoin-project/filecoin-network-viz

run: build deps
	open http://127.0.0.1:7002/
	./filnetsim

buildAndRun: build run
