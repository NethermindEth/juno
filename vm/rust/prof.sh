#!/usr/bin/env bash
blocks=(635010 635217 634990 635001 634008 633727 636939 633627 633628 635283 )
for block in ${blocks[@]}
do
  echo "bench ${block}"
  BENCH_BLOCK=$block cargo criterion -- --profile-time=150
done
