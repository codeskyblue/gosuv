#!/bin/bash -
#


# get current tag
VERSION=$(git name-rev --tags --name-only $(git rev-parse HEAD))

# use the latest tag
if test "X$VERSION" = "Xundefined"
then
    VERSION="$(git describe --abbrev=0 --tags)"
    if test -z "$VERSION"
    then
        VERSION="0.0.1"
    fi
    VERSION="${VERSION}.dev"
fi

SHA=$(git rev-parse HEAD)
exec go build -ldflags "-X main.GOSUV_VERSION=$VERSION"
