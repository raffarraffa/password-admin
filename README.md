
# --** Password Admin **-- 

Aplicación web local hecha con Go, HTML, CSS y JavaScript. El ejecutable sirve la interfaz y la API en `127.0.0.1`; Nginx puede actuar como proxy inverso. La bóveda persistente se guarda cifrada en un repositorio privado de GitHub y SQLite funciona en memoria mientras el servidor está abierto.

## Iniciar el servidor web

Por defecto, Password Admin escucha únicamente en `http://127.0.0.1:45678/`. Para publicarlo bajo una subruta, por ejemplo `https://example.com/pass-admin/`, configura también `BASE_PATH`. Crea un archivo `.env` en la carpeta desde la que inicias el ejecutable o junto al ejecutable:

```env
PORT=45678
BASE_PATH=/pass-admin
```

El proceso lee `.env` al iniciar. Las variables de entorno `PORT` y `BASE_PATH`, si están definidas, tienen prioridad sobre el archivo. Si no se configura `PORT`, se usa `45678`. `BASE_PATH` debe ser una ruta absoluta sencilla, sin barra final (por ejemplo, `/pass-admin`); si se omite o vale `/`, la aplicación se publica en la raíz. El servidor agrega la barra final y el HTML fija ese prefijo como base para que CSS, JavaScript y API sigan en `/pass-admin/`, incluso si alguien entra por `https://example.com/pass-admin` sin la barra final.

Para compilar y ejecutar un binario (también sirve para desarrollo):

```powershell
go build -o PasswordAdmin.exe .
.\PasswordAdmin.exe
```

Abre en el navegador la dirección que imprime el servidor. Para Nginx, configura el upstream hacia `http://127.0.0.1:<PORT>` y conserva el `Host` original. El `BASE_PATH` configurado en `.env` debe coincidir con el prefijo público. El backend acepta tanto el prefijo conservado como el prefijo eliminado por Nginx, así que funcionan ambas variantes de `proxy_pass`. **No publiques el upstream sin protección**: el backend sirve la bóveda desbloqueada del proceso. Usa HTTPS y restringe el acceso en Nginx con autenticación y/o una VPN. Por ejemplo, dentro de un `server` HTTPS para `/pass-admin/`, esta variante conserva el prefijo:

