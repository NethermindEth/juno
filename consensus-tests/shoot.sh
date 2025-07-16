#!/bin/bash

DATA_DIR=${DATA_DIR:-"consensus-tests"}

if [ -z "$1" ]; then
    INDEX=$(curl --silent 'http://webdis:7379/INCR/lock' | jq '.INCR')
else
    INDEX="$1"
fi

echo "Shooting account #$INDEX"

PORT="$((6060 + INDEX % 4))"
TARGET="$((INDEX % 4))"
export STARKNET_RPC="http://node$TARGET:6060"

PRIVATE_KEY=$(cat "$DATA_DIR/$INDEX.key")

for i in {1..1000}; do
    starkli invoke eth transfer 0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e 1 0 \
        --account "$DATA_DIR/$INDEX.json" \
        --private-key "$PRIVATE_KEY" \
        --l1-gas 1000000 \
        --l1-gas-price-raw 1 \
        --l1-data-gas 1000000 \
        --l1-data-gas-price-raw 1 \
        --l2-gas 1000000 \
        --l2-gas-price-raw 1 \
        --nonce "$i" \
        >/dev/null 2>&1

    if [ $((i % 100)) -eq 0 ]; then
        echo "Account #$INDEX sent $i transactions"
    fi
done
