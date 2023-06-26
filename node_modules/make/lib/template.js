import fs from 'fs';
import path from 'path';
import CLI from './cli';
import { Stream } from 'stream';

const { existsSync: exists } = fs;
const { assign } = Object;

export default class Template extends CLI {
  get example () {
    return 'make init <template> [options]';
  }

  get home () {
    return process.platform === 'win32' ? process.env.USERPROFILE : process.env.HOME;
  }

  get more () {
    let lens = this.templates.map(template => template.name.length);
    let max = Math.max.apply(null, lens.concat(CLI.PADDING));

    let templates = this.templates.map((template) => {
      let json = template.json;
      let config = json.make || {};
      let desc = config.description || json.description || `Generate ${template.name} setup`;
      let deps = assign({}, json.dependencies, json.devDependencies);
      deps = Object.keys(deps).slice(0, 4).join(', ');
      deps = deps.length > CLI.PADDING ? deps.slice(0, CLI.PADDING) + '...' : deps;
      if (deps) deps = ` (${deps})`;

      let pad = this.pad(template.name, max);
      let leftpad = this.options.leftpad || '    ';
      return `${leftpad}${template.name}${pad}${desc}${deps}`;
    }).join('\n');

    return `
  Templates:
${templates}
`;
  }

  // Used to parse arguments with minimist
  get alias () {
    return {
      h: 'help',
      v: 'version',
      d: 'debug',
      f: 'force'
    };
  }

  // Used to generate the help output
  get flags () {
    return {
      help: 'Show this help output',
      version: 'Show package version',
      debug: 'Enable extended log output',
      force: 'Force file write even if already existing',
      skip: 'Skip scripts hook'
    };
  }

  get directories () {
    return [
      path.join(this.home, '.config/make/templates'),
      path.join(this.home, '.make/templates'),
      path.join(__dirname, '../templates')
    ];
  }

  constructor (options = {}) {
    super(options);

    this.templates = this.loadTemplates();
    this.names = this.templates.map(dir => dir.name);
  }

  init () {
    if (this.argv.help) return this.help();

    let args = this.parse();
    let name = args._.shift();

    return this.run(name, args._)
      .then(() => {
        this.end();
      });
  }

  expandTemplateDirectory (template) {
    let dir = template.dir;
    if (!exists(dir)) return template;


    let files = fs.readdirSync(dir)
      .map(this.resolve(dir))
      .filter(this.file);

    var lengths = files.map(this.basename).map(file => file.length);
    var max = this.max = Math.max.apply(null, lengths) + 2;
    let cwd = path.resolve();

    var promises = files.map((file) => {
      let name = file.replace(dir + '/', '');
      let dest = path.resolve(path.basename(file));
      let destname = dest.replace(cwd, '.');

      return this.template(file, dest)
        .then(() => {
          this.info('%s%s-> %s', name, this.pad(name, max), destname);
          this.debug('Finished streaming %s content', path.basename(file));
        });
    });

    return assign({}, template, { promises });
  }

  // template(file, dest = path.resolve(file)) {
  template (file, dest = path.resolve(path.basename(file))) {
    if (path.basename(file) === 'package.json') return this.json(file, dest);
    if (path.basename(file) === '.eslintrc') return this.json(file, dest);
    if (path.basename(file) === '.babelrc') return this.json(file, dest);
    return this.stream(file, dest);
  }

  json (file, dest) {
    if (exists(dest)) return this.mergeJSON(file, dest).then(() => {
      this.debug('Finished merging %s file', path.basename(file));
    });

    return this.stream(file, dest);
  }

  mergeJSON (file, dest) {
    let name = path.basename(file);
    this.warning('%s%salready exists, merging', name, this.pad(name, this.max));
    return new Promise((r, errback) => {
      let data = this.readJSON(dest);
      let json = this.readJSON(file);

      let devs = json.devDependencies;
      let deps = json.dependencies;

      // make sure to ignore "make" field in JSON stringify
      let opts = { make: undefined };
      if (devs) opts.devDependencies = assign({}, devs, data.devDependencies);
      if (deps) opts.dependencies = assign({}, deps, data.dependencies);

      let result = assign({}, json, data, opts);
      this.debug('JSON:', result);
      fs.writeFile(dest, JSON.stringify(result, null, 2), (err) => {
        return err ? errback(err) : r();
      });
    });
  }

  stream (file, dest) {
    return new Promise((r, errback) => {
      let existing = exists(dest);
      let filename = path.basename(dest);
      let destname = dest.replace(path.resolve(), '.');
      let output = existing ? this.noopStream() : fs.createWriteStream(dest);
      let input = fs.createReadStream(file);

      if (existing) this.warning('%s%salready exists, skipping', filename, this.pad(filename, this.max));

      let stream = input.pipe(output)
        .on('error', errback)
        .on('close', r);
    });
  }

  run (name = 'default', args) {
    let template = this.templates.find((template) => {
      return template.name === name;
    });

    if (!template) {
      return CLI.fail('No "%s" template', name);
    }

    this.info('Running %s template in %s', name, args.join(' '), process.cwd());
    this.config = template.json ? template.json.make || {} : {};
    this.scripts = this.config.scripts || {};

    return this.invoke('start')
      .then(() => {
        let dir = this.expandTemplateDirectory(template);
        return Promise.all(dir.promises)
          .then(this.invoke.bind(this, 'install'))
          .catch(CLI.fail);
      });
  }

  invoke (name) {
    let args = this.args;
    if (this.argv.skip) return this.noop('Skipping %s script (--skip)', name);

    this.debug('Invoke %s', name, this.argv);
    return this.script('pre' + name)
      .then(this.script.bind(this, name))
      .then(this.script.bind(this, 'post' + name));
  }

  noop (...args) {
    if (args.length) this.info.apply(this, args);
    return new Promise((r, errback) => { r(); });
  }

  script (name) {
    let scripts = this.scripts || {};
    let script = scripts[name] || '';

    if (!script) return this.noop();

    this.info('%s script', name);
    return this.exec(script);
  }

  loadTemplates () {
    let dirs = this.directories;
    this.debug('Load templates from %d directories', dirs.length);

    return dirs
      // Ignore invalid dirs
      .filter(this.exists)
      // Load template from these dirs
      .map(this.loadTemplatesFrom, this)
      // Flatten
      .reduce((a, b) => {
        return a.concat(b);
      }, [])
      // Transfrom into a mapping { name: dir }
      .map((dir) => {
        let json = path.join(dir, 'package.json');

        return {
          dir: dir,
          name: this.basename(dir),
          json: exists(json) ? require(json) : {}
        };
      });
  }

  loadTemplatesFrom (dir) {
    this.debug('Load templates from', dir);
    return fs.readdirSync(dir)
      .map(this.resolve(dir), this)
      .filter(this.directory);
  }

  has (name, names = this.names) {
    return names.indexOf(name) !== -1;
  }

  noopStream () {
    var stream = new Stream();
    stream.write = () => {};
    stream.end = () => {
      stream.emit('finish');
      stream.emit('close');
    };
    return stream;
  }
}
