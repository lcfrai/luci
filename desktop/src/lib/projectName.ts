/** Mirrors the portable folder-name contract; the server remains authoritative. */
export const projectNameHelp = "Use 1–64 letters, numbers, hyphens or underscores, starting with a letter or number. No spaces. This becomes the project folder name.";

export function projectNameError(value: string): string | null {
  const name = value.trim();
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(name)) return projectNameHelp;
  if (/^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])$/i.test(name)) {
    return "Choose another name: this name is reserved by Windows.";
  }
  return null;
}