```nginx
location = /pass-admin {
    return 301 /pass-admin/;
}

location /pass-admin/ {
    auth_basic "Password Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://127.0.0.1:45678;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Sin barra final en `proxy_pass`, Nginx conserva `/pass-admin/`; con barra final, elimina ese prefijo antes de reenviar. El backend sirve la interfaz, CSS, JavaScript y API en ambos casos. Mantén el servidor Go escuchando en la interfaz local.

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

1. Abre Password Admin en el navegador y completa **Propietario**, **Repositorio** y **Token fine-grained**.
2. Si es un repositorio nuevo, crea una contraseña maestra de al menos 8 caracteres. Si ya contiene la bóveda, escribe la contraseña maestra existente para desbloquearla.
3. Para migrar una bóveda SQLite anterior, conecta primero un repositorio vacío. En la pantalla de creación, escribe la contraseña maestra de la bóveda anterior y pulsa **Importar bóveda SQLite existente**. Selecciona el archivo original. La aplicación lo abre en solo lectura, sube una copia cifrada y conserva el original.

La aplicación crea `password-admin.vault` en el repositorio. Ese archivo contiene la bóveda cifrada; no lo edites manualmente.

## Llevar la configuración a otro dispositivo

El token no se guarda en el repositorio ni en texto plano. Tras crear o desbloquear la bóveda, Password Admin lo cifra con la contraseña maestra y guarda los siguientes archivos junto al ejecutable:

- `password-admin.json`: propietario y nombre del repositorio.
- `password-admin.credentials-*`: token cifrado. El sufijo identifica el repositorio configurado.

Copia esos archivos junto con el ejecutable al otro dispositivo y abre la aplicación. Usa la misma contraseña maestra. Mantén privados los archivos: quien obtenga el archivo de credenciales **y** la contraseña maestra podrá usar el token. Si no tienes los archivos, puedes conectar de nuevo el repositorio pegando un token válido.

La carpeta del ejecutable debe permitir escritura. Si Windows o Linux impide escribir allí, mueve la aplicación a una carpeta del usuario o con permisos de escritura. Los archivos de configuración se guardan junto al ejecutable; en desarrollo, se guardan junto al binario temporal generado por `go run`.

Si la aplicación solicita de nuevo el token en otro equipo, verifica que copiaste tanto `password-admin.json` como el archivo `password-admin.credentials-*` que corresponde a ese repositorio. No renombres el archivo de credenciales.

## Seguridad

- La aplicación envía a GitHub una copia cifrada de todo el contenido de la bóveda. SQLite solo funciona en RAM durante la sesión. También cifra usuarios y contraseñas por campo con AES-256-GCM.
- La clave se deriva de la contraseña maestra mediante PBKDF2-HMAC-SHA-256 y una sal aleatoria. Sin la contraseña maestra no se puede descifrar la copia remota.
- El token fine-grained se cifra con la contraseña maestra y se guarda en un archivo `password-admin.credentials-*`, junto al ejecutable. La configuración del repositorio se guarda en `password-admin.json` en esa misma carpeta.
- La base de trabajo solo existe en RAM. Internet es necesario para descargar y sincronizar la bóveda. Si falla una sincronización, la interfaz avisa y permite reintentar; un cierre forzado o apagón puede perder los cambios que no llegaron a GitHub.
- La interfaz web no añade autenticación de usuarios/sesiones: si usas Nginx para dar acceso remoto, configura allí controles de acceso fuertes. El servidor Go escucha en `127.0.0.1` de forma predeterminada.
- La contraseña maestra no se puede recuperar. Guárdala en un lugar seguro.
- El CSV de Chrome exportado contiene contraseñas en texto claro. Guárdalo con cuidado y elimínalo después de importarlo en Chrome.
- GitHub mantiene el historial del archivo. Protege el repositorio privado, limita el alcance del token y revócalo si deja de ser necesario.

## Requisitos de desarrollo

- Go 1.22 o posterior.
- No se requiere Wails, Node.js ni instalar SQLite: el frontend no necesita compilación y SQLite usa `modernc.org/sqlite`.

Configura el repositorio y el token siguiendo [Configurar GitHub por primera vez](#configurar-github-por-primera-vez). Después crea una contraseña maestra o desbloquea la bóveda existente. Para migrar una instalación anterior, pulsa **Importar bóveda SQLite existente**, selecciona el archivo `vault.sqlite` desde el navegador e ingresa su contraseña maestra. El original local se conserva; solo se sube una copia cifrada.

La bóveda remota se descarga al iniciar y cada cambio se cifra y sincroniza con GitHub. Si una sincronización falla, no cierres la aplicación: restablece la conexión y pulsa **Guardar en GitHub**. La app detecta cambios remotos mediante el SHA de GitHub y evita sobrescribir una versión distinta. Está pensada para usarla en un dispositivo a la vez.

No copies `password-admin.json` ni `password-admin.credentials-*` al control de versiones. El archivo `.env` también está excluido del repositorio.

## Uso

1. Crea una cuenta para agrupar las contraseñas, por ejemplo «Personal» o «Trabajo».
2. En Chrome, exporta las contraseñas como CSV.
3. Pulsa **Importar CSV** y elige el archivo. La aplicación pregunta a qué cuenta asignarlo; también puedes crear una cuenta nueva en ese paso.
4. Selecciona una cuenta para buscar y consultar sus entradas.
5. Pulsa **Exportar para Chrome** para guardar un CSV de la cuenta seleccionada. En Chrome, utiliza la opción para importar contraseñas.

La opción **Todas las cuentas** muestra y busca las entradas de toda la bóveda; cada fila indica a qué cuenta pertenece. Si esa vista está seleccionada, la exportación genera un solo CSV con las contraseñas de todas las cuentas. El CSV de Chrome no conserva la organización por cuentas; si lo vuelves a importar en Password Admin, tendrás que asignarlo a una cuenta.

Se reconocen los encabezados `url` y `password`; `name` y `username` son opcionales. El CSV se procesa correctamente aunque tenga comas, comillas o saltos de línea dentro de los valores.

## Compilar para Linux y Windows

Instala Go 1.22 o posterior y ejecuta los comandos desde la raíz del proyecto. No hace falta Wails, Node.js, SQLite externo ni compilar el frontend. Los ejemplos generan binarios para Intel/AMD de 64 bits (`amd64`); para un servidor Linux ARM de 64 bits, cambia `GOARCH=amd64` por `GOARCH=arm64`.

### Compilar en Windows

En PowerShell:

```powershell
New-Item -ItemType Directory -Force .\dist | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -o .\dist\PasswordAdmin-windows-amd64.exe .
if ($LASTEXITCODE -ne 0) { throw "Falló la compilación para Windows" }
Copy-Item .env .\dist\.env
```

Para iniciarlo, ejecuta `.\dist\PasswordAdmin-windows-amd64.exe`. El archivo `.env` de `dist` configura el puerto. Si lo ejecutas desde otra carpeta, copia también `.env` junto al ejecutable o define `PORT` en el entorno de Windows antes de iniciarlo.

### Compilar para Linux desde Windows

Go permite generar el binario Linux desde Windows sin instalar herramientas de compilación de Linux:

```powershell
New-Item -ItemType Directory -Force .\dist | Out-Null
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -o .\dist\password-admin-linux-amd64 .
if ($LASTEXITCODE -ne 0) { throw "Falló la compilación para Linux" }
Copy-Item .env .\dist\.env
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

