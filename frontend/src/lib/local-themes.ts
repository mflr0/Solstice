/**
 * The themes kept in this browser (IndexedDB, per browser profile). The server
 * knows nothing of them. public/theme-boot.js reads the same database before
 * the first paint; keep the two in step.
 */

import { localSkinId, type Skin, type ThemeManifest } from '../core/appearance.js';
import { readPackage, type PackageFiles } from '../core/local-theme.js';

export interface LocalTheme {
  /** The served id: local-<manifest id>. */
  id: Skin;
  manifest: ThemeManifest;
  css: string;
  /** The package as it was imported, for an export. */
  zip: Blob;
}

const DATABASE = 'solstice-themes';
const STORE = 'themes';

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DATABASE, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: 'id' });
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error ?? new Error('IndexedDB'));
  });
}

async function run<T>(
  mode: IDBTransactionMode,
  action: (store: IDBObjectStore) => IDBRequest<T>,
): Promise<T> {
  const db = await open();
  try {
    return await new Promise<T>((resolve, reject) => {
      const tx = db.transaction(STORE, mode);
      const req = action(tx.objectStore(STORE));
      tx.oncomplete = () => resolve(req.result);
      tx.onabort = tx.onerror = () => reject(tx.error ?? new Error('IndexedDB'));
    });
  } finally {
    db.close();
  }
}

export function listLocalThemes(): Promise<LocalTheme[]> {
  return run('readonly', (store) => store.getAll() as IDBRequest<LocalTheme[]>);
}

/** The stylesheet of a local theme; null when it isn't here or can't be read. */
export async function readLocalCss(id: Skin): Promise<string | null> {
  try {
    const found = await run(
      'readonly',
      (store) => store.get(id) as IDBRequest<LocalTheme | undefined>,
    );
    return found?.css ?? null;
  } catch {
    return null;
  }
}

export async function saveLocalTheme(theme: LocalTheme): Promise<void> {
  await run('readwrite', (store) => store.put(theme));
  // Asks the browser not to clear it under storage pressure; it may say no.
  void navigator.storage?.persist?.();
}

export async function deleteLocalTheme(id: Skin): Promise<void> {
  await run('readwrite', (store) => store.delete(id));
}

/** Unpacks a zip, without saving it: null when it is no theme package. */
export async function unpackLocalTheme(zip: Blob): Promise<LocalTheme | null> {
  // Loaded when needed, and synchronous: fflate's workers would need a blob: script the CSP refuses.
  const { unzipSync } = await import('fflate');
  let files: PackageFiles;
  try {
    files = unzipSync(new Uint8Array(await zip.arrayBuffer()));
  } catch {
    return null;
  }
  const read = readPackage(files);
  return read && { id: localSkinId(read.manifest.id), ...read, zip };
}
