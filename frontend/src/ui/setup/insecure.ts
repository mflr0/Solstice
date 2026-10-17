/**
 * Whether the wizard's page is reached over plain HTTP by another machine:
 * the passwords it asks for would cross the network readable.
 */
export function insecurePage(protocol: string, hostname: string): boolean {
  if (protocol !== 'http:') return false;
  return hostname !== 'localhost' && !/^127(\.\d{1,3}){3}$/.test(hostname) && hostname !== '[::1]';
}
