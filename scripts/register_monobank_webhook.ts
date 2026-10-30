/**
 * Helper script to register or unregister Monobank Webhook URL.
 * Usage:
 *   bun scripts/register_monobank_webhook.ts <https://your-public-url/api/v1/webhooks/monobank>
 *   bun scripts/register_monobank_webhook.ts --delete
 */
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

// 1. Load .env
const envPath = resolve(import.meta.dir, '../.env');
if (existsSync(envPath)) {
  const content = readFileSync(envPath, 'utf-8');
  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const [key, ...rest] = trimmed.split('=');
    if (key && rest.length > 0) {
      process.env[key.trim()] = rest.join('=').trim();
    }
  }
}

const token = process.env.MONOBANK_TOKEN;
if (!token) {
  console.error('ERROR: MONOBANK_TOKEN not set in .env');
  process.exit(1);
}

const targetUrl = process.argv[2];

if (!targetUrl) {
  console.log(`
Usage:
  bun scripts/register_monobank_webhook.ts <WEBHOOK_URL>
  bun scripts/register_monobank_webhook.ts --delete

Example:
  bun scripts/register_monobank_webhook.ts https://finance.nneeeooo.pp.ua/api/v1/webhooks/monobank
  `);
  process.exit(0);
}

const webHookUrl = targetUrl === '--delete' ? '' : targetUrl;

console.log(`Registering webhook with Monobank...`);
console.log(`URL: ${webHookUrl || '(empty: deletes webhook)'}`);

const res = await fetch('https://api.monobank.ua/personal/webhook', {
  method: 'POST',
  headers: {
    'X-Token': token,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ webHookUrl })
});

const body = await res.text();
if (res.ok) {
  console.log(`SUCCESS [${res.status}]: Webhook registered successfully!`);
} else {
  console.error(`FAILED [${res.status}]:`, body);
}
