// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const lightCodeTheme = require("prism-react-renderer").themes.github;
const darkCodeTheme = require("prism-react-renderer").themes.dracula;

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "Juno",
  tagline: "Decentralising Starknet",
  favicon: "img/favicon.ico",

  // Set the production url of your site here
  url: "https://juno.nethermind.io",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/",

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "NethermindEth", // Usually your GitHub org/user name.
  projectName: "juno", // Usually your repo name.

  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",

  // Even if you don't use internalization, you can use this field to set useful
  // metadata like html lang. For example, if your site is Chinese, you may want
  // to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en-GB",
    locales: ["en-GB"],
  },

  presets: [
    [
      "classic",
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve("./sidebars.js"),
          routeBasePath: "/",
        },
        blog: false,
        theme: {
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ],
  ],

  plugins: [
    [
      "@easyops-cn/docusaurus-search-local",
      {
        docsRouteBasePath: "/",
        removeDefaultStopWordFilter: true,
        highlightSearchTermsOnTargetPage: true,
      },
    ],
    [
      "@docusaurus/plugin-client-redirects",
      {
        redirects: [
          {
            from: "/config",
            to: "/configuring",
          },
          {
            from: "/installing-on-gcp",
            to: "/running-on-gcp",
          },
          {
            from: "/updating_node",
            to: "/updating",
          },
        ],
      },
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      navbar: {
        title: "JUNO",
        logo: {
          alt: "Juno logo",
          src: "img/logo.svg",
        },
        items: [
          {
            type: "docsVersionDropdown",
            position: "right",
          },
          {
            href: "https://github.com/NethermindEth/juno",
            label: "GitHub",
            position: "right",
          },
        ],
      },
      footer: {
        style: "light",
        links: [
          {
            title: "Docs",
            items: [
              {
                label: "Running Juno",
                to: "running-juno",
              },
              {
                label: "Configuring Juno",
                to: "configuring",
              },
              {
                label: "Interacting with Juno",
                to: "json-rpc",
              },
            ],
          },
          {
            title: "Community",
            items: [
              {
                label: "Discord",
                href: "https://discord.gg/SZkKcmmChJ", // #juno channel in Nethermind server.
              },
              {
                label: "X (Twitter)",
                href: "https://x.com/NethermindStark", // NethermindStark account.
              },
              {
                label: "Telegram",
                href: "https://t.me/+LHRF4H8iQ3c5MDY0", // Juno - Starknet Full Node chat.
              },
            ],
          },
          {
            title: "Resources",
            items: [
              {
                label: "GitHub",
                href: "https://github.com/NethermindEth/juno",
              },
              {
                label: "Blog",
                href: "https://www.nethermind.io/blogs",
              },
              {
                label: "Juno Releases",
                href: "https://github.com/NethermindEth/juno/releases/latest",
              },
            ],
          },
        ],
        copyright: `Nethermind © ${new Date().getFullYear()}`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
        additionalLanguages: ["bash", "json", "go", "rust"],
      },
    }),
};

module.exports = config;
