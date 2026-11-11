/** Where the source code is, as the AGPL asks every user to be told. */
export const SOURCE_URL = 'https://github.com/mflr0/Solstice';

/** The licences of what ships with the application (frontend/public/licenses). */
export const LICENSES_PATH = '/licenses/README.txt';

const GUIDE_ANCHORS = {
  en: { ldap: 'ldap-and-active-directory', oidc: 'single-sign-on-openid-connect' },
  fr: { ldap: 'ldap-et-active-directory', oidc: 'connexion-unique-openid-connect' },
};

/** A section of the administration guide, in French or else in English. */
export function adminGuideURL(locale: string, section: 'ldap' | 'oidc'): string {
  const lang = locale === 'fr' ? 'fr' : 'en';
  return `${SOURCE_URL}/blob/main/docs/${lang}/administration.md#${GUIDE_ANCHORS[lang][section]}`;
}
