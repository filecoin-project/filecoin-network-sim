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
gf_branch_exp="feat/filecoin-network-sim"
gf_branch_act=$(git -C "$gf_dir" rev-parse --abbrev-ref HEAD)
if [ "$gf_branch_exp" != "$gf_branch_act" ]; then
  echo "using go-filecoin at $gf_pkg"
  echo "go-filecoin currently on branch: $gf_branch_act"
  die "go-filecoin should be on branch: $gf_branch_exp"
fi
