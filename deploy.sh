#!/bin/bash -
#
# https://equinox.io/docs/continuous-deployment/travis-ci

set -eu -o pipefail

# Download and unpack the most recent Equinox release tool
wget https://bin.equinox.io/c/mBWdkfai63v/release-tool-stable-linux-amd64.tgz
tar -vxf release-tool-stable-linux-amd64.tgz

VERSION=$(git describe --abbrev=0 --tags)
REVCNT=$(git rev-list --count HEAD)
DEVCNT=$(git rev-list --count $VERSION)
ISDEV=false
if test $REVCNT != $DEVCNT
then
	VERSION="$VERSION.dev$(expr $REVCNT - $DEVCNT)"
	ISDEV=true
fi
echo "VER: $VERSION"

GITCOMMIT=$(git rev-parse HEAD)
BUILDTIME=$(date -u +%Y/%m/%d-%H:%M:%S)

LDFLAGS="-X main.VERSION=$VERSION -X main.BUILDTIME=$BUILDTIME -X main.GITCOMMIT=$GITCOMMIT"
if [[ -n "${EX_LDFLAGS:-""}" ]]
then
	LDFLAGS="$LDFLAGS $EX_LDFLAGS"
fi

echo $VERSION
CHANNEL="stable"
if test "$ISDEV" = "true"
then
	CHANNEL="dev"
fi

go get github.com/elazarl/go-bindata-assetfs/...
go-bindata-assetfs -tags bindata res/...

# TODO: Replace app_xxx with correct application ID
./equinox release \
	    --channel="$CHANNEL" \
        --version="$VERSION" \
        --app="app_8Gji4eEAdDx" \
        --platforms="darwin_amd64 linux_amd64" \
        --signing-key="equinox.key" \
        --token="$EQUINOX_API_TOKEN" \
        -- -ldflags "-X main.Version $TRAVIS_COMMIT"
