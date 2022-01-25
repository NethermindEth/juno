require('dotenv').config()

var fs = require('fs');

fs.writeFile('./static/CNAME', process.env.TARGET_URL, function (err) {
  if (err) throw err;
  console.log('CNAME created ');
}); 
