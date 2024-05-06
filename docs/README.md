# Juno Documentation

The Juno Documentation can be accessed at <https://juno.nethermind.io/> and is built using [Docusaurus](https://docusaurus.io/), a modern static website generator.

## Installation

```bash
npm install
```

## Local Development

```bash
npm run start # Run 'node generate-juno-config.js' in the root directory to generate the Juno configuration options table
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

## Deployment

```bash
npm run build # Runs 'node generate-juno-config.js' and then 'docusaurus build'
# The 'generate-juno-config.js' script extracts Juno's configuration details and generates a table for the configure.md page
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

Deployment is handled using GitHub pages.

### Troubleshooting

Docusaurus depends heavily on caching to improve site performance. If you make changes that do not appear in the website, try clearing the cache by running:

```bash
npm run clear
```

### Versioning

To generate a new version of the documentation, run the following command:

```bash
npm run docusaurus docs:version <VERSION NAME>
```

To view versions of the documentation that have not yet been released, visit `http://localhost:3000/next/`.
