# Become a Staking Validator

Staking on Starknet provides an opportunity to contribute to network security and earn rewards by becoming a validator. To learn more about the validator process, check out the [Becoming a Validator](https://docs.starknet.io/staking/entering-staking/) guide.

## Prerequisites

- **STRK Tokens**: A sufficient balance for staking.
- **Node Setup**: Juno running as your Starknet-compatible implementation.
- **Starknet Wallet**: Use a wallet like [Braavos](https://braavos.app/wallet-features/ledger-on-braavos/) or [Argent](https://www.argent.xyz/blog/how-to-use-your-hardware-wallet-with-argent).
- **Block Explorer**: Tools like [Voyager](https://voyager.online) for contract interactions.

## 1. Set up Juno

Juno is a reliable choice for running a Starknet node. Follow the [Running Juno](running-juno) guide to configure Juno using Docker, binaries, or source builds.

## 2. Stake STRK tokens

Register as a validator by staking STRK tokens through the Starknet staking contract. For complete instructions, check out the [Becoming a Validator](https://docs.starknet.io/staking/entering-staking/) guide. The staking process includes:

- **Pre-approving STRK Transfer**: Allow the staking contract to lock your tokens.
- **Calling the `stake` Function**: Register operational and reward addresses, set commission rates, and enable pooling if desired.

## 3. Finalising your validator

Once Juno is running and your STRK tokens are staked:

1. Monitor your validator's status via dashboards like [Voyager](https://voyager.online/).
2. Stay updated for future network requirements or configurations.

You're now a staking validator! With your node running and tokens staked, you're supporting Starknet's security and earning rewards. Most operations are managed by the network, though future updates may need additional setup.
