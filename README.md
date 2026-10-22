
# --** Password Admin **-- 

Aplicación de escritorio hecha con Go y Wails. La bóveda persistente se guarda cifrada en un repositorio privado de GitHub; SQLite funciona únicamente en memoria mientras la aplicación está abierta. La interfaz está hecha con HTML, CSS y JavaScript, sin un servidor web externo.

## Configurar GitHub por primera vez

### 1. Crear el repositorio privado

En GitHub, crea un repositorio **Private**. Puede estar vacío; no hace falta inicializarlo con README, licencia ni `.gitignore`.

Por ejemplo, para `https://github.com/juanperez/password-vault`, los campos de Password Admin son:

| Campo en la aplicación | Valor |
| --- | --- |
| Propietario del repositorio | `juanperez` |
| Nombre del repositorio | `password-vault` |

Usa solamente el usuario o la organización en el primer campo, y el nombre del repositorio en el segundo. **No pegues la URL completa ni agregues `.git`.**

### 2. Crear un token fine-grained

En GitHub, abre **Settings → Developer settings → Personal access tokens → Fine-grained tokens → Generate new token** y configura:

1. **Resource owner:** el usuario o la organización dueña del repositorio.
2. **Repository access:** selecciona **Only select repositories** y elige el repositorio privado creado arriba.
3. **Repository permissions → Contents:** **Read and write**. No se necesitan permisos de Issues, Actions ni administración del repositorio.
4. Define una fecha de expiración que puedas mantener. Cuando el token expire, genera otro y vuelve a conectar el repositorio.

Copia el token al generarlo: GitHub no vuelve a mostrarlo. No lo pongas en el código, en el README, en un comando de terminal ni en el repositorio.

### 3. Conectar Password Admin

1. Inicia Password Admin y completa **Propietario**, **Repositorio** y **Token fine-grained**.
2. Si es un repositorio nuevo, crea una contraseña maestra de al menos 8 caracteres. Si ya contiene la bóveda, escribe la contraseña maestra existente para desbloquearla.
3. Para migrar una bóveda SQLite anterior, conecta primero un repositorio vacío. En la pantalla de creación, escribe la contraseña maestra de la bóveda anterior y pulsa **Importar bóveda SQLite existente**. Selecciona el archivo original. La aplicación lo abre en solo lectura, sube una copia cifrada y conserva el original.

La aplicación crea `password-admin.vault` en el repositorio. Ese archivo contiene la bóveda cifrada; no lo edites manualmente.

## Llevar la configuración a otro dispositivo

El token no se guarda en el repositorio ni en texto plano. Tras crear o desbloquear la bóveda, Password Admin lo cifra con la contraseña maestra y guarda los siguientes archivos junto al ejecutable:

- `password-admin.json`: propietario y nombre del repositorio.
- `password-admin.credentials-*`: token cifrado. El sufijo identifica el repositorio configurado.

Copia esos archivos junto con el ejecutable al otro dispositivo y abre la aplicación. Usa la misma contraseña maestra. Mantén privados los archivos: quien obtenga el archivo de credenciales **y** la contraseña maestra podrá usar el token. Si no tienes los archivos, puedes conectar de nuevo el repositorio pegando un token válido.

La carpeta del ejecutable debe permitir escritura. Si Windows o Linux impide escribir allí, mueve la aplicación a una carpeta del usuario o con permisos de escritura. En desarrollo, los archivos se guardan junto al ejecutable que inicia Wails; normalmente queda en `build/bin`. Comprueba esa carpeta al mover una configuración de desarrollo.

Si la aplicación solicita de nuevo el token en otro equipo, verifica que copiaste tanto `password-admin.json` como el archivo `password-admin.credentials-*` que corresponde a ese repositorio. No renombres el archivo de credenciales.

## Seguridad

- La aplicación envía a GitHub una copia cifrada de todo el contenido de la bóveda. SQLite solo funciona en RAM durante la sesión. También cifra usuarios y contraseñas por campo con AES-256-GCM.
- La clave se deriva de la contraseña maestra mediante PBKDF2-HMAC-SHA-256 y una sal aleatoria. Sin la contraseña maestra no se puede descifrar la copia remota.
- El token fine-grained se cifra con la contraseña maestra y se guarda en un archivo `password-admin.credentials-*`, junto al ejecutable. La configuración del repositorio se guarda en `password-admin.json` en esa misma carpeta.
- La base de trabajo solo existe en RAM. Internet es necesario para descargar y sincronizar la bóveda. Si falla una sincronización, la aplicación avisa y evita cerrarse mientras haya cambios pendientes; un cierre forzado o apagón puede perder los cambios que no llegaron a GitHub.
- La contraseña maestra no se puede recuperar. Guárdala en un lugar seguro.
- El CSV de Chrome exportado contiene contraseñas en texto claro. Guárdalo con cuidado y elimínalo después de importarlo en Chrome.
- GitHub mantiene el historial del archivo. Protege el repositorio privado, limita el alcance del token y revócalo si deja de ser necesario.

## Requisitos de desarrollo

- Go 1.22 o posterior.
- Wails CLI v2. Instálalo con:

  ```text
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
  ```

- Windows: Microsoft Edge WebView2 Runtime y MinGW-w64 (GCC).
- macOS: Xcode Command Line Tools.
- Linux: GCC y las bibliotecas de desarrollo GTK3 y WebKit2GTK requeridas por Wails.

