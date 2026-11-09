// Local SSO: runs the sibling nneeeooo-auth repo against the auth_dev database on the same Postgres as .env.
// migrate + dev-seed are idempotent; then serve with the password-less dev login on :8090.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const env = {};
for (const l of readFileSync(new URL("../.env", import.meta.url), "utf8").split(/\r?\n/)) {
  const m = l.match(/^(POSTGRES_\w+)=(.*)$/);
  if (m) env[m[1]] = m[2];
}
const e = encodeURIComponent;
const DATABASE_URL = `postgres://${e(env.POSTGRES_USER)}:${e(env.POSTGRES_PASSWORD)}@${env.POSTGRES_HOST}:${env.POSTGRES_PORT || 5432}/auth_dev`;
const opts = {
  cwd: process.env.AUTH_REPO ?? "../nneeeooo-auth",
  stdio: "inherit",
  env: { ...process.env, DATABASE_URL, AUTH_ISSUER: "http://localhost:8090", AUTH_DEV_LOGIN: "1" },
};
for (const args of [["migrate"], ["admin", "dev-seed"], ["serve"]]) {
  const r = spawnSync("cargo", ["run", "-j", "2", "--", ...args], opts);
  if (r.status !== 0) process.exit(r.status ?? 1);
}
