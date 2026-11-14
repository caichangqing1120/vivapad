const { execFileSync } = require('node:child_process');

function isLocalOnlyPath(path) {
  return /(^|\/)(docs|specs|plans|screenshots?|captures?|\.superpowers)(\/|$)/i.test(path)
    || /(^|\/)(plan|spec)(?:-[^/]*)?\.md$/i.test(path)
    || /(^|\/)[^/]*screenshot[^/]*\.(png|jpe?g|webp|gif|bmp|tiff?)$/i.test(path);
}

function checkPublicFiles() {
  const tracked = execFileSync('git', ['ls-files', '-z'], { encoding: 'utf8' });
  const history = execFileSync('git', ['log', '--full-history', '-m', '--format=', '--name-only', '-z', 'HEAD'], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
  const blocked = [...new Set([...tracked.split('\0'), ...history.split('\0')]
    .filter(isLocalOnlyPath))].sort();
  if (blocked.length) {
    console.error('Local-only files found in tracked files or commit history:\n' + blocked.join('\n'));
    process.exitCode = 1;
    return;
  }
  console.log('Public file policy passed: tracked files and commit history contain no local-only files.');
}

if (require.main === module) {
  checkPublicFiles();
}

module.exports = { isLocalOnlyPath };
