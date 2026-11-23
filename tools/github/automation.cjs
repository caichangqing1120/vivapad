const { setTimeout: delay } = require('node:timers/promises');

function detectCommand(body) {
  if (typeof body !== 'string') return null;
  if (/^\s*\/unclaim(?:ed)?\s*$/.test(body)) return 'unclaim';
  if (/^\s*\/close\s*$/.test(body)) return 'close';
  if (/(?:^|\s)\/claim(?:\s|$)/.test(body)) return 'claim';
  return null;
}

function sameLogin(left, right) {
  return Boolean(left && right && left.toLowerCase() === right.toLowerCase());
}

const historyQuery = `query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    defaultBranchRef { name }
    issue(number: $number) {
      state
      assignees(first: 100) { nodes { login } }
      timelineItems(first: 100, after: $cursor, itemTypes: [CROSS_REFERENCED_EVENT]) {
        pageInfo { hasNextPage endCursor }
        nodes { ... on CrossReferencedEvent {
          source { __typename ... on PullRequest {
            number state mergedAt baseRefName author { login } repository { nameWithOwner }
          } }
        } }
      }
    }
  }
}`;

async function mergedReference(github, context, number, login) {
  let cursor = null;
  const cursors = new Set();
  const repositoryName = `${context.repo.owner}/${context.repo.repo}`;
  while (true) {
    const { repository } = await github.graphql(historyQuery, { ...context.repo, number, cursor });
    if (!repository?.issue || !repository.defaultBranchRef?.name) {
      throw new Error('Cannot verify issue history or default branch');
    }
    const issue = repository.issue;
    if (issue.state !== 'OPEN' || !issue.assignees.nodes.some(who => sameLogin(who.login, login))) {
      return null;
    }
    for (const { source } of issue.timelineItems.nodes) {
      if (source?.__typename === 'PullRequest' && source.state === 'MERGED' && source.mergedAt &&
          sameLogin(source.author?.login, login) &&
          sameLogin(source.repository?.nameWithOwner, repositoryName) &&
          source.baseRefName === repository.defaultBranchRef.name) {
        return source;
      }
    }
    const next = issue.timelineItems.pageInfo;
    if (!next.hasNextPage) return null;
    if (!next.endCursor || cursors.has(next.endCursor)) throw new Error('Invalid timeline cursor');
    cursors.add(next.endCursor);
    cursor = next.endCursor;
  }
}

async function handleIssueComment({ github, context }) {
  const { issue, comment } = context.payload;
  if (!issue || issue.pull_request || !comment || comment.user.type === 'Bot') return 'ignored';
  const command = detectCommand(comment.body);
  if (!command) return 'ignored';
  const login = comment.user.login;
  const target = { ...context.repo, issue_number: issue.number };
  const respond = body => github.rest.issues.createComment({ ...target, body });
  const refuse = async reason => {
    await respond(`@${login}: ${reason}`);
    return 'refused';
  };
  const { data: current } = await github.rest.issues.get(target);
  if (current.state !== 'open') return refuse('This issue is closed.');
  const owned = current.assignees.some(who => sameLogin(who.login, login));

  if (command === 'claim') {
    if (current.assignees.length) {
      return refuse(owned ? 'You already claimed this issue.' :
        `This issue is assigned to ${current.assignees.map(who => `@${who.login}`).join(', ')}.`);
    }
    try {
      const { data: assigned } = await github.rest.issues.addAssignees({ ...target, assignees: [login] });
      if (!assigned.assignees.some(who => sameLogin(who.login, login))) {
        throw new Error('GitHub did not apply the assignment');
      }
    } catch (error) {
      await respond(`@${login}: Could not assign this issue. A maintainer must check assignment permissions.`);
      throw new Error('Issue assignment failed', { cause: error });
    }
    await respond(`@${login} claimed #${issue.number}. Confirm the scope and acceptance criteria before implementation.`);
    return 'claimed';
  }

  if (!owned) return refuse('Only a current assignee may use this command.');
  if (command === 'unclaim') {
    await github.rest.issues.removeAssignees({ ...target, assignees: [login] });
    await respond(`@${login} released #${issue.number}; it is available to claim.`);
    return 'unclaimed';
  }

  const merged = await mergedReference(github, context, issue.number, login);
  if (!merged) return refuse('No linked PR by you has merged into this repository\'s default branch; the issue stays open.');
  await github.rest.issues.update({ ...target, state: 'closed', state_reason: 'completed' });
  await respond(`@${login}: linked PR #${merged.number} has merged. Closed #${issue.number}.`);
  return 'closed';
}

async function validatePullRequest({ github, context, pullNumber, wait = delay }) {
  const number = pullNumber || context.payload.pull_request?.number;
  if (!Number.isSafeInteger(number) || number <= 0) throw new Error('Invalid pull request number');
  const query = `query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
    repository(owner: $owner, name: $repo) {
      pullRequest(number: $number) {
        author { __typename login }
        closingIssuesReferences(first: 100, after: $cursor) {
          pageInfo { hasNextPage endCursor }
          nodes { number state repository { nameWithOwner } assignees(first: 100) { nodes { login } } }
        }
      }
    }
  }`;
  const repositoryName = `${context.repo.owner}/${context.repo.repo}`;
  const cursors = new Set();
  const retryDelays = [2000, 4000, 8000, 16000];
  let retry = 0;
  let cursor = null;
  let count = 0;
  while (true) {
    const { repository } = await github.graphql(query, { ...context.repo, number, cursor });
    const pull = repository?.pullRequest;
    if (!pull?.author?.login || !pull.closingIssuesReferences) throw new Error('Cannot verify pull request references');
    if (pull.author.__typename === 'Bot' && pull.author.login === 'dependabot') return 'dependency-update';
    const references = pull.closingIssuesReferences;
    // GitHub may index closing references after delivering the PR event.
    if (cursor === null && references.nodes.length === 0 && references.pageInfo.hasNextPage === false &&
        retry < retryDelays.length) {
      await wait(retryDelays[retry++]);
      continue;
    }
    for (const issue of references.nodes) {
      if (!sameLogin(issue.repository?.nameWithOwner, repositoryName) || issue.state !== 'OPEN') {
        throw new Error(`#${issue.number} must be an open issue in this repository`);
      }
      if (!issue.assignees.nodes.some(who => sameLogin(who.login, pull.author.login))) {
        throw new Error(`Claim issue #${issue.number} and wait for bot confirmation before opening the PR`);
      }
      count++;
    }
    if (!references.pageInfo.hasNextPage) break;
    const next = references.pageInfo.endCursor;
    if (!next || cursors.has(next)) throw new Error('Invalid issue-reference cursor');
    cursors.add(next);
    cursor = next;
  }
  if (!count) throw new Error('Link a claimed issue with Closes #N, Fixes #N or Resolves #N');
  return 'linked-and-claimed';
}

module.exports = { detectCommand, handleIssueComment, validatePullRequest };
