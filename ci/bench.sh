#!/usr/bin/env bash

set -e

rm -f .benchruns

echo "Starting benchmarks..."

# Execute benchmark multiple times.
for i in {1..10}
do
   echo " + run #$i"
   go test -short -bench=. >> .benchruns
done

benchstat .benchruns | tee benchstats