El ejecutable Linux no es un `.exe`: transfiérelo junto con `dist/.env` a una máquina Linux de la misma arquitectura. Por ejemplo, instala ambos archivos en una carpeta propia:

```sh
mkdir -p "$HOME/password-admin"
cp password-admin-linux-amd64 .env "$HOME/password-admin/"
cd "$HOME/password-admin"
chmod 700 password-admin-linux-amd64
./password-admin-linux-amd64
```

El servidor imprime la URL local. Déjalo ejecutándose para que Nginx pueda conectarse al upstream `127.0.0.1:45678`. La carpeta del ejecutable debe permitir escritura porque allí se guardan la configuración de GitHub y las credenciales cifradas. Mantén el `.env` junto al ejecutable (o inicia el proceso desde la carpeta que lo contiene) y restringe el acceso a esos archivos.

### Ejecutar en segundo plano como servicio de Linux

En un servidor con `systemd`, instala el binario y `.env` en una carpeta persistente. El servicio se ejecuta con un usuario propio y se reinicia automáticamente si el proceso falla:

```sh
sudo useradd --system --create-home --home-dir /opt/password-admin --shell /usr/sbin/nologin password-admin
sudo install -o password-admin -g password-admin -m 700 password-admin-linux-amd64 /opt/password-admin/password-admin
sudo install -o password-admin -g password-admin -m 600 .env /opt/password-admin/.env
```

Crea `/etc/systemd/system/password-admin.service`:

```ini
[Unit]
Description=Password Admin web server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=password-admin
Group=password-admin
WorkingDirectory=/opt/password-admin
ExecStart=/opt/password-admin/password-admin
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/password-admin

[Install]
WantedBy=multi-user.target
```

Activa e inicia el servicio:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now password-admin
sudo systemctl status password-admin
```

Consulta los logs en vivo con `sudo journalctl -u password-admin -f`. La aplicación escucha en `127.0.0.1:45678` de acuerdo con el `.env`; Nginx puede usar esa dirección como upstream. Para cambiar el puerto, edita `/opt/password-admin/.env` y reinicia con `sudo systemctl restart password-admin`.

El usuario del servicio necesita permiso de escritura en `/opt/password-admin`: allí se guardan la configuración de GitHub y las credenciales cifradas. No ejecutes simultáneamente otra copia de la aplicación con la misma configuración, porque ambas compartirían la misma bóveda en memoria.

También puedes compilar en Linux directamente:

```sh
mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/password-admin-linux-amd64 .
cp .env dist/.env
```

En producción, configura Nginx con HTTPS y autenticación; no expongas el servidor sin protección. Consulta el ejemplo de Nginx en [Iniciar el servidor web](#iniciar-el-servidor-web).

## Solución de problemas

- **GitHub devuelve 404 al conectar:** confirma que propietario y repositorio están escritos como nombres, no como URL; revisa que el token tenga acceso al repositorio privado elegido.
- **GitHub devuelve 401 o 403:** comprueba que el token no expiró y que tiene permiso **Contents: Read and write**.
- **No se puede guardar el token o la configuración:** habilita escritura en la carpeta del ejecutable o mueve la app a una carpeta del usuario.
- **El puerto no cambia:** reinicia el ejecutable y comprueba que `PORT` esté definido en `.env` o en el entorno del proceso. La variable de entorno tiene prioridad; revisa que el puerto no esté ocupado.
- **No se puede desbloquear en el segundo dispositivo:** verifica que copiaste juntos el ejecutable, `password-admin.json` y el archivo de credenciales correspondiente, y que usas la contraseña maestra correcta.
- **Hay conflictos con cambios de GitHub:** la app evita sobrescribir una versión remota distinta. Cierra la app en el otro dispositivo y vuelve a conectar para descargar la versión más reciente; no hagas dos cambios simultáneos en la bóveda.
- **No hay conexión a internet:** la bóveda persistente está en GitHub. No cierres la aplicación mientras haya cambios pendientes; cuando vuelva la conexión, pulsa **Guardar en GitHub**.
