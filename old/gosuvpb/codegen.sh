#!/bin/bash -ex
#
cd $(dirname $0)
protoc --go_out=plugins=grpc:. *.proto
go install
