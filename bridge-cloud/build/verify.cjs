const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');

async function exercise(filename, quiet) {
  let output = 0;
  const sandbox = {
    require, Buffer, URL, setTimeout, clearTimeout,
    process: { env: { AUTH_TOKEN: 'test-secret' } },
    module: { exports: {} },
    console: Object.fromEntries(['log','warn','error','info','debug','trace'].map(k => [k, () => output++])),
  };
  vm.runInNewContext(fs.readFileSync(filename, 'utf8'), sandbox, { timeout: 10000 });
  const handler = sandbox.module.exports.handler;
  assert.equal(typeof handler, 'function');
  const responses = [];
  async function send(route, eventType, payload) {
    const event = { requestContext: { connectionId: route + '-id', eventType, apiGateway: { operationContext: { route } } } };
    if (payload) Object.assign(event, { body: payload.toString('base64'), isBase64Encoded: true });
    const response = await handler(event, { token: { access_token: 'test-iam' } });
    responses.push(JSON.parse(JSON.stringify(response)));
    return response;
  }
  function frame(type, payload = Buffer.alloc(0)) { const b = Buffer.alloc(9); b[0] = type; return Buffer.concat([b, payload]); }
  const bad = await send('adapter', 'MESSAGE', frame(1, Buffer.from('\x01wrong')));
  assert.equal(Buffer.from(bad.body, 'base64')[0], 3);
  await send('adapter', 'CONNECT');
  const hello = await send('adapter', 'MESSAGE', frame(1, Buffer.from('\x01test-secret')));
  assert.equal(Buffer.from(hello.body, 'base64')[0], 2);
  const pong = await send('adapter', 'MESSAGE', frame(0xf0));
  assert.equal(Buffer.from(pong.body, 'base64')[0], 0xf1);
  await send('adapter', 'MESSAGE', frame(6));
  await send('adapter', 'DISCONNECT');
  await send('helper', 'CONNECT');
  const helper = await send('helper', 'MESSAGE', frame(1, Buffer.from('\x01test-secret')));
  assert.equal(Buffer.from(helper.body, 'base64')[0], 2);
  await send('helper', 'MESSAGE', frame(0xf0));
  await send('helper', 'MESSAGE', frame(6));
  await send('helper', 'DISCONNECT');
  await send('unknown', 'CONNECT');
  if (quiet) assert.equal(output, 0, 'production function must emit no console output');
  return responses;
}
(async () => {
  const source = await exercise(process.argv[2], false);
  const built = await exercise(process.argv[3], true);
  assert.deepEqual(built, source);
  console.log('PASS: auth, HELLO, PING, SYNC, disconnect, exported handler, no console output');
})().catch(err => { console.error(err); process.exit(1); });
