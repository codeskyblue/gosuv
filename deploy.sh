#!/bin/bash -
#
# https://equinox.io/docs/continuous-deployment/travis-ci

set -eu -o pipefail

# Download and unpack the most recent Equinox release tool
wget https://bin.equinox.io/c/mBWdkfai63v/release-tool-stable-linux-amd64.tgz
tar -vxf release-tool-stable-linux-amd64.tgz

VER=$(git describe --tags --dirty --always)
echo "VER: $VER"

./equinox release \
	    --channel="stable" \
        --version="$VER" \
        --app="app_8Gji4eEAdDx" \
        --platforms="darwin_amd64 linux_amd64 linux_arm" \
        --signing-key="equinox.key" \
        --token="$EQUINOX_API_TOKEN" \
        -- -tags vfs -ldflags "-X main.Version=$VER"
