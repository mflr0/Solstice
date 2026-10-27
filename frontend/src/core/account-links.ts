/**
 * The pages of the links sent by e-mail (forgotten password, invitation) and
 * the administration's options around them: what to check before sending, and
 * when an option is offered. Problems are keys; the pages word them
 * (src/i18n).
 */

/** Whether a password is shorter than `min`, counted in characters as the server does. */
export function passwordTooShort(password: string, min: number): boolean {
  return [...password].length < min;
}

/** What the form to choose a password can get wrong. */
export type ChoosePasswordProblem = 'tooShort' | 'mismatch';

/**
 * Problems of the form that sets a password through a link: the length the
 * server asks for (counted in characters, as it does) and the confirmation.
 * Nothing is reported while a field is still empty: the page only checks when
 * something was typed.
 */
export function choosePasswordProblem(
  password: string,
  confirm: string,
  min: number,
): ChoosePasswordProblem | null {
  if (passwordTooShort(password, min)) return 'tooShort';
  if (password !== confirm) return 'mismatch';
  return null;
}

/** Whether the form to ask for a link has something to send. */
export function canAskForLink(identifier: string): boolean {
  return identifier.trim() !== '';
}

/** What the link's page is about. */
export type LinkPurpose = 'reset' | 'invite';

export function isLinkPurpose(v: unknown): v is LinkPurpose {
  return v === 'reset' || v === 'invite';
}

/** The account facts the administration's options depend on. */
export interface LinkAccount {
  active: boolean;
  email: string;
  /** null: never signed in; undefined: an older server that does not say. */
  lastLogin?: string | null;
  /** "ldap", "oidc": the directory or the provider signs it in, Solstice sets no password. */
  directory?: string;
}

/**
 * Whether an administrator may send a link to choose a password: e-mail links
 * work on this instance, the account may sign in, has an address, and its
 * password is Solstice's (not a directory's).
 */
export function canSendResetLink(a: LinkAccount, emailLinks: boolean): boolean {
  return emailLinks && a.active && a.email.trim() !== '' && !a.directory;
}

/**
 * Whether an invitation can be sent again: the same, and the person never
 * signed in (an account that did has "send a link" instead).
 */
export function canResendInvitation(a: LinkAccount, emailLinks: boolean): boolean {
  return canSendResetLink(a, emailLinks) && a.lastLogin === null;
}

/** The ways to give an account a new password. */
export type PasswordMode = 'generate' | 'type' | 'link';

export function passwordModes(a: LinkAccount, emailLinks: boolean): PasswordMode[] {
  return canSendResetLink(a, emailLinks) ? ['generate', 'type', 'link'] : ['generate', 'type'];
}
