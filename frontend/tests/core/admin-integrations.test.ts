import { describe, expect, it } from 'vitest';

import {
  auditFiltered,
  auditParams,
  auditQuery,
  hasProblems,
  newWebhookForm,
  webhookPayload,
  webhookProblems,
  isHookUrl,
  actionFamily,
} from '../../src/core/admin-forms.js';
import {
  deliveryTone,
  detailLines,
  eventGroup,
  groupEvents,
  groupState,
  plainValue,
  retentionShortens,
  toggleEvent,
  toggleGroup,
  tokenState,
  toneClass,
} from '../../src/core/admin-integrations.js';

const EVENTS = [
  'jira:issue_created',
  'jira:issue_updated',
  'jira:issue_deleted',
  'comment_created',
  'comment_updated',
  'comment_deleted',
  'worklog_created',
  'sprint_created',
  'sprint_started',
  'sprint_closed',
  'board_created',
  'board_updated',
  'board_deleted',
];

describe('webhook events', () => {
  it('groups events by what they are about', () => {
    expect(eventGroup('jira:issue_created')).toBe('issues');
    expect(eventGroup('comment_deleted')).toBe('comments');
    expect(eventGroup('worklog_created')).toBe('worklogs');
    expect(eventGroup('sprint_closed')).toBe('sprints');
    expect(eventGroup('board_updated')).toBe('boards');
    expect(eventGroup('version_released')).toBe('other');
    const groups = groupEvents([...EVENTS, 'version_released']);
    expect(groups.map((g) => g.group)).toEqual([
      'issues',
      'comments',
      'worklogs',
      'sprints',
      'boards',
      'other',
    ]);
    expect(groups[0]?.events).toEqual(EVENTS.slice(0, 3));
  });

  it('ticks whole groups in the server order', () => {
    const comments = EVENTS.slice(3, 6);
    expect(groupState([], comments)).toBe('none');
    expect(groupState(['comment_created'], comments)).toBe('some');
    expect(groupState(comments, comments)).toBe('all');
    const chosen = toggleGroup(['board_created', 'jira:issue_created'], comments, true, EVENTS);
    expect(chosen).toEqual(['jira:issue_created', ...comments, 'board_created']);
    expect(toggleGroup(chosen, comments, false, EVENTS)).toEqual([
      'jira:issue_created',
      'board_created',
    ]);
    expect(toggleEvent(['comment_created'], 'jira:issue_updated', true, EVENTS)).toEqual([
      'jira:issue_updated',
      'comment_created',
    ]);
  });
});

describe('webhook form', () => {
  it('needs a name, an http(s) address and an event', () => {
    const f = { ...newWebhookForm(), events: [] };
    expect(webhookProblems(f)).toEqual({
      name: 'nameRequired',
      url: 'urlInvalid',
      events: 'eventsRequired',
    });
    expect(isHookUrl('ftp://example.org')).toBe(false);
    expect(isHookUrl('https://ci.example.org/hook?x=1')).toBe(true);
    const ok = { ...newWebhookForm(), name: 'CI', url: 'http://127.0.0.1:9/' };
    expect(hasProblems(webhookProblems(ok))).toBe(false);
    expect(webhookProblems({ ...ok, name: 'x'.repeat(256) }).name).toBe('nameTooLong');
  });

  it('sends the secret only when it changes', () => {
    const base = { ...newWebhookForm(), name: ' CI ', url: ' https://ci.example.org ' };
    expect(webhookPayload({ ...base, secretAction: 'keep', secret: 'x' })).not.toHaveProperty(
      'secret',
    );
    expect(webhookPayload({ ...base, secretAction: 'replace', secret: '' })).not.toHaveProperty(
      'secret',
    );
    expect(webhookPayload({ ...base, secretAction: 'replace', secret: 's3' }).secret).toBe('s3');
    expect(webhookPayload({ ...base, secretAction: 'remove' }).secret).toBe('');
    expect(webhookPayload(base)).toMatchObject({ name: 'CI', url: 'https://ci.example.org' });
  });
});

