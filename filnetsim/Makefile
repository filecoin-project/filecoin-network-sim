build: build-viz
	go build

deps: filecoin-network-viz

build-viz:
	# cd filecoin-network-viz/viz-circle && make build

filecoin-network-viz:
	git clone git@github.com:filecoin-project/filecoin-network-viz

run: build deps
	open http://127.0.0.1:7002/
	./filnetsim

runDebug: build deps
	open http://127.0.0.1:7002/
	./filnetsim --debug

buildAndRun: build run
