const path    = require('path');
const unzip   = require('unzip');
const debug   = require('debug')('make:request');

let request = module.exports = require('request');
request.github = (name) => {
  let parts = name.split('/');
  if (parts.length < 2) {
    this.error('%s is not a valid repo path', name);
    return process.exit(2);
  }

  let repo = parts.slice(0, 2).join('/');
  let url = `https://github.com/${repo}/archive/master.zip`;

  repo = repo.split('/').join('-');
  let tmp = path.join(__dirname, '../.tmp', repo);

  debug('From github %s: %s', name, url);
  return new Promise((r, errback) => {
    var stream = request(url);

    stream.on('error', errback);
    stream.on('response', (res) => {
      if (res.statusCode !== 200) {
        this.error('Request %s %s status code', url, res.statusCode);
        return r(tmp);
      }

      debug('Request %s status code', res.statusCode);
      debug('Write to %s file', tmp);
      stream.pipe(unzip.Extract({ path: tmp }))
        .on('close', () => r(tmp));
    });
  });
};
