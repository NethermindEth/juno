---
title: Deploy on GCP
---

# Running Juno on GCP :cloud:

To run Juno on the Google Cloud Platform (GCP), you can use the Starknet RPC Virtual Machine (VM) developed by Nethermind.

## 1. Install the Starknet RPC Node

Head to the [Google Marketplace](https://console.cloud.google.com/marketplace/browse?q=Starknet%20RPC%20Node) and search for **"Starknet RPC Node"**. Then, click the **"GET STARTED"** button to begin the deployment process.

![Starknet RPC Node overview](/img/installing_on_gcp/overview.png)

## 2. Configure the Juno client

Choose the configuration settings for the Juno client and click the **"DEPLOY"** button.

![Starknet RPC Node configuration](/img/installing_on_gcp/config.png)

## 3. Post-configuration and testing

![Starknet RPC Node testing](/img/installing_on_gcp/testing.png)

## 4. Enable Juno during startup

1. Click on the name of the newly created VM instance to view its details.
2. Click the **"Edit"** button.
3. Head to the **"Automation"** section and enter the following startup script:
   ```bash
   #! /bin/bash
   sudo /usr/local/bin/run_juno.sh
   ```
4. Click the **"Save"** button.
5. Restart the VM.

## 5. Interact with the Juno node

You can interact with Juno using its [JSON-RPC Interface](json-rpc). Here's an example to check the availability of Juno:

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

<Tabs>
<TabItem value="request" label="Request">

```bash
curl --location 'http://localhost:6060' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "juno_version",
    "params": [],
    "id": 1
}'
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": "v0.11.7",
  "id": 1
}
```

</TabItem>
</Tabs>

:::tip
To learn how to configure Juno, check out the [Configuring Juno](configuring) guide.
:::
