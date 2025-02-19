"use strict";(self.webpackChunkjuno_docs=self.webpackChunkjuno_docs||[]).push([[9568],{4922:e=>{e.exports=JSON.parse('{"pluginId":"default","version":"0.12.4","label":"0.12.4","banner":"unmaintained","badge":true,"noIndex":false,"className":"docs-version-0.12.4","isLast":false,"docsSidebars":{"main":[{"type":"link","label":"Introduction","href":"/0.12.4/","docId":"intro","unlisted":false},{"type":"category","label":"Installation and Setup","collapsed":false,"items":[{"type":"link","label":"Hardware Requirements","href":"/0.12.4/hardware-requirements","docId":"hardware-requirements","unlisted":false},{"type":"link","label":"Running Juno","href":"/0.12.4/running-juno","docId":"running-juno","unlisted":false},{"type":"link","label":"Configuring Juno","href":"/0.12.4/configuring","docId":"configuring","unlisted":false},{"type":"link","label":"Juno Plugins","href":"/0.12.4/plugins","docId":"plugins","unlisted":false},{"type":"link","label":"Running Juno on GCP","href":"/0.12.4/running-on-gcp","docId":"running-on-gcp","unlisted":false},{"type":"link","label":"Updating Juno","href":"/0.12.4/updating","docId":"updating","unlisted":false}],"collapsible":true},{"type":"category","label":"Interacting with Juno","collapsed":false,"items":[{"type":"link","label":"JSON-RPC Interface","href":"/0.12.4/json-rpc","docId":"json-rpc","unlisted":false},{"type":"link","label":"WebSocket Interface","href":"/0.12.4/websocket","docId":"websocket","unlisted":false},{"type":"html","value":"<a href=\\"https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false\\" target=\\"_blank\\" rel=\\"noopener noreferrer\\" class=\\"menu__link external-link\\">Starknet Node API Endpoints&nbsp;<svg width=\\"13.5\\" height=\\"13.5\\" aria-hidden=\\"true\\" viewBox=\\"0 0 24 24\\" class=\\"iconExternalLink_node_modules-@docusaurus-theme-classic-lib-theme-Icon-ExternalLink-styles-module\\"><path fill=\\"currentColor\\" d=\\"M21 13v10h-21v-19h12v2h-10v15h17v-8h2zm3-12h-10.988l4.035 4-6.977 7.07 2.828 2.828 6.977-7.07 4.125 4.172v-11z\\"></path></svg>"},{"type":"html","value":"<a href=\\"https://rpc-request-builder.voyager.online/\\" target=\\"_blank\\" rel=\\"noopener noreferrer\\" class=\\"menu__link external-link\\">Starknet RPC Request Builder&nbsp;<svg width=\\"13.5\\" height=\\"13.5\\" aria-hidden=\\"true\\" viewBox=\\"0 0 24 24\\" class=\\"iconExternalLink_node_modules-@docusaurus-theme-classic-lib-theme-Icon-ExternalLink-styles-module\\"><path fill=\\"currentColor\\" d=\\"M21 13v10h-21v-19h12v2h-10v15h17v-8h2zm3-12h-10.988l4.035 4-6.977 7.07 2.828 2.828 6.977-7.07 4.125 4.172v-11z\\"></path></svg>"}],"collapsible":true},{"type":"link","label":"Become a Staking Validator","href":"/0.12.4/staking-validator","docId":"staking-validator","unlisted":false},{"type":"link","label":"Monitoring Juno","href":"/0.12.4/monitoring","docId":"monitoring","unlisted":false},{"type":"link","label":"Database Snapshots","href":"/0.12.4/snapshots","docId":"snapshots","unlisted":false},{"type":"link","label":"Frequently Asked Questions","href":"/0.12.4/faq","docId":"faq","unlisted":false}]},"docs":{"configuring":{"id":"configuring","title":"Configuring Juno","description":"Juno can be configured using several methods, with the following order of precedence:","sidebar":"main"},"faq":{"id":"faq","title":"Frequently Asked Questions","description":"What is Juno?","sidebar":"main"},"hardware-requirements":{"id":"hardware-requirements","title":"Hardware Requirements","description":"The following specifications outline the hardware required to run a Juno node. These specifications are categorised into minimal and recommended requirements for different usage scenarios.","sidebar":"main"},"intro":{"id":"intro","title":"Introduction","description":"Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers.","sidebar":"main"},"json-rpc":{"id":"json-rpc","title":"JSON-RPC Interface","description":"Interacting with Juno requires sending requests to specific JSON-RPC API methods. Juno supports all of Starknet\'s Node API Endpoints over HTTP and WebSocket.","sidebar":"main"},"monitoring":{"id":"monitoring","title":"Monitoring Juno","description":"Juno uses Prometheus to monitor and collect metrics data, which you can visualise with Grafana. You can use these insights to understand what is happening when Juno is running.","sidebar":"main"},"plugins":{"id":"plugins","title":"Juno Plugins","description":"Juno supports plugins that satisfy the JunoPlugin interface, enabling developers to extend and customize Juno\'s behaviour and functionality by dynamically loading external plugins during runtime.","sidebar":"main"},"running-juno":{"id":"running-juno","title":"Running Juno","description":"You can run a Juno node using several methods:","sidebar":"main"},"running-on-gcp":{"id":"running-on-gcp","title":"Running Juno on GCP","description":"To run Juno on the Google Cloud Platform (GCP), you can use the Starknet RPC Virtual Machine (VM) developed by Nethermind.","sidebar":"main"},"running-p2p":{"id":"running-p2p","title":"Running a Juno P2P Node","description":"Juno can be run as a peer-to-peer node for decentralised data synchronisation and to enhance the resilience and reliability of the Starknet network. Check out the\xa0Juno peer-to-peer launch guide to learn how it works."},"snapshots":{"id":"snapshots","title":"Database Snapshots","description":"You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node.","sidebar":"main"},"staking-validator":{"id":"staking-validator","title":"Become a Staking Validator","description":"Staking on Starknet provides an opportunity to contribute to network security and earn rewards by becoming a validator. Check out the Becoming a Validator guide to learn more about the validator process.","sidebar":"main"},"updating":{"id":"updating","title":"Updating Juno","description":"It is important to run the latest version of Juno as each update brings new features, security patches, and improvements over previous versions. Follow these steps to update Juno:","sidebar":"main"},"websocket":{"id":"websocket","title":"WebSocket Interface","description":"Juno provides a WebSocket RPC interface that supports all of Starknet\'s JSON-RPC API endpoints and allows you to subscribe to newly created blocks.","sidebar":"main"}}}')}}]);