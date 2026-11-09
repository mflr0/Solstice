/** The Getting started checklist My space shows administrators. */

import type { Onboarding } from './admin-types.js';

/** A setup step, in the order the checklist lists them. */
export type OnboardingStep = 'projects' | 'users' | 'mail' | 'boards' | 'issues';

export interface StepState {
  step: OnboardingStep;
  done: boolean;
}

/** The five steps, each done or not, and how many are done. */
export function onboardingSteps(o: Onboarding): { steps: StepState[]; done: number } {
  const steps: StepState[] = [
    { step: 'projects', done: o.projects },
    { step: 'users', done: o.otherUsers },
    { step: 'mail', done: o.mail },
    { step: 'boards', done: o.boards },
    { step: 'issues', done: o.issues },
  ];
  return { steps, done: steps.filter((s) => s.done).length };
}

/** Whether the checklist shows: not hidden, and a step still to do. */
export function showOnboarding(o: Onboarding | null): o is Onboarding {
  return !!o && !o.dismissed && onboardingSteps(o).done < 5;
}
