---
title: Frequently Asked Questions
---

<details>
  <summary>Do node operators receive any rewards, or is participation solely to support the network?</summary>

Presently, running a node does not come with direct rewards; its primary purpose is contributing to the network's functionality and stability. However, operating a node provides valuable educational benefits and deepens your knowledge of the network's operation.

</details>

<details>
  <summary>I noticed a warning in my logs saying: 'Failed storing Block \{"err": "unsupported block version"\}'. How should I proceed?</summary>

You can fix this problem by [updating to the latest version](updating.md) of Juno. Check for updates and install them to maintain compatibility with the latest block versions.

</details>

<details>
  <summary>After updating Juno, I receive "error while migrating DB." How should I proceed?</summary>

This error suggests your database is corrupted, likely due to the node being interrupted during migration. This can occur if there are insufficient system resources, such as RAM, to finish the process. The only solution is to resynchronize the node from the beginning. To avoid this issue in the future, ensure your system has adequate resources and that the node remains uninterrupted during upgrades.

</details>

<details>
  <summary>I received "Error: unable to verify latest block hash; are the database and --network option compatible?" while running Juno. How should I proceed?</summary>

To resolve this issue, ensure that the `eth-node` configuration aligns with the `network` option for the Starknet network.

</details>
