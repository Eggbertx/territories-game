#!/bin/bash

argc=$#
function usage {
	if [ "$1" = "-h" ]; then
  		echo "Usage: $0 <build|test|clean>"
  		exit 1
	fi
}
if [ -z "$1" ] || [ "$1" == "build" ]; then
  go build -v  -trimpath -gcflags="-trimpath" ./cmd/territories-referee
elif [ "$1" == "test" ]; then
  go test -cover ./...
elif [ $1 == "clean" ]; then
  rm -fv $OUT
else
  echo "Invalid argument. Use 'build', 'test', or 'clean'"
fi