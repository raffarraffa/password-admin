import {
  Accounts,
  ConfigureGitHub,
  CreateAccount,
  DeletePassword,
  ExportCSV,
  ImportCSV,
  ImportLocalVault,
  PasswordDetails,
  Passwords,
  PreviewCSV,
  SetupVault,
  SyncVault,
  UnlockVault,
  VaultStatus,
} from "./backend.js";

const $ = (selector) => document.querySelector(selector);
const gate = $("#gate");
const githubGate = $("#github-gate");
const githubForm = $("#github-form");
const vault = $("#vault");
const gateForm = $("#gate-form");
const modal = $("#modal");
const modalForm = $("#modal-form");
const toast = $("#toast");

let accounts = [];
let entries = [];
let selectedAccountID = null;
let selectedEntryID = 0;
let selectedEntry = null;
let importContent = "";
let toastTimer;
let returnToVault = false;
let githubConfig = {};

function notify(message, isError = false) {
  toast.textContent = message;
  toast.classList.toggle("error", isError);
  toast.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.classList.remove("show"), 3400);
}

function reportError(error) {
  notify(error?.message || String(error), true);
}

function showGate(initialized) {
  githubGate.classList.add("hidden");
  gate.classList.remove("hidden");
  vault.classList.add("hidden");
  $("#gate-title").textContent = initialized ? "Bienvenido de nuevo" : "Crea tu bóveda";
  $("#gate-description").textContent = initialized
    ? "Ingresa tu contraseña maestra para acceder a tus cuentas."
    : "Elige una contraseña maestra para proteger tus credenciales. No se puede recuperar.";
  $("#confirm-wrap").classList.toggle("hidden", initialized);
  $("#master-password").autocomplete = initialized ? "current-password" : "new-password";
  $("#gate-submit").textContent = initialized ? "Desbloquear bóveda" : "Crear bóveda";
  gateForm.dataset.initialized = String(initialized);
  $("#import-existing").classList.toggle("hidden", initialized);
  $("#master-password").focus();
}

function showGitHubSetup(config = {}, allowCancel = false) {
  gate.classList.add("hidden");
  vault.classList.add("hidden");
  githubGate.classList.remove("hidden");
  $("#github-owner").value = config.owner || "";
  $("#github-repo").value = config.repo || "";
  $("#github-token").value = "";
  $("#github-cancel").classList.toggle("hidden", !allowCancel);
  $("#github-owner").focus();
}

async function showVault() {
  githubGate.classList.add("hidden");
  gate.classList.add("hidden");
  vault.classList.remove("hidden");
  $("#sync-status").textContent = "Bóveda cifrada en GitHub";
  await refreshAccounts();
}

githubForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const submit = githubForm.querySelector('button[type="submit"]');
  submit.disabled = true;
  try {
    const status = await ConfigureGitHub(
      $("#github-owner").value,
      $("#github-repo").value,
      $("#github-token").value,
    );
    githubConfig = status.config || {};
    githubForm.reset();
    returnToVault = false;
    showGate(status.initialized);
    notify("Repositorio privado conectado.");
  } catch (error) {
    reportError(error);
  } finally {
    submit.disabled = false;
  }
});

$("#github-cancel").addEventListener("click", async () => {
  if (!returnToVault) return;
  returnToVault = false;
  try {
    await showVault();
  } catch (error) {
    reportError(error);
  }
});

gateForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const password = $("#master-password").value;
  const initialized = gateForm.dataset.initialized === "true";
  try {
    if (initialized) {
      await UnlockVault(password);
    } else {
      if (Array.from(password).length < 8) {
        throw new Error("La contraseña maestra debe tener al menos 8 caracteres.");
      }
      if (password !== $("#confirm-password").value) {
        throw new Error("Las contraseñas no coinciden.");
      }
      await SetupVault(password);
    }
    gateForm.reset();
    await showVault();
  } catch (error) {
    try {
      const status = await VaultStatus();
      githubConfig = status.config || githubConfig;
      if (status.unlocked) {
        gateForm.reset();
        await showVault();
      }
    } catch (statusError) {
      reportError(statusError);
      return;
    }
    reportError(error);
  }
});