Consulta la guía oficial de instalación de Wails para los paquetes de tu sistema. SQLite usa `modernc.org/sqlite`, así que no requiere instalar SQLite.

El proyecto ya incluye `wails.json` y configura el servidor de desarrollo en el puerto local `45678`, evitando conflictos con puertos reservados en algunos equipos. No ejecutes `wails init`: inicializa un proyecto nuevo y no hace falta para abrir este.

## Ejecutar desde el código fuente

```text
wails dev
```

Configura el repositorio y el token siguiendo [Configurar GitHub por primera vez](#configurar-github-por-primera-vez). Después crea una contraseña maestra o desbloquea la bóveda existente. Para migrar una instalación anterior, usa **Importar bóveda SQLite existente**, selecciona el archivo `vault.sqlite` y escribe su contraseña maestra. El original local se conserva; solo se sube una copia cifrada.

La bóveda remota se descarga al iniciar y cada cambio se cifra y sincroniza con GitHub. Si una sincronización falla, no cierres la aplicación: restablece la conexión y pulsa **Guardar en GitHub**. La app detecta cambios remotos mediante el SHA de GitHub y evita sobrescribir una versión distinta. Está pensada para usarla en un dispositivo a la vez.

Si PowerShell no encuentra `wails` después de instalar el CLI, agrégalo al `PATH` de esa terminal y vuelve a ejecutar:

```powershell
$env:Path += ";$(go env GOPATH)\bin"
wails dev
```

No copies `password-admin.json` ni `password-admin.credentials-*` al control de versiones. En una carpeta de desarrollo, puedes ignorar esos archivos localmente con `.git/info/exclude` para no modificar las reglas compartidas del proyecto:

```text
/build/bin/password-admin.json
/build/bin/password-admin.credentials-*
```

## Uso

1. Crea una cuenta para agrupar las contraseñas, por ejemplo «Personal» o «Trabajo».
2. En Chrome, exporta las contraseñas como CSV.
3. Pulsa **Importar CSV** y elige el archivo. La aplicación pregunta a qué cuenta asignarlo; también puedes crear una cuenta nueva en ese paso.
4. Selecciona una cuenta para buscar y consultar sus entradas.
5. Pulsa **Exportar para Chrome** para guardar un CSV de la cuenta seleccionada. En Chrome, utiliza la opción para importar contraseñas.

La opción **Todas las cuentas** muestra y busca las entradas de toda la bóveda; cada fila indica a qué cuenta pertenece. Si esa vista está seleccionada, la exportación genera un solo CSV con las contraseñas de todas las cuentas. El CSV de Chrome no conserva la organización por cuentas; si lo vuelves a importar en Password Admin, tendrás que asignarlo a una cuenta.

Se reconocen los encabezados `url` y `password`; `name` y `username` son opcionales. El CSV se procesa correctamente aunque tenga comas, comillas o saltos de línea dentro de los valores.

## Compilar

Instala los requisitos de desarrollo de Wails para el sistema destino y ejecuta desde la raíz del proyecto:

### Windows

```powershell
wails build
```

El ejecutable se genera en `build/bin`.

### macOS

```sh
wails build
```

La compilación crea una aplicación para la arquitectura del Mac. Firma y notarización para distribución se configuran por separado.

### Linux

```sh
wails build
```

Instala antes GCC y las dependencias GTK3/WebKit2GTK de Wails.

Wails usa el WebView nativo de cada plataforma y sus dependencias de compilación; por eso, lo más sencillo y fiable es compilar por separado en cada sistema operativo de destino. El frontend está escrito directamente en HTML, CSS y JavaScript: no requiere Node.js ni un paso de compilación de frontend.

## Solución de problemas

- **GitHub devuelve 404 al conectar:** confirma que propietario y repositorio están escritos como nombres, no como URL; revisa que el token tenga acceso al repositorio privado elegido.
- **GitHub devuelve 401 o 403:** comprueba que el token no expiró y que tiene permiso **Contents: Read and write**.
- **No se puede guardar el token o la configuración:** habilita escritura en la carpeta del ejecutable o mueve la app a una carpeta del usuario. Para `wails dev`, revisa la carpeta del ejecutable de desarrollo, normalmente `build/bin`.
- **No se puede desbloquear en el segundo dispositivo:** verifica que copiaste juntos el ejecutable, `password-admin.json` y el archivo de credenciales correspondiente, y que usas la contraseña maestra correcta.
- **Hay conflictos con cambios de GitHub:** la app evita sobrescribir una versión remota distinta. Cierra la app en el otro dispositivo y vuelve a conectar para descargar la versión más reciente; no hagas dos cambios simultáneos en la bóveda.
- **No hay conexión a internet:** la bóveda persistente está en GitHub. No cierres la aplicación mientras haya cambios pendientes; cuando vuelva la conexión, pulsa **Guardar en GitHub**.

## Icono

La imagen fuente de la aplicación es `build/appicon.png`. Wails la usa para generar los iconos de la compilación para Windows y macOS; en Linux, la aplicación la usa como icono de ventana. Si ya existe `build/windows/icon.ico`, reemplázalo también con el ICO correspondiente para que Windows no siga usando el icono generado anteriormente. Después ejecuta `wails build` otra vez.
---
