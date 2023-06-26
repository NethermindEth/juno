const minimist   = require('minimist');
const events     = require('events');
const debug      = require('debug');
const log        = require('./log');
const logsymbols = require('log-symbols');
const fsutil     = require('./util');
const { spawn }  = require('child_process');

const PADDING = 20;

// CLI
//
// This class exposes various utilities for parsing options and logging purpose.
//
// - this.argv  - minimist result from options.argv or process.argv.slice(2)
// - this.debug - debug module logger, enabled with -d flag
// - this.alias - if defined, is used to parse arguments with minimist
// - this.env   - options.env or a clone of process.env
// - this.start - Timestamp marking instance creation, namely used to report
//                build time.
//
// And these static methods:
//
// - CLI.fail - to invoke with an error, log the error with npmlog error level
// - CLI.end  - Log options.success message with elapsed time as a parameter
//
// Options:
//
// - this.options.namespace - Define the debug namespace (eg. require('debug')(namespace)).
//                            Default: make:cli
// - this.options.success   - Success message to print with end()
export default class CLI extends events.EventEmitter {

  get example() { return ''; }

  // Used to parse arguments with minimist
  get alias() {
    return {
      h: 'help',
      v: 'version',
      d: 'debug'
    };
  }

  // Used to generate the help output
  get flags() {
    return {
      help: 'Show this help output',
      version: 'Show package version'
    };
  }

  constructor(opts = {}) {
    super();
    this.start = Date.now();

    this.options = opts;
    this.options.namespace = this.options.namespace || 'make:cli';
    this.options.success = this.options.success || ('Build success in %sms');

    this.options.name = this.options.name || process.argv[1].split('/').slice(-1)[0];

    this.argv = this.parse(this.options.argv);
    this.args = this.argv._.concat();
    this.env = this.options.env || Object.assign({}, process.env);

    if (this.options.debug || this.argv.debug) {
      debug.enable(this.options.namespace);
    }

    this.debug = debug(this.options.namespace);
  }

  parse(argv = process.argv.slice(2), alias = this.alias) {
    return minimist(argv, { alias });
  }

  exec(recipe, opts = { env: this.env, stdio: 'inherit' }) {
    return new Promise((r, errback) => {
      // coming from https://github.com/npm/npm/blob/master/lib/utils/lifecycle.js#L222
      let sh = 'sh';
      let flags = '-c';

      if (process.platform === 'win32') {
        sh = process.env.comspec || 'cmd';
        flags = '/d /s /c';
        opts.windowsVerbatimArguments = true;
      }

      let args = [flags, recipe];
      this.debug('exec:', sh, flags, recipe);
      this.silly('env:', opts.env);
      spawn(sh, args, opts)
        .on('error', errback)
        .on('close', (code) => {
          if (code !== 0) {
            this.error(recipe);
            return errback(new Error('Recipe exited with code %d', code));
          }

          r();
        });
    });
  }

  end(cb) {
    var time = Date.now() - this.start;
    return CLI.end(this.options.success, time, cb);
  }

  help(targets = {}) {
    let targetList = '';
    let leftpad = this.options.leftpad || '    ';
    if (Object.keys(targets).length) targetList += ' Targets:\n';

    var keys = Object.keys(targets);
    targetList += keys.map((t) => {
      return leftpad + t + this.pad(t) + 'Run target ' + t;
    }).join('\n');

    var options = '';
    if (this.flags) {
      options += 'Options:\n';
      options += Object.keys(this.flags).map((flag) => {
        return leftpad + '--' + flag + this.pad('--' + flag) + this.flags[flag];
      }).join('\n');
    }

    let opts = {
      example: this.example || this.options.example,
      name: this.options.name,
      commands: targetList,
      more: this.more,
      options,
    };

    return CLI.help(opts);
  }

  pad(str, padding = PADDING) {
    let len = padding - str.length;
    return new Array(len <= 1 ? 2 : len).join(' ');
  }

  // Help output
  //
  // Options:
  //
  // - name     - Used in the generated example (ex: $ name --help)
  // - example  - Used in the generated example instead of the default one
  // - options  - Used in the generated example instead of the default one
  static help(options = {}) {
    options.name = options.name || '';
    options.example = options.example || (options.name + ' --help');

    console.log(`
  $ ${options.example}

  ${options.options}`);

    if (options.commands) console.log('\n', options.commands);
    if (options.more) console.log(options.more);

    console.log();
  }

  static fail(e, exit) {
    log.error.apply(log, arguments);
    if (exit) process.exit(isNaN(exit) ? 1 : exit);
  }

  static end(message, time, cb) {
    log.info(message, time);
    cb && cb();
  }


}

CLI.PADDING = PADDING;

Object.assign(CLI.prototype, log);
Object.assign(CLI.prototype, fsutil);
