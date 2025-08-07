#!/bin/bash

DATA_DIR=${DATA_DIR:-"consensus-tests"}
export STARKNET_RPC="http://node0:6060"

sleep 2 # TODO: Figure out why we need to wait

shooters="$1"

root_address="0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"
export ROOT_PRIVATE_KEY="0x3a4791edf67fa0b32b812e41bc8bc4e9b79915412b1331f7669cbe23e97e15a"

until starkli account fetch "$root_address" --force --output "$DATA_DIR/root.json"; do
    echo "Failed to fetch root account, retrying..."
    sleep 1
done

for ((i = 1; i <= shooters; i++)); do
    consensus-tests/setup.sh "$i"
done
