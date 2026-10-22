# Password Admin

Aplicación web local hecha con Go, HTML, CSS y JavaScript. El ejecutable sirve la interfaz y la API en `127.0.0.1`; Nginx puede actuar como proxy inverso. La bóveda se conserva en un archivo SQLite del mismo servidor y no necesita GitHub ni conexión a Internet.

Esta versión crea una bóveda local nueva. No descarga ni migra automáticamente una bóveda que estuviera en GitHub; los datos remotos no se modifican.

## Iniciar el servidor web

Por defecto, Password Admin escucha únicamente en `http://127.0.0.1:45678`. Al iniciar, el proceso lee `.env` desde la carpeta actual o desde la carpeta del ejecutable. Las variables del entorno tienen prioridad sobre los valores del archivo.

Configura el puerto y la ubicación persistente de SQLite:

```env
PORT=45678
DB_PATH=./data/password-admin.sqlite
```

`DB_PATH` puede ser una ruta absoluta o relativa al directorio del ejecutable. Por ejemplo, `./data/password-admin.sqlite` guarda la base en la carpeta `data` junto al ejecutable, sin depender de desde dónde se inicia el proceso. Si no se configura, la base se crea como `password-admin.sqlite` junto al ejecutable. La aplicación crea la carpeta de datos si no existe. Detén la aplicación antes de copiar o respaldar la base de datos.

Para compilar y ejecutar en Windows:

```powershell
go build -o PasswordAdmin.exe .
.\PasswordAdmin.exe
```

Abre en el navegador la dirección que imprime el servidor. Mantén el servidor Go escuchando en la interfaz local. No publiques el upstream sin protección: la aplicación web no administra usuarios ni sesiones individuales. Usa HTTPS y restringe el acceso en Nginx con autenticación y/o una VPN. Por ejemplo, dentro de un `server` HTTPS:

```nginx
location / {
    auth_basic "Password Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://127.0.0.1:45678;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

### Publicar bajo una subruta

Para publicarlo en una subruta como `https://server/pass-admin/`, conserva la barra final y configura Nginx para retirar el prefijo al reenviar:

```nginx
location = /pass-admin {
    return 301 /pass-admin/;
}

location /pass-admin/ {
    proxy_pass http://127.0.0.1:45678/;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

La barra final de `proxy_pass` hace que Nginx quite `/pass-admin/` al enviar la solicitud a Go, mientras el navegador conserva el prefijo para cargar `./app.css`, `./app.js` y `./api/...`. Si la ruta pública es otra, reemplaza `/pass-admin` en ambas reglas.

## Base de datos y seguridad

- La base SQLite persistente es `DB_PATH`. Protege el archivo y su carpeta con permisos del sistema operativo; en Linux la aplicación restringe la base a permisos `0600`.
- Los nombres de usuario y las contraseñas guardados se cifran con AES-256-GCM. Los nombres de cuentas, nombres de sitios, URLs y otros metadatos permanecen legibles en SQLite: el archivo completo **no** está cifrado.
- La clave de cifrado se deriva de la contraseña maestra con PBKDF2-HMAC-SHA-256 y una sal aleatoria. La clave se mantiene en memoria mientras la aplicación está desbloqueada y no se guarda en la base. Al reiniciar, hay que desbloquear de nuevo la bóveda.
- La contraseña maestra no se puede recuperar. Conserva una copia de seguridad de la base y de `.env` en un medio protegido. Una copia de la base no sustituye la contraseña maestra.
- No se debe ejecutar más de una instancia de Password Admin a la vez usando el mismo archivo SQLite.
- La interfaz web no añade autenticación de usuarios/sesiones. Si das acceso remoto con Nginx, configura autenticación fuerte y HTTPS. El servidor Go escucha en `127.0.0.1` de forma predeterminada.
- El CSV exportado contiene contraseñas en texto claro. Guárdalo con cuidado y elimínalo después de usarlo.

## Uso

1. En el primer inicio, crea una contraseña maestra de al menos 8 caracteres.
2. Crea una cuenta para agrupar contraseñas, por ejemplo «Personal» o «Trabajo».
3. En Chrome, exporta las contraseñas como CSV y pulsa **Importar CSV**.
4. Selecciona una cuenta para buscar y consultar sus entradas.
5. Pulsa **Exportar para Chrome** para guardar un CSV y luego utiliza la opción de importación de contraseñas de Chrome.

La opción **Todas las cuentas** muestra y busca las entradas de toda la bóveda. Si esa vista está seleccionada, la exportación genera un solo CSV con contraseñas de todas las cuentas. El CSV de Chrome no conserva la organización por cuentas.

Se reconocen los encabezados `url` y `password`; `name` y `username` son opcionales. El CSV puede contener comas, comillas o saltos de línea dentro de los valores.

## Compilar para Linux y Windows

Instala Go 1.22 o posterior y ejecuta los comandos desde la raíz del proyecto. No hace falta Wails, Node.js, SQLite externo ni compilar el frontend. Los ejemplos generan binarios para Intel/AMD de 64 bits (`amd64`); para Linux ARM de 64 bits, cambia `GOARCH=amd64` por `GOARCH=arm64`.

### Compilar en Windows

En PowerShell:

```powershell
New-Item -ItemType Directory -Force .\dist | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -o .\dist\PasswordAdmin-windows-amd64.exe .
if ($LASTEXITCODE -ne 0) { throw "Falló la compilación para Windows" }
Copy-Item .env.example .\dist\.env
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

