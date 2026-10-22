async function request(endpoint, body = {}) {
  const response = await fetch(`/api/${endpoint}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    cache: "no-store",
  });
  if (!response.ok) {
    const result = await response.json().catch(() => null);
    throw new Error(result?.error || `Error del servidor (HTTP ${response.status}).`);
  }
  if (response.status === 204) return null;
  const contentType = response.headers.get("Content-Type") || "";
  return contentType.includes("application/json") ? response.json() : response;
}

export const VaultStatus = () => request("vault-status");
export const ConfigureGitHub = (owner, repo, token) =>
  request("configure-github", { owner, repo, token });
export const SetupVault = (password) => request("setup-vault", { password });
export const UnlockVault = (password) => request("unlock-vault", { password });
export async function ImportLocalVault(password, file) {
  const body = new FormData();
  body.append("password", password);
  body.append("vault", file, file.name);
  const response = await fetch("/api/import-local-vault", {
    method: "POST",
    body,
    cache: "no-store",
  });
  if (!response.ok) {
    const result = await response.json().catch(() => null);
    throw new Error(result?.error || `Error del servidor (HTTP ${response.status}).`);
  }
}
export const Accounts = () => request("accounts");
export const CreateAccount = (name) => request("create-account", { name });
export const Passwords = (accountID) => request("passwords", { accountID });
export const PasswordDetails = (entryID) => request("password-details", { entryID });
export const PreviewCSV = (content) => request("preview-csv", { content });
export const ImportCSV = (content, accountID, newAccountName) =>
  request("import-csv", { content, accountID, newAccountName });
export async function ExportCSV(accountID) {
  const response = await request("export-csv", { accountID });
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a");
  link.href = url;
  link.download = "chrome-passwords.csv";
  link.click();
  URL.revokeObjectURL(url);
  return true;
}
export const DeletePassword = (entryID) => request("delete-password", { entryID });
export const SyncVault = () => request("sync-vault");
