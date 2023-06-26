

module.exports = parse;


function parse(content) {

  var lines = content.split(/\r?\n/);

  var variables = {};
  var targets = {};

  // reference to last parsed target
  var last = {};

  var tokens = {
    // var declartion
    '\\w+\\s?=\\s*"[^"]*"': function(line) {
      var reg = /(\w+)\s?=\s?"([^"]*)"/;
      var parts = line.match(reg);
      var name = parts[1];
      var value = parts[2];

      variables[name] = value;
    },

    // Target declaration
    '^\\w[^:]+\\s?:': function(line) {
      var reg = /^([^:]+)\s?:\s?(.*)/;
      var parts = line.match(reg);
      var target = parts[1];
      var prerequities = parts[2].split(/\s+/).filter(function(prereq) {
        return prereq;
      });

      last = targets[target] = {
        prerequities: prerequities,
        recipe: ''
      };
    },

    // Rule declaration (anything tabed by one indentation)
    '^(\\s\\s|\\t)': function(line) {
      var reg = /(\s+|\t+)(.*)/;
      var parts = line.match(reg);
      var rule = parts[2];

      // TODO: Var substitution for everything, not just rules
      rule = rule.replace(/\$(\w+)/, function(match, name) {
        if (variables[name]) return variables[name];
        return match;
      });

      if (last.recipe) last.recipe += '\n' + rule;
      else last.recipe += rule;
    }
  };

  lines.forEach(function(line) {
    if (!line) return;

    Object.keys(tokens).forEach(function(token) {
      var reg = new RegExp(token);
      if (reg.test(line)) tokens[token](line);
    });
  });

  return {
    variables: variables,
    targets: targets
  };
}
