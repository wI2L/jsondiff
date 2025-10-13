#!/usr/bin/env bash

set -e

RUNS="${1:-10}"

rm -f .benchruns

echo "Starting benchmark with $RUNS runs..."

for ((i=1; i<=RUNS; i++))
do
   echo " + run #$i"
   go test -short -bench=. >> .benchruns
done

benchstat .benchruns | tee benchstats
