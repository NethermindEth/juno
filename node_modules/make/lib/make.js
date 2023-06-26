const CLI      = require('./cli');
const Template = require('./template');
const parse    = require('./parser');
const cp       = require('template-copy');
const path     = require('path');

const UNKNOWN_TARGET = 'target:unknown';

export default class Make extends CLI {

  // Used to generate the help output
  get example () {
    return 'make <target...> [options]';
  }

  get more () {
    return new Template().more;
  }

  get flags () {
    return {
      help: 'Show this help output',
      version: 'Show package version',
      debug: 'Enable extended log output'
    };
  }

  get alias () {
    return {
      h: 'help',
      v: 'version',
      d: 'debug'
    };
  }

  constructor (filename, opts = {}) {
    super(opts);

    if (this.argv.help && !filename) {
      this.help();
      return;
    }

    this.debug('Make init CLI with %s options', Object.keys(opts).join(' '));

    this.makefile = filename;

    this.target = this.args.shift();

    if (this.target === 'init') {
      this.generate(this.args.shift(), this.args);
      return;
    }

    if (!filename) {
      Make.fail('Missing Makefile / Bakefile', filename);
      this.info('Run "make init" to generate a Makefile.');
      return;
    }

    process.nextTick(this.init.bind(this));
  }

  init () {
    let argv = this.argv;
    if (argv.help && !this.makefile) {
      return this.help();
    }

    if (argv.version) {
      console.log(require('../package.json').version);
      return this;
    }

    this.file = this.read(this.makefile);
    this.result = parse(this.file);

    this.targets = this.result.targets;
    this.variables = this.result.variables;

    let first = this.target || 'all';
    if (!this.targets[first]) {
      this.debug('No target %s', first);
      if (first === 'all') return this.help(this.targets);
      return this.noTarget(first);
    }

    if (argv.help) {
      return this.help(this.targets);
    }

    let args = this.argv._;
    if (first === 'all' && !args.length) args = ['all'];

    // Run!
    return this.run(first, args);
  }

  run (target, targets) {
    this.debug('Run %s targets', targets.join(' '));
    var argv = targets.concat();

    return new Promise((r, errback) => {
      (function next (name) {
        if (!name) return this.end(r);

        this.executeTarget(name)
          .then(() => {
            next.call(this, argv.shift());
          })
          .catch((err) => {
            Make.fail(argv.debug ? err : err.message);
            errback(err);
          });
      }).call(this, argv.shift());
    });
  }

  noTarget (target) {
    this.debug('Emit %s', UNKNOWN_TARGET);
    let handler = this.emit(UNKNOWN_TARGET, target, this.targets);
    this.debug('Handled', handler);

    return new Promise((r, errback) => {
      if (handler) return errback();

      this.help(this.targets);
      return errback(new Error('No target matching "' + target + '"'));
    });
  }

  executeTarget (target) {
    return new Promise((r, errback) => {
      if (!this.targets[target]) {
        return this.noTarget(target);
      }

      this.info('Invoking %s target', target);
      var name = this.targets[target];
      return this.executeRecipe(name, target)
        .then(r)
        .catch(errback);
    });
  }

  executeRecipe (target, name) {
    return new Promise((r, errback) => {
      var prerequities = target.prerequities;

      // deps on this recipe, execute rules right away
      if (!prerequities.length) return this.executeRules(target)
        .then(r)
        .catch(errback);

      // found prereq, execute them before executing rules
      this.debug('Prerequities "%s" for target %s', prerequities.join(' '), name);
      return this.executePrereq(target)
        .then(() => {
          return this.executeRules(target)
            .then(r)
            .catch(errback);
        })
        .catch(errback);
    });
  }

  executePrereq (target) {
    return new Promise((r, errback) => {
      var prerequities = target.prerequities;

      // Before executing this recipe, execute any prerequities first
      (function nextPrereq (pre) {
        if (!pre) return r();

        this.executeTarget(pre)
          .catch(errback)
          .then(() => {
            nextPrereq.call(this, prerequities.shift());
          });
      }).call(this, prerequities.shift());
    });
  }

  executeRules (target) {
    return this.exec(target.recipe);
  }

  generate (name = 'default', args = this.argv._) {
    this.debug('Argv', process.argv);
    var template = new Template({
      namespace: 'make:init',
      argv: process.argv.slice(3)
    });

    if (this.argv.help) return template.help();

    if (name.charAt(0) === '@') {
      // Lookup on github instead (todo: npm)

      let repo = name.slice(1).split('/').slice(0, 2).join('/');
      let repository = repo.split('/').slice(-1)[0];
      this.info('From github: %s', repo);
      let request  = require('./fetch');
      return request.github(name.slice(1))
        .then((dir) => {
          this.success('Done fetching %s content from github', name.slice(1));
          this.info('Directory %s', dir);

          let subtree = name.split('/').slice(2).join('/');
          let src = path.join(dir, `${repository}-master`, subtree || '');
          let dest = path.resolve() + '/';

          let cmd = `${src} ${dest}`;
          this.debug('hcp', cmd);
          return cp(cmd, { debug: true })
            .catch(err => this.error(err))
            .then(() => this.success('hcp ok'));
        });
    }

    return template.run(name, args)
      .then(template.end.bind(template));
  }
}

Make.UNKNOWN_TARGET = UNKNOWN_TARGET;