Edita `dist\.env` si necesitas cambiar el puerto o la ruta de la base. `DB_PATH=./data/...` se guarda junto al ejecutable:

```powershell
.\dist\PasswordAdmin-windows-amd64.exe
```

### Compilar para Linux desde Windows

Go puede generar el binario Linux desde Windows sin instalar herramientas de compilación de Linux:

```powershell
New-Item -ItemType Directory -Force .\dist | Out-Null
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -o .\dist\password-admin-linux-amd64 .
if ($LASTEXITCODE -ne 0) { throw "Falló la compilación para Linux" }
Copy-Item .env.example .\dist\.env
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

El ejecutable Linux no tiene extensión `.exe`. Transfiere `password-admin-linux-amd64` y `.env` a una máquina Linux de la misma arquitectura:

```sh
mkdir -p "$HOME/password-admin"
cp password-admin-linux-amd64 .env "$HOME/password-admin/"
cd "$HOME/password-admin"
chmod 700 password-admin-linux-amd64
chmod 600 .env
./password-admin-linux-amd64
```

La base se creará en `data/password-admin.sqlite`, junto al ejecutable. El servicio imprime la URL local para abrir en el navegador.

También puedes compilar directamente en Linux:

```sh
mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/password-admin-linux-amd64 .
cp .env.example dist/.env
```

## Ejecutar como servicio Linux con systemd

Instala el binario en una carpeta persistente. El servicio usa un usuario propio y reinicia el proceso cuando falla:

```sh
sudo useradd --system --create-home --home-dir /opt/password-admin --shell /usr/sbin/nologin password-admin
sudo install -o password-admin -g password-admin -m 700 password-admin-linux-amd64 /opt/password-admin/password-admin
sudo install -o password-admin -g password-admin -m 600 .env /opt/password-admin/.env
sudo install -d -o password-admin -g password-admin -m 700 /opt/password-admin/data
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
ReadWritePaths=/opt/password-admin/data

[Install]
WantedBy=multi-user.target
```

Activa y comprueba el servicio:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now password-admin
sudo systemctl status password-admin
sudo journalctl -u password-admin -f
```

El `.env` debe incluir `PORT=45678` y `DB_PATH=/opt/password-admin/data/password-admin.sqlite`. La aplicación escucha en `127.0.0.1:45678`, disponible para Nginx en el mismo servidor. Después de editar `.env`, reinicia el servicio con `sudo systemctl restart password-admin`.

## Desarrollo y pruebas

```sh
go test ./...
go vet ./...
```

No se requiere SQLite externo: el proyecto usa `modernc.org/sqlite` sin CGO.

## Solución de problemas

- **La base se crea en otra carpeta:** comprueba `DB_PATH`. Las rutas relativas se resuelven desde el directorio del ejecutable; las rutas absolutas se usan tal como están configuradas.
- **No se puede crear o abrir SQLite:** comprueba los permisos de escritura del usuario del servicio en la carpeta indicada en `DB_PATH` y confirma que no haya otra instancia usando la misma base.
- **El puerto no cambia:** reinicia el ejecutable y verifica que `PORT` esté en `.env` o en el entorno del proceso. La variable del entorno tiene prioridad y el puerto no debe estar ocupado.
- **No se puede desbloquear la bóveda:** confirma la contraseña maestra y que se está abriendo el archivo correcto. La contraseña no se puede recuperar.
- **La página no carga bajo una subruta:** verifica que la dirección termine con `/` y que `proxy_pass` incluya la barra final como en el ejemplo.
