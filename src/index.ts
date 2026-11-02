// Mutfak On printer agent. Listens on 127.0.0.1 only; the kasa page calls it with fetch().
import { createServer, IncomingMessage, ServerResponse } from 'node:http';
import { listPrinters, printRaw } from './printers.js';
import { testReceipt } from './escpos.js';

const VERSION = '0.1.0';
const PORT = Number(process.env.AGENT_PORT ?? 9187);
// Only our own pages may call the agent (the browser sends Origin on cross-origin fetches).
const ALLOWED_ORIGINS = (process.env.AGENT_ORIGINS ?? 'http://localhost:5280,http://mutfakon-portal.trackallops.com,https://mutfakon-portal.trackallops.com').split(',');

function send(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

async function readJson(req: IncomingMessage): Promise<Record<string, unknown>> {
  const chunks: Buffer[] = [];
  for await (const c of req) chunks.push(c as Buffer);
  const text = Buffer.concat(chunks).toString('utf8');
  return text ? JSON.parse(text) : {};
}

const server = createServer(async (req, res) => {
  const origin = req.headers.origin;
  if (origin) {
    if (!ALLOWED_ORIGINS.includes(origin)) return send(res, 403, { error: 'ORIGIN_NOT_ALLOWED' });
    res.setHeader('Access-Control-Allow-Origin', origin);
    res.setHeader('Vary', 'Origin');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
    res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS');
    res.setHeader('Access-Control-Allow-Private-Network', 'true'); // Chrome's preflight for public page -> localhost
  }
  if (req.method === 'OPTIONS') { res.writeHead(204); return res.end(); }

  try {
    const url = new URL(req.url ?? '/', 'http://localhost');
    if (req.method === 'GET' && url.pathname === '/health') return send(res, 200, { ok: true, version: VERSION, platform: process.platform });
    if (req.method === 'GET' && url.pathname === '/printers') return send(res, 200, { printers: await listPrinters() });
    if (req.method === 'POST' && url.pathname === '/test-print') {
      const { printer } = await readJson(req);
      if (typeof printer !== 'string' || !printer) return send(res, 400, { error: 'PRINTER_REQUIRED' });
      await printRaw(printer, testReceipt(printer));
      return send(res, 200, { ok: true });
    }
    send(res, 404, { error: 'NOT_FOUND' });
  } catch (err) {
    console.error(err);
    send(res, 500, { error: 'PRINT_FAILED', message: (err as Error).message });
  }
});

server.listen(PORT, '127.0.0.1', () => console.log(`Mutfak On printer agent ${VERSION} on http://127.0.0.1:${PORT}`));
