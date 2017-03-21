#!/bin/bash -
#

set -e
set -o pipefail

cd $(dirname $0)
source "$HOME/.bash_profile"
export GOPATH=$GOPATH:/home/pi/build_tmp 

go generate
exec go build -tags vfs "$@"
