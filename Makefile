VIZ_NODE_MODULES := "filecoin-network-viz/viz-circle/node_modules"
EXPLORER_NODE_MODULES := "filecoin-explorer/node_modules"

build: frontend
	cd filnetsim && go build

install:
	cd filnetsim && go install

test:
	go test ./...

run:
	filnetsim/filnetsim

runDebug: build
	filnetsim/filnetsim --debug

deps: submodules bin/go-filecoin $(VIZ_NODE_MODULES) $(EXPLORER_NODE_MODULES)

frontend: submodules viz explorer
.PHONY: frontend

viz: $(VIZ_NODE_MODULES)
	(cd filecoin-network-viz/viz-circle; yarn run build)
	(cd filecoin-network-viz/viz-blockchain; yarn run build)

$(VIZ_NODE_MODULES):
	(cd filecoin-network-viz/viz-circle; yarn install)
	(cd filecoin-network-viz/viz-blockchain; yarn install)

explorer: $(EXPLORER_NODE_MODULES)
	(cd filecoin-explorer; yarn run build)

$(EXPLORER_NODE_MODULES):
	(cd filecoin-explorer; yarn install)

bin/go-filecoin:
	@bin/build-filecoin.sh

submodules:
	git submodule init
	git submodule update
