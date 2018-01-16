#!/bin/bash -
#

set -e

TARGET=build_tmp/src/github.com/codeskyblue/gosuv
HOST="pi3-0"
ssh pi@$HOST mkdir -p $TARGET

rsync -avz -e "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" --progress \
	--exclude gosuv --exclude dist --exclude .git \
	--delete \
	. pi@$HOST:$TARGET

echo "Build remotely ..."
ssh pi@$HOST bash $TARGET/build_standalone.sh
echo "Build finished, copying ..."
scp pi@$HOST:$TARGET/gosuv ./dist/gosuv-linux-arm
echo "All finished"
