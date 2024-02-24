---
slug: /installing-on-gcp
sidebar_position: 6
title: Installing on the GCP
---

To install the Starknet RPC Node on the Google Cloud Platform, you can use the Juno RPC virtual machine developed by Nethermind.

**Juno** is a golang [Starknet](https://starknet.io/) node implementation by [Nethermind](https://nethermind.io/) with the aim of decentralizing Starknet.

## Installing Starkent RPC Juno Node VM

To quickly set up a Starkent RPC Juno Node VM environment on the Google Cloud Platform, follow these steps:

1. **Search “Starknet RPC Node” in [Google Marketplace](https://console.cloud.google.com/marketplace) and click the LAUNCH button to start the deployment process.**
   
   ![step1](/img/installing_on_gcp/step1.png)

2. **Select the configuration for the Juno client and click the DEPLOY button.**
   
   ![step2](/img/installing_on_gcp/step2.png)
   
3. **Post-Configuration and testing after deployment.**
   
   ![step3](/img/installing_on_gcp/step3.png)
   
4. **Enable Juno Auto Start During Startup**
   1. Click the newly created VM instance name to view the detail.
   2. Click the Edit button.
   3. Go to the "Automation" section to input the startup script as below.
      ```bash
      #! /bin/bash
      sudo /usr/local/bin/run_juno.sh
      ```
   4. Click the Save button.
   5. Restart the VM.


5. **Use Your Starknet RPC Juno Node**
   
   You can use the Juno node and access it through Rest APIs. The following is an example to verify Juno availability.
   
   ```bash
   curl --location 'http://IP_Address:6060' \
   --header 'Content-Type: application/json' \
   --data '{"jsonrpc":"2.0","method":"juno_version","params":[],"id":1}'
   ```

   The expected result is like this.

   ```{
      "jsonrpc": "2.0",
      "result": "v0.9.3",
      "id": 1
   }
   ```
   You can find more details from https://github.com/NethermindEth/juno