$("#import-existing").addEventListener("click", async () => {
  const password = $("#master-password").value;
  if (!password) {
    notify("Ingresa la contraseña maestra de la bóveda SQLite que vas a importar.", true);
    $("#master-password").focus();
    return;
  }
  try {
    await ImportLocalVault(password);
    const status = await VaultStatus();
    githubConfig = status.config || githubConfig;
    if (!status.unlocked) {
      notify("No se importó ninguna bóveda.", true);
      return;
    }
    gateForm.reset();
    await showVault();
    notify("Bóveda local importada y guardada en GitHub.");
  } catch (error) {
    try {
      const status = await VaultStatus();
      githubConfig = status.config || githubConfig;
      if (status.unlocked) {
        gateForm.reset();
        await showVault();
      }
    } catch (statusError) {
      reportError(statusError);
      return;
    }
    reportError(error);
  }
});

$("#sync-button").addEventListener("click", async () => {
  try {
    await SyncVault();
    $("#sync-status").textContent = "Guardado en GitHub";
    notify("Bóveda guardada en GitHub.");
  } catch (error) {
    $("#sync-status").textContent = "Error al guardar";
    reportError(error);
  }
});

$("#github-settings").addEventListener("click", () => {
  returnToVault = true;
  showGitHubSetup(githubConfig, true);
});

if (window.runtime?.EventsOn) {
  window.runtime.EventsOn("vault-sync-error", (message) => {
    $("#sync-status").textContent = "Error al guardar";
    notify(message, true);
  });
}

async function refreshAccounts(preferredAccountName = "") {
  accounts = await Accounts();
  if (preferredAccountName) {
    selectedAccountID = accounts.find((account) => account.name === preferredAccountName)?.id ?? null;
  }
  if (selectedAccountID === null ||
      (selectedAccountID !== 0 && !accounts.some((account) => account.id === selectedAccountID))) {
    selectedAccountID = accounts[0]?.id ?? null;
  }
  if (accounts.length) {
    await selectAccount(selectedAccountID);
  } else {
    selectedAccountID = null;
    entries = [];
    $("#account-title").textContent = "Selecciona una cuenta";
    renderAccounts();
    renderEntries();
    renderDetails();
  }
}

function renderAccounts() {
  const list = $("#account-list");
  list.replaceChildren();
  const addAccountItem = (name, id, count, iconText) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `account-item${id === selectedAccountID ? " active" : ""}`;
    button.setAttribute("aria-current", id === selectedAccountID ? "page" : "false");
    const icon = document.createElement("span");
    icon.className = "account-icon";
    icon.textContent = iconText;
    const label = document.createElement("span");
    label.textContent = name;
    const countLabel = document.createElement("span");
    countLabel.className = "account-count";
    countLabel.textContent = String(count);
    button.append(icon, label, countLabel);
    button.addEventListener("click", () => selectAccount(id));
    list.append(button);
  };
  if (accounts.length) {
    const totalCount = accounts.reduce((sum, account) => sum + account.passwordCount, 0);
    addAccountItem("Todas las cuentas", 0, totalCount, "✳");
  }
  for (const account of accounts) {
    addAccountItem(account.name, account.id, account.passwordCount, "◈");
  }
}

async function selectAccount(id) {
  selectedAccountID = id;
  selectedEntryID = 0;
  selectedEntry = null;
  const account = accounts.find((item) => item.id === id);
  $("#account-title").textContent = id === 0 ? "Todas las cuentas" : account?.name || "Selecciona una cuenta";
  renderAccounts();
  entries = await Passwords(id);
  renderEntries();
  renderDetails();
}

function renderEntries() {
  const query = $("#search").value.trim().toLocaleLowerCase();
  const filtered = entries.filter((entry) =>
    `${entry.name} ${entry.url} ${entry.username} ${entry.accountName || ""}`.toLocaleLowerCase().includes(query),
  );
  const list = $("#entries");
  list.replaceChildren();
  $("#entry-count").textContent = `${filtered.length} ${filtered.length === 1 ? "entrada" : "entradas"}`;
  $("#empty-state").classList.toggle("hidden", filtered.length > 0);
  $("#empty-title").textContent = entries.length ? "Sin resultados" : "Tu bóveda empieza aquí";
  $("#empty-description").textContent = entries.length
    ? "Prueba con otro sitio o nombre de usuario."
    : "Importa un CSV de Chrome para guardar las contraseñas en una cuenta.";
  $("#empty-import").classList.toggle("hidden", entries.length > 0);

  for (const entry of filtered) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = `entry-row${entry.id === selectedEntryID ? " selected" : ""}`;
    const site = document.createElement("span");
    site.className = "entry-site";
    const title = document.createElement("strong");
    title.textContent = entry.name || entry.url;
    const url = document.createElement("span");
    url.textContent = entry.url;
    site.append(title, url);
    if (selectedAccountID === 0) {
      const accountName = document.createElement("span");
      accountName.className = "entry-account";
      accountName.textContent = entry.accountName;
      site.append(accountName);
    }
    const username = document.createElement("span");
    username.className = "entry-user";
    username.textContent = entry.username || "Sin usuario";
    row.append(site, username);
    row.addEventListener("click", () => {
      selectedEntryID = entry.id;
      selectedEntry = null;
      renderEntries();
      renderDetails();
      PasswordDetails(entry.id).then((details) => {
        if (selectedEntryID !== entry.id) return;
        selectedEntry = details;
        renderDetails(details);
      }).catch(reportError);
    });
    list.append(row);
  }
  if (!filtered.some((entry) => entry.id === selectedEntryID)) {
    selectedEntryID = 0;
    selectedEntry = null;
    renderDetails();
  }
}

