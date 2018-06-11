build:
	cd filnetsim && go build

install:
	cd filnetsim && go install

test:
	go test ./...

run: build
	filnetsim/filnetsim

runDebug: build deps
	filnetsim/filnetsim --debug

deps: submodules bin/go-filecoin

bin/go-filecoin:
	@bin/build-filecoin.sh

submodules:
	git submodule init
	git submodule update
