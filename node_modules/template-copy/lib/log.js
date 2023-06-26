const chalk  = require('chalk');
const { format } = require('util');
const { error, info, success, warning } = require('log-symbols');

const HEADING = '';

let log = module.exports = {
  success(...args) {
    let msg = format.apply(null, args);
    let level = chalk.green('info');
    let symbol = success;
    this._write(`${log.heading} ${symbol} ${level} ${msg}`);
    return this;
  },

  warn(...args) {
    let msg = format.apply(null, args);
    let level = chalk.yellow('warn');
    let symbol = warning;
    this._write(`${log.heading} ${symbol} ${level} ${msg}`);
    return this;
  },

  info(...args) {
    let msg = format.apply(null, args);
    let level = chalk.green('info');
    let symbol = info;
    this._write(`${log.heading} ${symbol} ${level} ${msg}`);
    return this;
  },

  error(...args) {
    let msg = format.apply(null, args);
    let level = chalk.red('ERR ');
    let symbol = error;
    this._write(`${log.heading} ${symbol} ${level} ${msg}`);
    return this;
  },

  _write(str) {
    let stream = this.stream || process.stderr;
    stream.write(str + '\n');
    return this;
  }
};

log.heading = HEADING;