function renderDetails(entry = selectedEntry) {
  const panel = $("#details");
  panel.replaceChildren();
  if (!entry) {
    const placeholder = document.createElement("div");
    placeholder.className = "detail-placeholder";
    const icon = document.createElement("div");
    icon.className = "empty-icon";
    icon.textContent = "◇";
    const text = document.createElement("p");
    text.textContent = "Selecciona una contraseña para ver sus detalles.";
    placeholder.append(icon, text);
    panel.append(placeholder);
    return;
  }

  const title = document.createElement("h2");
  title.className = "detail-title";
  title.textContent = entry.name || entry.url;
  panel.append(title);
  const addField = (label, value, copy = false) => {
    const field = document.createElement("div");
    field.className = "detail-field";
    const heading = document.createElement("label");
    heading.textContent = label;
    const valueBox = document.createElement("div");
    valueBox.className = "detail-value";
    const text = document.createElement("span");
    text.className = "value-text";
    text.textContent = value || "—";
    valueBox.append(text);
    if (copy && value) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "mini-button";
      button.textContent = "Copiar";
      button.addEventListener("click", async () => {
        try {
          await navigator.clipboard.writeText(value);
          notify(`${label} copiado al portapapeles.`);
        } catch {
          reportError(new Error("No se pudo copiar al portapapeles."));
        }
      });
      valueBox.append(button);
    }
    field.append(heading, valueBox);
    panel.append(field);
    return text;
  };
  addField("Sitio web", entry.url, true);
  addField("Nombre de usuario", entry.username, true);
  const passwordText = addField("Contraseña", "••••••••••••");
  let revealed = false;
  const passwordButtons = document.createElement("div");
  passwordButtons.className = "detail-field";
  passwordButtons.style.display = "flex";
  passwordButtons.style.gap = "7px";
  const reveal = document.createElement("button");
  reveal.type = "button";
  reveal.className = "mini-button";
  reveal.textContent = "Mostrar";
  reveal.addEventListener("click", () => {
    revealed = !revealed;
    passwordText.textContent = revealed ? entry.password : "••••••••••••";
    reveal.textContent = revealed ? "Ocultar" : "Mostrar";
  });
  const copyPassword = document.createElement("button");
  copyPassword.type = "button";
  copyPassword.className = "mini-button";
  copyPassword.textContent = "Copiar contraseña";
  copyPassword.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(entry.password);
      notify("Contraseña copiada al portapapeles.");
    } catch {
      reportError(new Error("No se pudo copiar al portapapeles."));
    }
  });
  passwordButtons.append(reveal, copyPassword);
  panel.append(passwordButtons);

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "delete-button";
  remove.textContent = "Eliminar contraseña";
  remove.addEventListener("click", async () => {
    if (!window.confirm("¿Eliminar esta contraseña de la bóveda?")) return;
    try {
      await DeletePassword(entry.id);
      selectedEntryID = 0;
      await selectAccount(selectedAccountID);
      notify("Contraseña eliminada.");
    } catch (error) {
      try {
        await refreshAccounts();
      } catch {
        // The original mutation error is more useful than a refresh failure here.
      }
      reportError(error);
    }
  });
  panel.append(remove);
}

$("#search").addEventListener("input", renderEntries);

