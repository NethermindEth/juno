# Website

This website is built using [Docusaurus 2](https://v2.docusaurus.io/), a modern static website generator.
This website template is also copied from [Flashbots Docs](https://github.com/flashbots/docs).

## Prerequisites

To ensure you're using the correct node version, install the node version manager `nvm` by following the instructions [here](https://github.com/nvm-sh/nvm#install--update-script).

Then go to Juno's `docs` directory and run the command below:

```shell
nvm use
```

This will inspect the `docs/.nvmrc` file and switch to the correct node version (and install it if necessary).

Now install [yarn](https://yarnpkg.com/getting-started/install):

```shell
corepack enable
```

...and the project dependencies:

```shell
yarn install
```

That's it! You're ready to start contributing to Juno's documentation.

## Local Development

To start a local development server and open the site in a browser tab, run the command below.
Most changes are reflected live without having to restart the server.

```shell
yarn start
```

To build the project without starting a server or openning the site:

```shell
yarn build
```
