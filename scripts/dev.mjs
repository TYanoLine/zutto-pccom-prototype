import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const isWindows = process.platform === 'win32';
const npmCLI = process.env.npm_execpath
  ?? path.join(path.dirname(process.execPath), 'node_modules', 'npm', 'bin', 'npm-cli.js');
const children = [];
let stopping = false;

function start(name, command, args, cwd) {
  const child = spawn(command, args, {
    cwd,
    stdio: 'inherit',
    windowsHide: true,
  });
  children.push(child);
  child.on('error', error => {
    console.error(`[${name}] failed to start:`, error.message);
    stop(1);
  });
  child.on('exit', (code, signal) => {
    if (stopping) return;
    if (signal) console.error(`[${name}] stopped by ${signal}`);
    else console.error(`[${name}] exited with code ${code ?? 1}`);
    stop(code ?? 1);
  });
}

function terminateTree(child) {
  if (!child.pid || child.exitCode !== null) return;
  if (isWindows) {
    spawnSync('taskkill', ['/pid', String(child.pid), '/t', '/f'], {
      stdio: 'ignore',
      windowsHide: true,
    });
  } else {
    child.kill('SIGTERM');
  }
}

function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  for (const child of children) terminateTree(child);
  process.exitCode = code;
}

process.on('SIGINT', () => stop(0));
process.on('SIGTERM', () => stop(0));
process.on('exit', () => {
  for (const child of children) terminateTree(child);
});

start('server', 'go', ['run', './cmd/server'], path.join(root, 'apps', 'server'));
start('web', process.execPath, [npmCLI, 'run', 'dev'], path.join(root, 'apps', 'web'));