describe('deliveries', () => {
  it('tells how a delivery went', () => {
    expect(deliveryTone({ status: 'sent', statusCode: 204 })).toBe('ok');
    expect(deliveryTone({ status: 'sent', statusCode: 500 })).toBe('failed');
    expect(deliveryTone({ status: 'failed', statusCode: null })).toBe('failed');
    expect(deliveryTone({ status: 'pending', statusCode: 503 })).toBe('waiting');
    expect(deliveryTone({ status: 'sending', statusCode: null })).toBe('waiting');
    expect(toneClass('ok')).toBe('done');
    expect(toneClass('failed')).toBe('is-failed');
  });
});

describe('tokens', () => {
  const now = new Date('2026-10-15T12:00:00Z');
  it('tells expired, expiring and working tokens apart', () => {
    expect(tokenState({ expires: null, expired: false }, now)).toBe('active');
    expect(tokenState({ expires: '2026-10-18T00:00:00Z', expired: false }, now)).toBe('expiring');
    expect(tokenState({ expires: '2027-01-01T00:00:00Z', expired: false }, now)).toBe('active');
    expect(tokenState({ expires: '2026-10-01T00:00:00Z', expired: true }, now)).toBe('expired');
    expect(tokenState({ expires: '2026-10-14T00:00:00Z', expired: false }, now)).toBe('expired');
  });
});

describe('audit log', () => {
  it('reads and writes its filters in the address', () => {
    const q = auditQuery('?user=paul.martin&action=user&page=3&q=');
    expect(q).toEqual({ user: 'paul.martin', action: 'user', from: '', to: '', q: '', page: 3 });
    expect(auditParams({ ...q, page: 1 })).toEqual({
      user: 'paul.martin',
      action: 'user',
      from: null,
      to: null,
      q: null,
      page: null,
    });
    expect(auditQuery('?page=-2').page).toBe(1);
    expect(auditFiltered(auditQuery(''))).toBe(false);
    expect(auditFiltered(q)).toBe(true);
    expect(actionFamily('login.success')).toBe('login');
  });

  it('turns details into readable lines', () => {
    expect(detailLines(null)).toEqual([]);
    expect(detailLines({ name: { from: 'Paul', to: 'Paul M.' }, method: 'form', pat: 4 })).toEqual([
      { kind: 'change', key: 'name', from: 'Paul', to: 'Paul M.' },
      { kind: 'value', key: 'method', value: 'form' },
      { kind: 'value', key: 'pat', value: 4 },
    ]);
    // A status renamed: the whole entry is {from, to}; only what moved shows.
    expect(
      detailLines({
        from: { name: 'Open', category: 'todo' },
        to: { name: 'Ouvert', category: 'todo' },
      }),
    ).toEqual([{ kind: 'change', key: 'name', from: 'Open', to: 'Ouvert' }]);
    // Group members: lists from and to.
    expect(detailLines({ from: ['a'], to: ['a', 'b'] })).toEqual([
      { kind: 'change', key: '', from: ['a'], to: ['a', 'b'] },
    ]);
    // Settings: nested source changes flatten one level.
    expect(detailLines({ baseURL: { source: { from: 'env', to: 'database' } } })).toEqual([
      { kind: 'change', key: 'baseURL.source', from: 'env', to: 'database' },
    ]);
  });

  it('writes values plainly', () => {
    expect(plainValue(null)).toBe('—');
    expect(plainValue('')).toBe('—');
    expect(plainValue(true, 'oui', 'non')).toBe('oui');
    expect(plainValue(['a', 'b'])).toBe('a, b');
    expect(plainValue([])).toBe('—');
    expect(plainValue({ a: 1 })).toBe('{"a":1}');
  });

  it('knows when a shorter retention purges at once', () => {
    expect(retentionShortens(0, 90)).toBe(true);
    expect(retentionShortens(365, 90)).toBe(true);
    expect(retentionShortens(90, 365)).toBe(false);
    expect(retentionShortens(90, 0)).toBe(false);
  });
});