function openModal(title, build, onConfirm, confirmLabel = "Continuar") {
  $("#modal-title").textContent = title;
  $("#modal-submit").textContent = confirmLabel;
  const content = $("#modal-content");
  content.replaceChildren();
  build(content);
  modal.returnValue = "cancel";
  modal.showModal();
  modal.onclose = () => {
    if (modal.returnValue !== "confirm") importContent = "";
  };
  modalForm.onsubmit = async (event) => {
    event.preventDefault();
    if (event.submitter?.value !== "confirm") {
      modal.close("cancel");
      return;
    }
    try {
      const shouldClose = await onConfirm(content);
      if (shouldClose !== false) modal.close("confirm");
    } catch (error) {
      try {
        await refreshAccounts();
      } catch {
        // Preserve the operation's error if refreshing also fails.
      }
      reportError(error);
    }
  };
}

$("#add-account").addEventListener("click", () => {
  let nameInput;
  openModal("Crear una cuenta", (content) => {
    const label = document.createElement("label");
    label.textContent = "Nombre de la cuenta";
    nameInput = document.createElement("input");
    nameInput.placeholder = "Por ejemplo: Personal";
    nameInput.required = true;
    content.append(label, nameInput);
  }, async () => {
    const account = await CreateAccount(nameInput.value);
    selectedAccountID = account.id;
    await refreshAccounts();
    notify("Cuenta creada.");
  }, "Crear cuenta");
});

async function selectCsvFile() {
  $("#csv-file").value = "";
  $("#csv-file").click();
}
$("#import-button").addEventListener("click", selectCsvFile);
$("#empty-import").addEventListener("click", selectCsvFile);

$("#csv-file").addEventListener("change", async (event) => {
  const file = event.target.files?.[0];
  if (!file) return;
  try {
    importContent = await file.text();
    const count = await PreviewCSV(importContent);
    let accountSelect;
    let newName;
    openModal("Asignar cuenta", (content) => {
      const description = document.createElement("p");
      description.textContent = `Se importarán ${count} contraseñas desde "${file.name}". Elige la cuenta a la que pertenecen.`;
      const label = document.createElement("label");
      label.textContent = "Cuenta";
      accountSelect = document.createElement("select");
      for (const account of accounts) {
        const option = document.createElement("option");
        option.value = String(account.id);
        option.textContent = account.name;
        accountSelect.append(option);
      }
      const newOption = document.createElement("option");
      newOption.value = "new";
      newOption.textContent = "Crear una cuenta nueva...";
      accountSelect.append(newOption);
      accountSelect.value = accounts.length ? String(selectedAccountID || accounts[0].id) : "new";
      const nameLabel = document.createElement("label");
      nameLabel.textContent = "Nombre de la cuenta nueva";
      newName = document.createElement("input");
      newName.placeholder = "Por ejemplo: Trabajo";
      newName.classList.toggle("hidden", accountSelect.value !== "new");
      nameLabel.classList.toggle("hidden", accountSelect.value !== "new");
      accountSelect.addEventListener("change", () => {
        const creating = accountSelect.value === "new";
        newName.classList.toggle("hidden", !creating);
        nameLabel.classList.toggle("hidden", !creating);
      });
      content.append(description, label, accountSelect, nameLabel, newName);
    }, async () => {
      const creating = accountSelect.value === "new";
      const result = await ImportCSV(importContent, creating ? 0 : Number(accountSelect.value), creating ? newName.value : "");
      importContent = "";
      await refreshAccounts(result.accountName);
      notify(`Se importaron ${result.count} contraseñas a "${result.accountName}".`);
    }, "Importar");
  } catch (error) {
    importContent = "";
    reportError(error);
  }
});

$("#export-button").addEventListener("click", async () => {
  if (!accounts.length) {
    notify("Crea o selecciona una cuenta antes de exportar.", true);
    return;
  }
  const scope = selectedAccountID === 0 ? "todas las cuentas" : "la cuenta seleccionada";
  if (!window.confirm(`Se exportarán las contraseñas de ${scope} en un CSV sin cifrar. Guárdalo de forma segura y elimínalo después de importarlo en Chrome.`)) {
    return;
  }
  try {
    if (await ExportCSV(selectedAccountID)) notify("Exportación completada.");
  } catch (error) {
    reportError(error);
  }
});

try {
  const status = await VaultStatus();
  githubConfig = status.config || {};
  if (status.unlocked) {
    await showVault();
  } else if (!status.configured || !status.hasToken) {
    showGitHubSetup(status.config);
  } else {
    showGate(status.initialized);
  }
} catch (error) {
  showGitHubSetup();
  reportError(error);
}
