# Website

This website is built using [Docusaurus 2](https://v2.docusaurus.io/), a modern static website generator.
This website template is also copied from [Flashbots Docs](https://github.com/flashbots/docs).

## Prerequisites (Linux)

- `Yarn` installation:

```shell
curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add
echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
sudo apt update
sudo apt install yarn
```

_Please note:_ `sudo apt install yarn` will also install nodejs, however, it may not be the version which you require.
Therefore, you should use `nvm` to install the latest version of node.

- `Nvm` installation:

```shell
sudo apt-get update
sudo apt-get install build-essential checkinstall libssl-dev
curl -o- https://raw.githubusercontent.com/creationix/nvm/v0.32.1/install.sh | bash
  
nvm install --lts
```

_Please note:_ you may have to use the last command every terminal session because the installed version is stored in `~/.nvm` and not `/usr/bin`.

## Installation

First create a copy of the environment file `.env.template` in the root of the codebase and rename it to `.env`.

Then run the following:

```shell
yarn install
```

## Local Development

```shell
yarn start
```

This command starts a local development server and open up a browser window. Most changes are reflected live without having to restart the server.

## Build

```shell
yarn build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

## Deployment

```shell
GIT_USER=<Your GitHub username> USE_SSH=true yarn deploy
```

If you are using GitHub pages for hosting, this command is a convenient way to build the website and push to the `gh-pages` branch.
