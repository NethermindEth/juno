
const cli = require('gentle-cli');
const { join, resolve } = require('path');
const { readdirSync: dir } = require('fs');
const assert = require('assert');

describe('bake init', () => {

  let bake = (cmd) => {
    return cli({ cwd: join(__dirname, 'examples') })
      .use('node ' + join(__dirname, '../bin/make.js') + ' ' + cmd);
  };

  it('bake init', (done) => {
    bake('init')
      .expect(2, () => {
        let files = dir(join(__dirname, 'examples'));
        assert.equal(files.length, 8);

        var json = require('./examples/package.json');
        assert.equal(json.name, '');
        assert.equal(json.scripts.test, 'mocha -R spec');
        done();
      })
  });
});
