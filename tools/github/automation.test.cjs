const { test } = require('node:test');
const assert = require('node:assert/strict');
const { detectCommand, handleIssueComment, validatePullRequest } = require('./automation.cjs');

function fixture({ body = '/claim', assignees = [], state = 'open', pages = [] } = {}) {
  const calls = [];
  const waits = [];
  const issue = { state, assignees: assignees.map(login => ({ login })) };
  const policy = { author: 'alice', authorType: 'User', references: null, pages: [] };
  const context = {
    repo: { owner: 'owner', repo: 'vivapad' },
    payload: {
      issue: { number: 7 },
      comment: { body, user: { login: 'alice', type: 'User' } },
      pull_request: { number: 8, body: 'Closes #7', user: { login: 'alice' } },
    },
  };
  const github = {
    rest: {
      issues: {
        get: async () => ({ data: issue }),
        createComment: async data => { calls.push(['comment', data.body]); },
        addAssignees: async data => {
          calls.push(['assign', data.assignees]);
          return { data: { assignees: data.assignees.map(login => ({ login })) } };
        },
        removeAssignees: async data => { calls.push(['unassign', data.assignees]); },
        update: async data => { calls.push(['update', data]); },
      },
      pulls: { get: async () => ({ data: context.payload.pull_request }) },
    },
    graphql: async (query, variables) => {
      if (query.includes('closingIssuesReferences')) {
        calls.push(['policy', variables.number, variables.cursor]);
        const references = policy.references || [{
          ...issue, number: 7, state: issue.state.toUpperCase(), repository: { nameWithOwner: 'owner/vivapad' },
          assignees: { nodes: issue.assignees },
        }];
        return { repository: { pullRequest: {
          author: { __typename: policy.authorType, login: policy.author },
          closingIssuesReferences: policy.pages.shift() || {
            nodes: references, pageInfo: { hasNextPage: false, endCursor: null },
          },
        } } };
      }
      calls.push(['graphql', variables.cursor]);
      assert.ok(query.includes('CROSS_REFERENCED_EVENT'));
      const page = pages.shift();
      assert.ok(page, 'unexpected GraphQL call');
      return { repository: { defaultBranchRef: { name: 'main' }, issue: {
        state: state.toUpperCase(), assignees: { nodes: issue.assignees },
        timelineItems: page,
      } } };
    },
  };
  return { github, context, calls, issue, policy, waits, wait: async ms => { waits.push(ms); } };
}

function page(prs = [], next = null) {
  return {
    nodes: prs.map(pr => ({ source: {
      __typename: 'PullRequest', number: 8, state: 'MERGED', mergedAt: '2026-11-10T00:00:00Z',
      baseRefName: 'main', author: { login: 'alice' }, repository: { nameWithOwner: 'owner/vivapad' }, ...pr,
    } })),
    pageInfo: { hasNextPage: next !== null, endCursor: next },
  };
}

test('command syntax follows Motty and does not interpret arbitrary text', () => {
  for (const body of ['/claim', 'I would like to /claim this', ' /claim\n']) {
    assert.equal(detectCommand(body), 'claim');
  }
  for (const body of ['/unclaim', ' /unclaimed\n']) assert.equal(detectCommand(body), 'unclaim');
  assert.equal(detectCommand('/close'), 'close');
  for (const body of ['/claimer', '/close later', '/unclaim please', '`/claim`', '', null]) {
    assert.equal(detectCommand(body), null);
  }
});

test('claim assigns the commenter and confirms only the actual assignment', async () => {
  const fixtureData = fixture();
  assert.equal(await handleIssueComment(fixtureData), 'claimed');
  assert.deepEqual(fixtureData.calls[0], ['assign', ['alice']]);
  assert.match(fixtureData.calls[1][1], /@alice.*claimed/i);
});

test('an existing assignment or a closed issue is never overwritten', async () => {
  for (const options of [{ assignees: ['bob'] }, { assignees: ['ALICE'] }, { state: 'closed' }]) {
    const fixtureData = fixture(options);
    await handleIssueComment(fixtureData);
    assert.deepEqual(fixtureData.calls.map(call => call[0]), ['comment']);
  }
});

