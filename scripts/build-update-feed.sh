#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  printf 'Usage: %s RELEASE_DIR VERSION HTTPS_BASE_URL [OS/ARCH=OLD_VERSION=ARCHIVE ...]\n' "$0" >&2
  exit 2
fi
if [[ -z ${VIVA_UPDATE_KEY:-} || -z ${VIVAPAD_UPDATE_PUBLIC_KEY:-} ]]; then
  printf 'VIVA_UPDATE_KEY and its matching VIVAPAD_UPDATE_PUBLIC_KEY are required; refusing to generate an unsigned feed.\n' >&2
  exit 1
fi

release_dir=$1
version=$2
base_url=$3
shift 3

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ || $base_url != https://* ]]; then
  printf 'Expected a stable version without the v prefix and an HTTPS base URL.\n' >&2
  exit 2
fi
if [[ ! -d $release_dir ]]; then
  printf 'Release directory does not exist: %s\n' "$release_dir" >&2
  exit 2
fi
if [[ -e $release_dir/latest.json || -e $release_dir/latest.json.sig || -e $release_dir/SHA256SUMS ]]; then
  printf 'Use a fresh release directory without an existing feed or checksums.\n' >&2
  exit 2
fi

patch_args=()
for source in "$@"; do
  patch_args+=(--patch-from "$source")
done

export GOWORK=off
stage=$(mktemp -d "$release_dir/.feed.XXXXXXXX")
trap 'rm -rf -- "$stage"' EXIT
go tool viva feed --dir "$release_dir" --out "$stage/latest.json" --app dev.vivapad.app \
  --version "$version" --channel stable --base-url "$base_url" --patch-max-ratio 0.6 ${patch_args[@]+"${patch_args[@]}"}

node - "$release_dir" "$stage" <<'NODE'
const { createHash, createPublicKey, verify } = require('node:crypto');
const { createReadStream, readFileSync, readdirSync, writeFileSync } = require('node:fs');
const { join } = require('node:path');

async function main() {
  const [release, stage] = process.argv.slice(2);
  const encoded = process.env.VIVAPAD_UPDATE_PUBLIC_KEY.trim();
  const raw = Buffer.from(encoded, /^[0-9a-f]{64}$/i.test(encoded) ? 'hex' : 'base64');
  if (raw.length !== 32) throw new Error('Expected a 32-byte Ed25519 public key');
  const der = Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), raw]);
  const key = createPublicKey({ key: der, format: 'der', type: 'spki' });
  const body = readFileSync(join(stage, 'latest.json'));
  const signature = Buffer.from(readFileSync(join(stage, 'latest.json.sig'), 'utf8').trim(), 'base64');
  if (!verify(null, body, key, signature)) throw new Error('Feed signature does not match the release public key');

  const files = readdirSync(release, { withFileTypes: true })
    .filter(entry => !entry.name.startsWith('.'));
  if (files.some(entry => !entry.isFile() || !/^[\w.+-]+$/.test(entry.name))) {
    throw new Error('Expected only regular, safely named release assets');
  }
  const sums = [];
  for (const name of [...files.map(entry => entry.name), 'latest.json', 'latest.json.sig'].sort()) {
    const dir = name.startsWith('latest.json') ? stage : release;
    const hash = createHash('sha256');
    for await (const chunk of createReadStream(join(dir, name))) hash.update(chunk);
    sums.push(`${hash.digest('hex')}  ${name}\n`);
  }
  writeFileSync(join(stage, 'SHA256SUMS'), sums.join(''));
}

main().catch(error => { console.error(error.message); process.exitCode = 1; });
NODE
mv "$stage/latest.json" "$stage/latest.json.sig" "$stage/SHA256SUMS" "$release_dir/"
