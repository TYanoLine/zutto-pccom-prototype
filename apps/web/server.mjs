import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const PORT = parseInt(process.env.PORT || '3001', 10);
const HOST = process.env.HOST || '0.0.0.0';
const DIST_DIR = path.resolve(__dirname, 'dist');
const API_DIR = path.resolve(__dirname, 'api');
const BACKEND_BASE = process.env.BACKEND_BASE || 'https://zutto-pccom-prototype.onrender.com';

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.wav': 'audio/wav',
  '.mp3': 'audio/mpeg',
  '.ogg': 'audio/ogg',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.ttf': 'font/ttf',
  '.txt': 'text/plain; charset=utf-8',
};

// API handler cache
const apiHandlers = new Map();

async function getApiHandler(apiName) {
  if (apiHandlers.has(apiName)) {
    return apiHandlers.get(apiName);
  }
  const filePath = path.join(API_DIR, `${apiName}.js`);
  if (!fs.existsSync(filePath)) {
    return null;
  }
  try {
    const fileUrl = new URL(`file://${filePath}`).href;
    const mod = await import(fileUrl);
    const handler = mod.default;
    if (typeof handler === 'function') {
      apiHandlers.set(apiName, handler);
      return handler;
    }
  } catch (err) {
    console.error(`Failed to load API handler for ${apiName}:`, err);
  }
  return null;
}

function wrapResponse(res) {
  res.status = function (code) {
    res.statusCode = code;
    return res;
  };
  res.json = function (data) {
    if (!res.getHeader('Content-Type')) {
      res.setHeader('Content-Type', 'application/json; charset=utf-8');
    }
    res.end(JSON.stringify(data));
  };
  res.send = function (body) {
    res.end(body);
  };
}

async function proxyToBackend(req, res, targetUrl) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 60000);

  try {
    const headers = { ...req.headers };
    delete headers.host;
    delete headers.connection;

    let body = undefined;
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      body = await new Promise((resolve, reject) => {
        const chunks = [];
        req.on('data', chunk => chunks.push(chunk));
        req.on('end', () => resolve(Buffer.concat(chunks)));
        req.on('error', reject);
      });
    }

    const upstreamRes = await fetch(targetUrl, {
      method: req.method,
      headers,
      body,
      signal: controller.signal,
      cache: 'no-store',
    });

    res.statusCode = upstreamRes.status;
    for (const [key, value] of upstreamRes.headers.entries()) {
      if (key.toLowerCase() !== 'content-encoding') {
        res.setHeader(key, value);
      }
    }

    const data = await upstreamRes.arrayBuffer();
    res.end(Buffer.from(data));
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error(`Upstream proxy error (${targetUrl}):`, msg);
    res.statusCode = 502;
    res.setHeader('Content-Type', 'application/json; charset=utf-8');
    res.end(JSON.stringify({ error: 'Upstream gateway error', detail: msg }));
  } finally {
    clearTimeout(timeout);
  }
}

const server = http.createServer(async (req, res) => {
  wrapResponse(res);

  const reqUrl = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`);
  const pathname = decodeURIComponent(reqUrl.pathname);

  // 1. API routes (/api/...)
  if (pathname.startsWith('/api/')) {
    const parts = pathname.slice('/api/'.length).split('/');
    const apiName = parts[0];

    const handler = await getApiHandler(apiName);
    if (handler) {
      try {
        await handler(req, res);
      } catch (err) {
        console.error(`Error executing API handler ${apiName}:`, err);
        if (!res.writableEnded) {
          res.status(500).json({ error: 'Internal Server Error', detail: String(err) });
        }
      }
      return;
    }

    // Fallback: proxy directly to backend
    const upstreamUrl = new URL(pathname + reqUrl.search, BACKEND_BASE);
    await proxyToBackend(req, res, upstreamUrl);
    return;
  }

  // 2. Static files & SPA routing
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    res.status(405).json({ error: 'Method Not Allowed' });
    return;
  }

  if (!fs.existsSync(DIST_DIR)) {
    res.status(503).setHeader('Content-Type', 'text/html; charset=utf-8');
    res.end('<h1>503 Build Pending</h1><p>apps/web/dist does not exist. Please run "npm run build" first.</p>');
    return;
  }

  let filePath = path.join(DIST_DIR, pathname);
  const safePath = path.resolve(filePath);
  if (!safePath.startsWith(DIST_DIR)) {
    res.status(403).json({ error: 'Forbidden' });
    return;
  }

  // Check if target file exists and is not a directory
  let stat;
  try {
    stat = fs.statSync(safePath);
    if (stat.isDirectory()) {
      const indexFile = path.join(safePath, 'index.html');
      if (fs.existsSync(indexFile)) {
        filePath = indexFile;
        stat = fs.statSync(indexFile);
      } else {
        stat = null;
      }
    }
  } catch {
    stat = null;
  }

  // If file doesn't exist, SPA fallback to /index.html
  if (!stat) {
    const spaIndex = path.join(DIST_DIR, 'index.html');
    if (fs.existsSync(spaIndex)) {
      filePath = spaIndex;
      stat = fs.statSync(spaIndex);
    } else {
      res.status(404).json({ error: 'Not Found' });
      return;
    }
  }

  const ext = path.extname(filePath).toLowerCase();
  const contentType = MIME_TYPES[ext] || 'application/octet-stream';
  res.setHeader('Content-Type', contentType);
  res.setHeader('Content-Length', stat.size);

  // Cache static assets aggressively (Vite adds content hashes), but do not cache HTML
  if (ext === '.html') {
    res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
  } else {
    res.setHeader('Cache-Control', 'public, max-age=31536000, immutable');
  }

  if (req.method === 'HEAD') {
    res.end();
    return;
  }

  const stream = fs.createReadStream(filePath);
  stream.on('error', (streamErr) => {
    console.error('File stream error:', streamErr);
    if (!res.headersSent) res.status(500).end();
  });
  stream.pipe(res);
});

server.listen(PORT, HOST, () => {
  console.log(`[zutto-web] Production server listening on http://${HOST}:${PORT}`);
  console.log(`[zutto-web] Serving static files from: ${DIST_DIR}`);
  console.log(`[zutto-web] Backend upstream: ${BACKEND_BASE}`);
});
