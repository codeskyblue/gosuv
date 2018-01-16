#!/bin/bash -
#

set -e
set -o pipefail

cd $(dirname $0)

if test $(whoami) = "pi"
then
    export GOPATH=/home/pi/build_tmp 
	export GOROOT=$HOME/go
	export PATH=$PATH:$GOROOT/bin
fi
#[[ -f $HOME/.bash_profile ]] && source "$HOME/.bash_profile"
#[[ -f $HOME/.bashrc ]] && source "$HOME/.bashrc"

go generate
exec go build -tags vfs "$@"
