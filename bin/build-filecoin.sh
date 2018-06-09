#!/bin/sh

die() {
  echo "error: $1"
  exit 1
}

# check we have golang
which go >/dev/null || die "please install go: https://golang.org/dl"

# check go filecoin is on the right branch
gf_pkg="github.com/filecoin-project/go-filecoin"
gf_dir="$GOPATH/src/$gf_pkg"
gf_branch_exp="demo/network-simulation"
gf_branch_act=$(git -C "$gf_dir" rev-parse --abbrev-ref HEAD)
if [ "$gf_branch_exp" != "$gf_branch_act" ]; then
  echo 2> "using go-filecoin at $gf_pkg"
  echo 2> "go-filecoin currently on branch: $gf_branch_act"
  echo 2> "go-filecoin should be on branch: $gf_branch_exp"
  echo 2> "please check out the right branch:"
  echo 2> "  cd $gf_dir && git checkout $gf_branch_exp"
  exit 1
fi

# check we have the go-filecoin binary built. if not, build it for ourselves.
gf_bin="bin/go-filecoin"
if [ ! -f "$gf_bin" ]; then
  echo go build -o "$gf_bin" "$gf_pkg"
  go build -o "$gf_bin" "$gf_pkg"
fi
