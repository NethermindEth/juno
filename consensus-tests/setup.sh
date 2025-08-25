#!/bin/bash

DATA_DIR=${DATA_DIR:-"consensus-tests"}
INDEX="$1"
TARGET="$((INDEX % 4))"
export STARKNET_RPC="http://node$TARGET:6060"

echo "Generate a new keypair for account #$INDEX"
KEYPAIR=$(starkli signer gen-keypair)
export PRIVATE_KEY=$(starkli signer gen-keypair | awk '/Private key/ {print $4}')

echo "Private key #$INDEX: $PRIVATE_KEY"

echo "Init account #$INDEX"
ADDRESS=$(starkli account oz init "$DATA_DIR/$INDEX.json" --force --private-key "$PRIVATE_KEY" --class-hash "0x61dac032f228abef9c6626f995015233097ae253a7f72d68552db02f2971b8f" 2>&1 | tee /dev/stderr  | tr -d ' ' | grep "^0x")

echo "Fund ETH to account #$INDEX"
until starkli invoke eth transfer "$ADDRESS" u256:100000000000000000 --account "$DATA_DIR/root.json" --private-key "$ROOT_PRIVATE_KEY" --watch --poll-interval 500; do
    echo "Failed to fund ETH to account #$INDEX, retrying..."
    sleep 1
done

echo "Fund STRK to account #$INDEX"
until starkli invoke strk transfer "$ADDRESS" u256:100000000000000000 --account "$DATA_DIR/root.json" --private-key "$ROOT_PRIVATE_KEY" --watch --poll-interval 500; do
    echo "Failed to fund STRK to account #$INDEX, retrying..."
    sleep 1
done

echo "Deploy account #$INDEX"
until yes "" | starkli account deploy "$DATA_DIR/$INDEX.json" --private-key "$PRIVATE_KEY" --poll-interval 500; do
    echo "Failed to deploy account #$INDEX, retrying..."
    sleep 1
done

echo "Write private key to be reused"
echo "$PRIVATE_KEY" > "$DATA_DIR/$INDEX.key"
