const fs = require('fs');
const lazy = () => import('./lazy.mjs');
class Legacy { start() {} }
module.exports = { lazy, Legacy };
