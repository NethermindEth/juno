---
title: Frequently Asked Questions
---

# Frequently Asked Questions :question:

<details>
  <summary>What is Juno?</summary>

Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers.

</details>

<details>
  <summary>How can I run Juno?</summary>

Check out the [Running Juno](running-juno) guide to learn the simplest and fastest ways to run a Juno node. You can also check the [Running Juno on GCP](running-on-gcp) guide to learn how to run Juno on GCP.

</details>

<details>
  <summary>What are the hardware requirements for running Juno?</summary>

We recommend running Juno with at least 4GB of RAM and 250GB of SSD storage. Check out the [Hardware Requirements](hardware-requirements) for more information.

</details>

<details>
  <summary>How can I configure my Juno node?</summary>

You can configure Juno using [command line parameters](configuring#command-line-params), [environment variables](configuring#environment-variables), and a [YAML configuration file](configuring#configuration-file). Check out the [Configuring Juno](configuring) guide to learn their usage and precedence.

</details>

<details>
  <summary>How can I update my Juno node?</summary>

Check out the [Updating Juno](updating) guide for instructions on updating your node to the latest version.

</details>

<details>
  <summary>How can I interact with my Juno node?</summary>

You can interact with a running Juno node using the [JSON-RPC](json-rpc) and [WebSocket](websocket) interfaces.

</details>

<details>
  <summary>How can I monitor my Juno node?</summary>

Juno captures metrics data using [Prometheus](https://prometheus.io), and you can visualise them using [Grafana](https://grafana.com). Check out the [Monitoring Juno](monitoring) guide to get started.

</details>

<details>
  <summary>Do node operators receive any rewards, or is participation solely to support the network?</summary>

Presently, running a node does not come with direct rewards; its primary purpose is contributing to the network's functionality and stability. However, operating a node provides valuable educational benefits and deepens your knowledge of the network's operation.

</details>

<details>
  <summary>How can I view Juno logs from Docker?</summary>

You can view logs from the Docker container using the following command:

```bash
docker logs -f juno
```

</details>

<details>
  <summary>How can I get real-time updates of new blocks?</summary>

The [WebSocket](websocket#subscribe-to-newly-created-blocks) interface provides a `juno_subscribeNewHeads` method that emits an event when new blocks are added to the blockchain.

</details>

<details>
  <summary>Does Juno provide snapshots to sync with Starknet quickly?</summary>

Yes, Juno provides snapshots for both the Starknet Mainnet and Sepolia networks. Check out the [Database Snapshots](snapshots) guide to get started.

</details>

<details>
  <summary>How can I contribute to Juno?</summary>

You can contribute to Juno by running a node, starring on GitHub, reporting bugs, and suggesting new features. Check out the [Contributions and Partnerships](/#contributions-and-partnerships) page for more information.

</details>

<details>
  <summary>I noticed a warning in my logs saying: **Failed storing Block \{"err": "unsupported block version"\}**. How should I proceed?</summary>

You can fix this problem by [updating to the latest version](updating) of Juno. Check for updates and install them to maintain compatibility with the latest block versions.

</details>

<details>
  <summary>After updating Juno, I receive **error while migrating DB.** How should I proceed?</summary>

This error suggests your database is corrupted, likely due to the node being interrupted during migration. This can occur if there are insufficient system resources, such as RAM, to finish the process. The only solution is to resynchronise the node from the beginning. To avoid this issue in the future, ensure your system has adequate resources and that the node remains uninterrupted during upgrades.

</details>

<details>
  <summary>I receive **Error: unable to verify latest block hash; are the database and --network option compatible?** while running Juno. How should I proceed?</summary>

To resolve this issue, ensure that the `eth-node` configuration aligns with the `network` option for the Starknet network.

</details>

<details>
  <summary>I receive **process \<PID\> killed** and **./build/juno: invalid signature (code or signature have been modified)** while running the binary on macOS. How should I proceed?</summary>

You need to re-sign the binary to resolve this issue using the following command:

```bash
codesign --sign - ./build/juno
```

</details>
