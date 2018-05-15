build:
	cd filnetsim && go build

install:
	cd filnetsim && go install

run: build
	open http://127.0.0.1:7002/
	filnetsim/filnetsim

runDebug: build deps
	open http://127.0.0.1:7002/
	filnetsim/filnetsim --debug

deps: submodules bin/go-filecoin

bin/go-filecoin:
	@bin/build-filecoin.sh

submodules:
	git submodule init
	git submodule update
