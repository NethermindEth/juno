const cli = require('gentle-cli');
const { join } = require('path');

const NL = process.platform === 'win32' ? '\r\n' : '\n';

describe('make cli', () => {
  let make = (cmd) => {
    return cli()
      .use('node ' + join(__dirname, '../bin/make.js') + ' ' + cmd);
  };

  it('Outputs help', (done) => {
    make('-h')
      .expect('make <target...> [options]')
      .expect('Options:')
      .expect(0)
      .end(done);
  });

  it('make foo', (done) => {
    make('foo')
      .expect(['prefoo', 'blahblah', 'foo'].join(NL))
      .expect(0)
      .end(done);
  });

  it('make all', (done) => {
    make('all')
      .expect(['prefoo', 'blahblah', 'foo', 'foo2', 'blahblah', 'foobar'].join(NL))
      .expect(0)
      .end(done);
  });

  it('make maoow - Outputs help on UNKNOWN_TARGET', (done) => {
    make('maoow')
      .expect('make <target...> [options]')
      .expect('Options:')
      .expect(0)
      .end(done);
  });
});
