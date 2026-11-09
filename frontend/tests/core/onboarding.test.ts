import { describe, expect, it } from 'vitest';

import type { Onboarding } from '../../src/core/admin-types.js';
import { onboardingSteps, showOnboarding } from '../../src/core/onboarding.js';

const none: Onboarding = {
  projects: false,
  otherUsers: false,
  mail: false,
  boards: false,
  issues: false,
  dismissed: false,
};

describe('onboardingSteps', () => {
  it('lists the five steps in order, none done on a new instance', () => {
    const { steps, done } = onboardingSteps(none);
    expect(steps.map((s) => s.step)).toEqual(['projects', 'users', 'mail', 'boards', 'issues']);
    expect(steps.every((s) => !s.done)).toBe(true);
    expect(done).toBe(0);
  });

  it('counts the steps done', () => {
    const { steps, done } = onboardingSteps({ ...none, projects: true, mail: true });
    expect(done).toBe(2);
    expect(steps.filter((s) => s.done).map((s) => s.step)).toEqual(['projects', 'mail']);
  });

  it('reads another user as the users step', () => {
    expect(onboardingSteps({ ...none, otherUsers: true }).steps[1]).toEqual({
      step: 'users',
      done: true,
    });
  });
});

describe('showOnboarding', () => {
  const all = { ...none, projects: true, otherUsers: true, mail: true, boards: true, issues: true };

  it('shows while a step is left', () => {
    expect(showOnboarding(none)).toBe(true);
    expect(showOnboarding({ ...all, issues: false })).toBe(true);
  });

  it('hides once every step is done', () => {
    expect(showOnboarding(all)).toBe(false);
  });

  it('hides once dismissed, or before it loads', () => {
    expect(showOnboarding({ ...none, dismissed: true })).toBe(false);
    expect(showOnboarding(null)).toBe(false);
  });
});
