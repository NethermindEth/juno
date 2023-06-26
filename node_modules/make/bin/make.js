#!/usr/bin/env node
const fs        = require('fs');
const path      = require('path');
const debug     = require('debug')('make');
const which     = require('which');

const { existsSync: exists } = fs;
const { spawn } = require('child_process');
const { format } = require('util');

const { CLI, Make } = require('..');
const { verbose, info, warn, error  } = require('../src/log');
const { fail } = CLI;

const assign = Object.assign || require('object-assign');
const separator = process.platform === 'win32' ? ';' : ':';
const PATH = process.platform === 'win32' ? 'Path' : ':';

let env = assign({}, process.env);
env[PATH] = path.resolve('./node_modules/.bin') + separator + process.env.PATH;

const makefile = exists('Bakefile') ? 'Bakefile' :
  exists('Makefile') ? 'Makefile' :
  '';

let make = new Make(makefile, {
  env: env
});

make.on(Make.UNKNOWN_TARGET, (target, targets) => {
  var cmd = 'make-' + target;
  which(cmd, (err, filename) => {
    if (err) {
      fail(err.message);
      return make.help(targets);
    }

    var args = process.argv.slice(3);

    info('Go for it', filename, args);
    var sh = spawn(filename, args, {
      stdio: 'inherit',
      env: make.env
    });

    // sh.on('error', error.bind(null));

    sh.on('close', (code) => {
      if (code === 0) return;

      // fail(new Error(format('%s exited with code %d', cmd, code)));
      process.exit(code);
    });
  });
});