test('silent assignment refusal and API failures do not claim success', async () => {
  for (const failure of ['empty', 'error']) {
    const fixtureData = fixture();
    fixtureData.github.rest.issues.addAssignees = async () => {
      if (failure === 'error') throw new Error('forbidden');
      return { data: { assignees: [] } };
    };
    await assert.rejects(handleIssueComment(fixtureData), /assign/i);
    assert.match(fixtureData.calls.at(-1)[1], /could not assign/i);
  }
});

test('bot comments, PR comments and unrelated text perform no API calls', async () => {
  for (const kind of ['bot', 'pr', 'text']) {
    const fixtureData = fixture();
    if (kind === 'bot') fixtureData.context.payload.comment.user.type = 'Bot';
    if (kind === 'pr') fixtureData.context.payload.issue.pull_request = {};
    if (kind === 'text') fixtureData.context.payload.comment.body = 'hello';
    assert.equal(await handleIssueComment(fixtureData), 'ignored');
    assert.deepEqual(fixtureData.calls, []);
  }
});

test('only the current assignee can release a claim', async () => {
  const owned = fixture({ body: '/unclaimed', assignees: ['ALICE'] });
  assert.equal(await handleIssueComment(owned), 'unclaimed');
  assert.deepEqual(owned.calls[0], ['unassign', ['alice']]);
  const other = fixture({ body: '/unclaim', assignees: ['bob'] });
  assert.equal(await handleIssueComment(other), 'refused');
  assert.deepEqual(other.calls.map(call => call[0]), ['comment']);
});

test('close follows timeline pagination and closes only after a matching merged PR', async () => {
  const fixtureData = fixture({ body: '/close', assignees: ['alice'], pages: [page([], 'next'), page([{}])] });
  assert.equal(await handleIssueComment(fixtureData), 'closed');
  assert.deepEqual(fixtureData.calls.filter(call => call[0] === 'graphql'), [['graphql', null], ['graphql', 'next']]);
  const update = fixtureData.calls.find(call => call[0] === 'update')[1];
  assert.equal(update.state, 'closed');
  assert.equal(update.state_reason, 'completed');
});

test('close refuses missing, foreign, unmerged, wrong-author and wrong-base PRs', async () => {
  const invalid = [
    null, { repository: { nameWithOwner: 'foreign/vivapad' } }, { state: 'OPEN' },
    { mergedAt: null }, { author: { login: 'bob' } }, { baseRefName: 'other' },
  ];
  for (const pullRequest of invalid) {
    const fixtureData = fixture({ body: '/close', assignees: ['alice'], pages: [page(pullRequest ? [pullRequest] : [])] });
    assert.equal(await handleIssueComment(fixtureData), 'refused');
    assert.ok(!fixtureData.calls.some(call => call[0] === 'update'));
  }
});

test('close refuses non-assignees without reading history', async () => {
  const fixtureData = fixture({ body: '/close', assignees: ['bob'] });
  assert.equal(await handleIssueComment(fixtureData), 'refused');
  assert.deepEqual(fixtureData.calls.map(call => call[0]), ['comment']);
});

test('a missing or repeated timeline cursor fails closed', async () => {
  for (const pages of [[page([], '')], [page([], 'same'), page([], 'same')]]) {
    const fixtureData = fixture({ body: '/close', assignees: ['alice'], pages });
    await assert.rejects(handleIssueComment(fixtureData), /cursor/i);
    assert.ok(!fixtureData.calls.some(call => call[0] === 'update'));
  }
});

test('PR policy consumes GitHub-resolved references and current PR author, not raw event text', async () => {
  const fixtureData = fixture({ assignees: ['ALICE'] });
  fixtureData.context.payload.pull_request.body = 'The API is authoritative, not this stale body';
  fixtureData.context.payload.pull_request.user.login = 'stale-author';
  assert.equal(await validatePullRequest(fixtureData), 'linked-and-claimed');
  assert.deepEqual(fixtureData.calls, [['policy', 8, null]]);
});

test('PR policy rereads the first page when GitHub has not indexed the closing reference yet', async () => {
  const fixtureData = fixture({ assignees: ['alice'] });
  fixtureData.policy.pages = [{ nodes: [], pageInfo: { hasNextPage: false } }];
  assert.equal(await validatePullRequest(fixtureData), 'linked-and-claimed');
  assert.deepEqual(fixtureData.calls, [['policy', 8, null], ['policy', 8, null]]);
  assert.equal(fixtureData.waits.length, 1);
});

