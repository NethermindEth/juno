require('dotenv').config();
const math = require('remark-math');
const katex = require('rehype-katex');
const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');

// With JSDoc @type annotations, IDEs can provide config autocompletion
/** @type {import('@docusaurus/types').DocusaurusConfig} */
(
    module.exports = {
        title: 'Juno docs',
        tagline: 'Juno docs',
        url: "https://my-web.com/",
        onBrokenLinks: 'throw',
        onBrokenMarkdownLinks: 'warn',
        favicon: 'img/juno.jpg',
        organizationName: 'NethermindEth',
        projectName: 'juno',
        baseUrl: "/",
        stylesheets: [
            {
                href: 'https://cdn.jsdelivr.net/npm/katex@0.12.0/dist/katex.min.css',
                type: 'text/css',
                integrity:
                    'sha384-AfEj0r4/OFrOo5t7NnNe46zW/tFgW6x/bCJG8FqQCEo3+Aro6EYUG4+cU+KJWu/X',
                crossorigin: 'anonymous',
            },
        ],
        themeConfig:
        /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
            ({
                algolia: {
                    apiKey: '************************',
                    indexName: 'juno',
                    // Optional: see doc section below
                    appId: '**********',
                },
                prism: {
                    theme: lightCodeTheme,
                    darkTheme: darkCodeTheme,
                    additionalLanguages: ['solidity'],
                },
                hideableSidebar: true,
                navbar: {
                    title: 'Juno Docs',
                    logo: {
                        alt: 'Juno Logo',
                        src: 'img/juno_small.jpg',
                    },
                    items: [
                        {
                            href: 'https://github.com/NethermindEth/juno/',
                            label: 'GitHub',
                            position: 'right',
                        },
                    ],
                },
            }),
        presets: [
            [
                '@docusaurus/preset-classic',
                /** @type {import('@docusaurus/preset-classic').Options} */
                ({
                    docs: {
                        sidebarPath: require.resolve('./docs/sidebars.js'),
                        // Please change this to your repo.
                        routeBasePath: '/',
                        remarkPlugins: [math],
                        rehypePlugins: [katex],
                    },
                    theme: {
                        customCss: require.resolve('./src/css/custom.css'),
                    },
                }),
            ],
        ],
        plugins: [
            [
                'docusaurus2-dotenv',
                {
                    systemvars: true,
                },
            ],
        ],
    }
);
