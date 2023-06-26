# template-copy

**Tiny scaffolding engine**

> Like cp, but with Lodash templates

## Install

    npm i template-copy

## Usage

```js
const t = require('template-copy');

t('index.html templates/ ~/.config/templates/ava/ test/');
  .catch((err) => {
    console.log('ERR', err);
  })
  .then(() => {
    console.log('done');
  });
```

## Description

File copy is done using Streams, with `lodash.template` parsing files if they
have template placeholders `{{ ... }}`.

Handlebars template are executed in the context of the following object:

```js
Object.assign({}, env, opts)

// where `env` and `opts` have the following structure

{
  env: {
    PATH: '...',
    ...
  },

  opts: {
    debug: true,
    name: 'Foobar'
  }
}
```

The templates context is a merged version of various sources, with the
following order of precedence:

- opts    - Command line flags as parsed by minimist
- env     - `process.env` variables begining with `hcp_`
- prompts - Generated prompts, see below

## Prompts

Handlebars templates can have any number of placeholders. Variables are either
available in the context object, or automatically prompted for the user to
enter a value.

Skipping a prompt is then available with `--name Value`.

To enable it, inquirer must be installed and available in `node_modules`.

    npm i inquirer -D

If not installed, prompts are not generated.

## JSON

JSON files are merged together with destination, if it already exists.

## Related

- [hcp](https://github.com/mklabs/handlebars-copy) - CLI command power by template-copy
