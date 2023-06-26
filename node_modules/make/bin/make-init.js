#!/usr/bin/env node
const { Template } = require('..');

var run = new Template({
  namespace: 'make:init'
});

run.init();
