// The panel's event stream against a server that keeps dropping it. No depphunter is
// needed: what is under test is how Api.watch reconnects, not what the server says.

const assert = require('node:assert');
const http = require('node:http');
const { it } = require('node:test');

require('./stub');
const { Api } = require('../out/api.js');

it('reconnects promptly after every drop, however many there were', async () => {
  // Each connection gets one event and is then reset mid-response: the socket goes
  // and the response never ends. A client that only retried on 'end' never comes
  // back; one whose delay doubled for good takes 1s, 2s, 4s... between them.
  let connections = 0;
  const server = http.createServer((req, res) => {
    connections++;
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    res.write(`id: ${connections}\nevent: selection\ndata: {"id":"n${connections}","origin":"x"}\n\n`);
    setTimeout(() => req.socket.destroy(), 20);
  });
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const api = new Api(`http://127.0.0.1:${server.address().port}/?token=t`);

  const seen = [];
  const stream = api.watch(e => seen.push(e.data.id));
  try {
    // Four connections are three reconnections: about 3s at 1s each, 7s doubling.
    const deadline = Date.now() + 5000;
    while (seen.length < 4 && Date.now() < deadline) await new Promise(r => setTimeout(r, 50));
    assert.deepStrictEqual(seen.slice(0, 4), ['n1', 'n2', 'n3', 'n4']);
  } finally {
    stream.dispose();
    server.close();
  }
});
