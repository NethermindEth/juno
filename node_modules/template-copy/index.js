var Template = require('./src/template');

module.exports = t;
t.Template = Template;

function t (argv, opts) {
  argv = Array.isArray(argv) ? argv
    : typeof argv === 'string' ? argv.split(' ')
    : process.argv.slice(2);

  try {
    let t = new Template(argv, opts);
    return t.init(argv);
  } catch (e) {
    console.error('ERR: Something wrong happend while creating Template instance', e);
    throw e;
  }
};
