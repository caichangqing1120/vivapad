const assert = require('node:assert/strict');
const test = require('node:test');
const { execFileSync, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { isLocalOnlyPath } = require('./check-public-files.cjs');

test('local documents, plans and screenshots cannot enter public history', () => {
  for (const path of [
    'docs/specs/001-vivapad/plan.md',
    'internal/docs/design.md',
    'plan.md',
    'PLAN.md',
    'feature/plan-next.md',
    'plans/next.md',
    'spec.md',
    'feature/spec-next.md',
    'specs/editor.md',
    'artifacts/screenshots/desktop.png',
    'screenshots/compact.png',
    'Screenshots/compact.png',
    'captures/screenshot-desktop.png',
    'captures/desktop.png',
    'desktop-screenshot.png',
    '.superpowers/sdd/plan/progress.md',
  ]) {
    assert.equal(isLocalOnlyPath(path), true, path);
  }
});

test('source, contribution rules and application assets stay publishable', () => {
  for (const path of [
    'README.md',
    'CONTRIBUTING.md',
    'AGENTS.md',
    '.github/workflows/ci.yml',
    'cmd/vivapad/main.go',
    'cmd/vivapad/screenshot_test.go',
    'assets/vivapad-icon.png',
    'assets/icons/save.svg',
    'internal/printing/fonts/NotoSansSC.ttf',
    'tools/github/check-public-files.test.cjs',
  ]) {
    assert.equal(isLocalOnlyPath(path), false, path);
  }
});

function repository(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'vivapad-public-files-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const git = (...args) => execFileSync('git', args, { cwd: directory, stdio: 'pipe' });
  git('init', '--quiet');
  git('config', 'user.name', 'Public File Test');
  git('config', 'user.email', 'public-file-test@example.invalid');
  return { directory, git };
}

function check(directory) {
  return spawnSync(process.execPath, [path.join(__dirname, 'check-public-files.cjs')], {
    cwd: directory,
    encoding: 'utf8',
  });
}

test('CLI rejects a deleted local document that is still in history', (t) => {
  const { directory, git } = repository(t);
  fs.mkdirSync(path.join(directory, 'docs'));
  fs.writeFileSync(path.join(directory, 'docs', 'plan.md'), 'local plan');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Add local plan');
  git('rm', 'docs/plan.md');
  git('commit', '--quiet', '-m', 'Remove local plan');
  const result = check(directory);
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /docs\/plan\.md/);
});

test('CLI accepts clean history and rejects a force-staged screenshot', (t) => {
  const { directory, git } = repository(t);
  fs.writeFileSync(path.join(directory, 'README.md'), 'VivaPad');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Add public source');
  const clean = check(directory);
  assert.equal(clean.status, 0, clean.stderr);
  fs.mkdirSync(path.join(directory, 'screenshots'));
  fs.writeFileSync(path.join(directory, 'screenshots', 'desktop.png'), 'capture');
  git('add', '-f', 'screenshots/desktop.png');
  const blocked = check(directory);
  assert.equal(blocked.status, 1, blocked.stderr);
  assert.match(blocked.stderr, /screenshots\/desktop\.png/);
});

test('CLI rejects files present only in merge commits', (t) => {
  const { directory, git } = repository(t);
  fs.writeFileSync(path.join(directory, 'README.md'), 'VivaPad');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Add public source');
  git('branch', 'clean');
  git('switch', '--quiet', '-c', 'side');
  fs.writeFileSync(path.join(directory, 'side.txt'), 'side');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Side change');
  git('switch', '--quiet', '-c', 'merged', 'HEAD~1');
  git('merge', '--quiet', '--no-ff', '--no-commit', 'side');
  fs.mkdirSync(path.join(directory, 'docs'));
  fs.writeFileSync(path.join(directory, 'docs', 'plan.md'), 'merge-only plan');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Merge with local plan');
  git('switch', '--quiet', 'clean');
  fs.writeFileSync(path.join(directory, 'clean.txt'), 'clean');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Clean side change');
  git('switch', '--quiet', 'merged');
  git('merge', '--quiet', '--no-ff', '--no-commit', 'clean');
  git('rm', 'docs/plan.md');
  git('commit', '--quiet', '-m', 'Merge without local plan');
  const result = check(directory);
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /docs\/plan\.md/);
});
