// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  main: [
    "introduction",
    {
      type: "category",
      label: "Installation and Setup",
      collapsed: false,
      items: [
        "hardware-requirements",
        "running-juno",
        "running-on-gcp",
        "configuring",
        "updating",
      ],
    },
    {
      type: "category",
      label: "Interacting with Juno",
      collapsed: false,
      items: ["json-rpc", "grpc", "websocket"],
    },
    "monitoring",
    "snapshots",
    "faq",
  ],
};

module.exports = sidebars;
