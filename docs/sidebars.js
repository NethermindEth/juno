// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  main: [
    "intro",
    {
      type: "category",
      label: "Installation and Setup",
      collapsed: false,
      items: [
        "hardware-requirements",
        "running-juno",
        "configuring",
        "updating",
        "tuning",
        "running-on-gcp",
        "plugins",
      ],
    },
    {
      type: "category",
      label: "Interacting with Juno",
      collapsed: false,
      items: [
        "json-rpc",
        "websocket",
        {
          type: "html",
          value:
            '<a href="https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false" target="_blank" rel="noopener noreferrer" class="menu__link external-link">Starknet Node API Endpoints&nbsp;<svg width="13.5" height="13.5" aria-hidden="true" viewBox="0 0 24 24" class="iconExternalLink_node_modules-@docusaurus-theme-classic-lib-theme-Icon-ExternalLink-styles-module"><path fill="currentColor" d="M21 13v10h-21v-19h12v2h-10v15h17v-8h2zm3-12h-10.988l4.035 4-6.977 7.07 2.828 2.828 6.977-7.07 4.125 4.172v-11z"></path></svg>',
        },
        {
          type: "html",
          value:
            '<a href="https://rpc-request-builder.voyager.online/" target="_blank" rel="noopener noreferrer" class="menu__link external-link">Starknet RPC Request Builder&nbsp;<svg width="13.5" height="13.5" aria-hidden="true" viewBox="0 0 24 24" class="iconExternalLink_node_modules-@docusaurus-theme-classic-lib-theme-Icon-ExternalLink-styles-module"><path fill="currentColor" d="M21 13v10h-21v-19h12v2h-10v15h17v-8h2zm3-12h-10.988l4.035 4-6.977 7.07 2.828 2.828 6.977-7.07 4.125 4.172v-11z"></path></svg>',
        },
      ],
    },
    "staking-validator",
    "monitoring",
    "snapshots",
    "faq",
  ],
};

module.exports = sidebars;
