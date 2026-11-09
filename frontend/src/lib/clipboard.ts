/** Copies text; false when neither the clipboard nor the selection fallback worked. */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return copyBySelection(text);
  }
}

/** Plain http has no navigator.clipboard: copy from a throwaway field instead. */
function copyBySelection(text: string): boolean {
  const focused = document.activeElement;
  const field = document.createElement('textarea');
  field.value = text;
  field.style.position = 'fixed';
  field.style.opacity = '0';
  document.body.append(field);
  field.select();
  const copied = document.execCommand('copy');
  field.remove();
  if (focused instanceof HTMLElement) focused.focus();
  return copied;
}
