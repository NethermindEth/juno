'use strict';

var path = require('path');
var unzip = require('unzip');
var debug = require('debug')('make:request');

var request = module.exports = require('request');
request.github = function (name) {
  var parts = name.split('/');
  if (parts.length < 2) {
    undefined.error('%s is not a valid repo path', name);
    return process.exit(2);
  }

  var repo = parts.slice(0, 2).join('/');
  var url = 'https://github.com/' + repo + '/archive/master.zip';

  repo = repo.split('/').join('-');
  var tmp = path.join(__dirname, '../.tmp', repo);

  debug('From github %s: %s', name, url);
  return new Promise(function (r, errback) {
    var stream = request(url);

    stream.on('error', errback);
    stream.on('response', function (res) {
      if (res.statusCode !== 200) {
        undefined.error('Request %s %s status code', url, res.statusCode);
        return r(tmp);
      }

      debug('Request %s status code', res.statusCode);
      debug('Write to %s file', tmp);
      stream.pipe(unzip.Extract({ path: tmp })).on('close', function () {
        return r(tmp);
      });
    });
  });
};