const app = () => window.go.main.App;

export const VaultStatus = () => app().VaultStatus();
export const SetupVault = (password) => app().SetupVault(password);
export const UnlockVault = (password) => app().UnlockVault(password);
export const Accounts = () => app().Accounts();
export const CreateAccount = (name) => app().CreateAccount(name);
export const Passwords = (accountID) => app().Passwords(accountID);
export const PasswordDetails = (entryID) => app().PasswordDetails(entryID);
export const PreviewCSV = (content) => app().PreviewCSV(content);
export const ImportCSV = (content, accountID, newAccountName) =>
  app().ImportCSV(content, accountID, newAccountName);
export const ExportCSV = (accountID) => app().ExportCSV(accountID);
export const DeletePassword = (id) => app().DeletePassword(id);