test('PR policy rejects a resolved but unclaimed reference after retrying an empty result', async () => {
  const fixtureData = fixture();
  fixtureData.policy.pages = [{ nodes: [], pageInfo: { hasNextPage: false } }];
  await assert.rejects(validatePullRequest(fixtureData), /claim issue #7/i);
  assert.equal(fixtureData.waits.length, 1);
});

test('PR policy stops retrying when GitHub keeps returning no closing references', async () => {
  const fixtureData = fixture({ assignees: ['alice'] });
  fixtureData.policy.references = [];
  await assert.rejects(validatePullRequest(fixtureData), /link a claimed issue/i);
  assert.equal(fixtureData.calls.length, 5);
  assert.equal(fixtureData.waits.reduce((total, ms) => total + ms, 0), 30_000);
});

test('PR policy rejects absent, foreign, unassigned or closed resolved Issue references', async () => {
  for (const kind of ['none', 'foreign', 'unassigned', 'closed']) {
    const fixtureData = fixture({ assignees: ['alice'] });
    if (kind === 'none') fixtureData.policy.references = [];
    if (kind === 'foreign') fixtureData.policy.references = [{
      number: 7, state: 'OPEN', repository: { nameWithOwner: 'foreign/vivapad' },
      assignees: { nodes: [{ login: 'alice' }] },
    }];
    if (kind === 'unassigned') fixtureData.issue.assignees = [];
    if (kind === 'closed') fixtureData.issue.state = 'closed';
    await assert.rejects(validatePullRequest(fixtureData), /issue|claim/i);
  }
});

test('only the actual Dependabot account is exempt from Issue policy', async () => {
  const fixtureData = fixture();
  fixtureData.policy.author = 'dependabot';
  fixtureData.policy.authorType = 'Bot';
  fixtureData.policy.references = [];
  assert.equal(await validatePullRequest(fixtureData), 'dependency-update');
  fixtureData.policy.authorType = 'User';
  await assert.rejects(validatePullRequest(fixtureData), /issue/i);
  fixtureData.policy.authorType = 'Bot';
  fixtureData.policy.author = 'other';
  await assert.rejects(validatePullRequest(fixtureData), /issue/i);
});

test('raw closing-keyword text cannot replace a GitHub-resolved reference', async () => {
  for (const body of ['```text\nCloses #7\n```', '~~~text\nCloses #7\n~~~', '```\nCloses #7', 'Closes #7']) {
    const fixtureData = fixture({ assignees: ['alice'] });
    fixtureData.policy.references = [];
    fixtureData.context.payload.pull_request.body = body;
    await assert.rejects(validatePullRequest(fixtureData), /issue/i);
  }
});

test('PR policy follows reference pagination and validates every linked Issue', async () => {
  const fixtureData = fixture({ assignees: ['alice'] });
  fixtureData.policy.pages = [
    { nodes: [], pageInfo: { hasNextPage: true, endCursor: 'next' } },
    { nodes: [{ number: 99, state: 'OPEN', repository: { nameWithOwner: 'owner/vivapad' },
      assignees: { nodes: [{ login: 'bob' }] } }], pageInfo: { hasNextPage: false } },
  ];
  await assert.rejects(validatePullRequest(fixtureData), /claim/i);
  assert.deepEqual(fixtureData.calls, [['policy', 8, null], ['policy', 8, 'next']]);
});

test('PR policy refuses unavailable API data and repeated cursors', async () => {
  const missing = fixture();
  missing.github.graphql = async () => ({ repository: null });
  await assert.rejects(validatePullRequest(missing), /pull request/i);
  const repeating = fixture();
  repeating.policy.pages = [
    { nodes: [], pageInfo: { hasNextPage: true, endCursor: 'same' } },
    { nodes: [], pageInfo: { hasNextPage: true, endCursor: 'same' } },
  ];
  await assert.rejects(validatePullRequest(repeating), /cursor/i);
});

test('manual adapter rechecks query the requested PR number', async () => {
  const fixtureData = fixture({ assignees: ['alice'] });
  delete fixtureData.context.payload.pull_request;
  await validatePullRequest({ ...fixtureData, pullNumber: 19 });
  assert.deepEqual(fixtureData.calls, [['policy', 19, null]]);
});
