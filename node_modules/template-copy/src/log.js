'use strict';

var chalk = require('chalk');

var _require = require('util');

var format = _require.format;

var _require2 = require('log-symbols');

var _error = _require2.error;
var _info = _require2.info;
var _success = _require2.success;
var warning = _require2.warning;


var HEADING = '';

var log = module.exports = {
  success: function success() {
    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    var msg = format.apply(null, args);
    var level = chalk.green('info');
    var symbol = _success;
    this._write(log.heading + ' ' + symbol + ' ' + level + ' ' + msg);
    return this;
  },
  warn: function warn() {
    for (var _len2 = arguments.length, args = Array(_len2), _key2 = 0; _key2 < _len2; _key2++) {
      args[_key2] = arguments[_key2];
    }

    var msg = format.apply(null, args);
    var level = chalk.yellow('warn');
    var symbol = warning;
    this._write(log.heading + ' ' + symbol + ' ' + level + ' ' + msg);
    return this;
  },
  info: function info() {
    for (var _len3 = arguments.length, args = Array(_len3), _key3 = 0; _key3 < _len3; _key3++) {
      args[_key3] = arguments[_key3];
    }

    var msg = format.apply(null, args);
    var level = chalk.green('info');
    var symbol = _info;
    this._write(log.heading + ' ' + symbol + ' ' + level + ' ' + msg);
    return this;
  },
  error: function error() {
    for (var _len4 = arguments.length, args = Array(_len4), _key4 = 0; _key4 < _len4; _key4++) {
      args[_key4] = arguments[_key4];
    }

    var msg = format.apply(null, args);
    var level = chalk.red('ERR ');
    var symbol = _error;
    this._write(log.heading + ' ' + symbol + ' ' + level + ' ' + msg);
    return this;
  },
  _write: function _write(str) {
    var stream = this.stream || process.stderr;
    stream.write(str + '\n');
    return this;
  }
};

log.heading = HEADING;