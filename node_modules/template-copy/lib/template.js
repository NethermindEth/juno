const fs       = require('fs');
const path     = require('path');
const glob     = require('glob');
const mkdirp   = require('mkdirp');
const assign   = require('deep-assign');
const through2 = require('through2');
const template = require('lodash.template');
const minimist = require('minimist');
const exists   = fs.existsSync;

const debug = require('debug')('template-copy');
const log = require('./log');

const node = (resolve, reject) => (err) => { err ? reject(err) : resolve(); };

export default class Template {
  get messages () {
    return {
      NOT_A_DIRECTORY: 'target \'%s\' is not a directory',
      MISSING_FILE_OPERAND: 'missing file operand',
      MISSING_DESTINATION_FILE_OPERAND: 'missing destination file operand after \'%s\''
    };
  }

  get templateSettings () {
    return {
      interpolate: /{{(.+?)}}/g
    };
  }

  get inquirer () {
    try {
      return require('inquirer').createPromptModule();
    } catch (e) {
      this.error('Inquirer is not available, skipping prompt');
    }
  }

  get globSettings () {
    return {
      dot: true,
      nodir: true,
      matchBase: true
    };
  }

  get arrow () {
    return '=>';
  }

  constructor (argv = [], options = {}) {
    this.options = options;
    this.options.argv = this.options.argv || {};

    this.argv = this.parse(argv);
    this.env = options.env || Object.assign({}, process.env);

    this.env = Object.keys(this.env)
      .filter(name => /^hcp_/i.test(name))
      .map(name => { return { name, value: this.env[name] }; })
      .reduce((a, b) => {
        a[b.name.replace(/^hcp_/i, '')] = b.value;
        return a;
      }, {});
  }

  init (files = []) {
    this.files = files;
    this.dest = files.pop();
    this.dir = this.dest && this.dest.slice(-1)[0] === '/';

    this.check();

    this.files = files.map(this.globSync).reduce(this.flatten, []);
    if (!this.files.length) this.files = files;

    let promises = this.files.map(file => this.write(file, this.dest));
    return Promise.all(promises)
      .catch((err) => this.error(err));
  }

  parse (args, alias = {}) {
    return minimist(args, alias);
  }

  write (file, dest) {
    var promise = this.dir ? this.mkdirp(this.dest) : this.noop();
    return promise.then(this.copy.bind(this, file));
  }

  copy (file) {
    var dest = this.dir ? path.join(this.dest, path.basename(file)) : this.dest;
    dest = path.resolve(dest);
    return this.exists(dest)
      .then((filepath) => {
        let ext = path.extname(file);
        if (filepath && ext === '.json') return this.json(file, dest);
        return this.file(file, dest);
      });
  }

  file (file, dest) {
    return this.isDirectory(file)
      .then((dir) => {
        if (dir) return this.dirCopy(file, dest);

        return this.exists(dest)
          .then((filepath) => {
            if (filepath) {
              this.warn('%s already exists, skipping ...', filepath);
              return this.noop();
            }

            return this.mkdirp(path.dirname(dest))
              .then(() => this.fileCopy(file, dest));
          });
      });
  }

  dirCopy (from, to) {
    debug('dir copy', from, to);
    return this.glob(path.join(from, '**'))
      .then((files) => {
        this.files = this.files.concat(files);

        files = files.concat().map((file) => {
          let rel = file.replace(from, '');
          return { rel, file };
        });

        this.info('Directory copy %s to %s', from, this.dest);
        return this.recurse(files);
      });
  }

  recurse (files = []) {
    let file = files.shift();
    if (!file) return true;

    return this.file(file.file, path.join(this.dest, file.rel))
      .then(this.recurse.bind(this, files));
  }

  fileCopy (file, dest) {
    this.success('%s %s %s', file, this.arrow, dest);

    file = path.resolve(file);
    return new Promise((r, errback) => {
      debug('cp', file, dest);
      let input = fs.createReadStream(file);
      let out = fs.createWriteStream(dest);
      let chunks = '';

      input.setEncoding('utf8');
      input.on('data', chunk => { chunks += chunk; });

      input
        .pipe(this.transform(file, dest))
        .pipe(out)
        .on('error', errback)
        .on('close', r);
    });
  }

  transform () {
    let render = this.render.bind(this);
    return through2(function (chunk, enc, done) {
      render(chunk + '', (err, res) => {
        if (err) return done(err);
        this.push(res);
        return done();
      });
    });
  }

  placeholders (content = '') {
    let placeholders = content.match(/{{([^}]+}})/g);
    if (!placeholders) return;

    placeholders = placeholders.map((p) => {
      return p.replace(/{{\s*/, '').replace(/\s*}}/, '');
    });

    placeholders = placeholders.reduce(this.uniq.bind(this), []);

    // skip values already available in env or options
    placeholders = placeholders.filter((name) => {
      return !(name in this.options || name in this.env);
    });

    return placeholders;
  }

  question (name, message) {
    return {
      type: 'input',
      message: name || message,
      default: this.argv[name] || this.env[name] || name,
      name
    };
  }

  render (content, done) {
    let placeholders = this.placeholders(content);
    if (!placeholders) return this.template(content, {}, done);

    let questions = placeholders.map(this.question, this);

    let defaults = placeholders.map((name) => {
      return { name };
    });

    let argv = Object.assign({}, this.options);
    let data = Object.assign({}, this.env, argv, defaults);

    if (!questions.length) return this.template(content, data, done);

    this.prompt(questions)
      .catch(done)
      .then((answers) => {
        placeholders.forEach((p) => {
          let answer = answers[p] === 'true' ? true
            : answers[p] === 'false' ? false
            : answers[p];

          data[p] = typeof answer !== 'undefined' ? answer : (this.argv[p] || p);
        });

        this.template(content, data, done);
      });
  }

  prompt (questions, done) {
    let inquirer = this.inquirer;
    return inquirer ? inquirer(questions, done) : done();
  }

  template (content, data, done) {
    var tpl = template(content, this.templateSettings);
    let output = tpl(data);
    done(null, output);
  }

  json (file, destination) {
    let input = this.readJSON(path.resolve(file));
    let dest = path.resolve(destination);
    dest = exists(dest) ? this.readJSON(dest) : {};

    let output = assign({}, input, dest);

    return new Promise((r, errback) => {
      fs.writeFile(destination, JSON.stringify(output, null, 2), node(r, errback));
    });
  }

  readJSON (file) {
    try {
      return require(file);
    } catch (e) {
      this.warn(e.message);
      return {};
    }
  }

  noop (...args) {
    return new Promise((r, errback) => r(...args));
  }

  mkdirp (filepath) {
    return this.isDirectory(filepath)
      .then((isDir) => {
        if (isDir) return filepath;
        return new Promise((r, errback) => {
          mkdirp(path.resolve(filepath), node(r, errback));
        });
      });
  }

  exists (filepath) {
    return new Promise((r, errback) => {
      fs.exists(filepath, (exists) => {
        r(exists ? filepath : false);
      });
    });
  }

  isDirectory (filepath) {
    return new Promise((r, errback) => {
      fs.stat(filepath, (err, stat) => {
        if (err && err.code === 'ENOENT') return r(false);
        err ? errback(err) : r(stat.isDirectory());
      });
    });
  }

  globSync (pattern, opts = this.globSettings) {
    return glob.sync(pattern, opts);
  }

  glob (pattern, opts = this.globSettings) {
    return new Promise((r, errback) => {
      glob(pattern, opts, (err, files) => {
        err ? errback(err) : r(files);
      });
    });
  }

  flatten (a, b) {
    return a.concat(b);
  }

  uniq (a, b) {
    if (a.indexOf(b) !== -1) return a;
    return a.concat(b);
  }

  check () {
    if (!this.files.length) {
      if (this.dest) this.error(this.messages.MISSING_DESTINATION_FILE_OPERAND, this.dest);
      else this.error(this.messages.MISSING_FILE_OPERAND);
      process.exit(1);
    }

    if (this.files.length > 1 && this.dest) {
      if (!this.dir) this.error(this.messages.NOT_A_DIRECTORY, this.dest).exit(1);
    }
  }
}

Object.assign(Template.prototype, log);
