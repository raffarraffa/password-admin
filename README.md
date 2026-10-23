# Password Admin

Este proyecto tiene dos variantes que no se deben confundir. La compilación de escritorio Wails abre una ventana propia en Windows o Linux y no inicia un servidor web. La variante web Go sirve HTTP en `127.0.0.1:45678` y puede ejecutarse de forma persistente con systemd; consulta la sección de servicio web más abajo. Los binarios de ambas variantes no son intercambiables.

## Uso portable

1. Copia el ejecutable de tu sistema operativo a una carpeta propia del escritorio o a un pendrive.
2. Opcionalmente, copia `.env.example` en esa misma carpeta con el nombre `.env`.
3. Ejecuta `PasswordAdmin-windows-amd64.exe` en Windows o `password-admin-linux-amd64` en Linux.
4. En el primer inicio, crea una contraseña maestra. En los siguientes, desbloquea la bóveda con esa contraseña.

Por defecto, la base se crea como `data/password-admin.sqlite` junto al ejecutable. La aplicación crea `data` si no existe. En Linux, el archivo SQLite se limita a permisos `0600`. La carpeta debe permitir escritura.

El `.env` es opcional. Las variables del entorno tienen prioridad; si existe, la app lee `.env` desde la carpeta del ejecutable:

```env
DB_PATH=./data/password-admin.sqlite
```

`DB_PATH` puede ser absoluto o relativo al directorio del ejecutable. Para trasladar la bóveda, cierra la aplicación y copia junta la carpeta `data` (o la ruta configurada en `DB_PATH`) y el ejecutable. No ejecutes simultáneamente dos copias que apunten al mismo archivo SQLite. Haz copias de seguridad con la aplicación cerrada.

La aplicación usa el runtime de escritorio del sistema: WebView2 Evergreen en Windows y GTK 3/WebKit2GTK 4.0 en Linux. Estos componentes deben estar disponibles en el equipo de destino; por eso el ejecutable y la base son portables, pero no incluyen los runtimes del sistema.

## Datos y seguridad

- Los usuarios y las contraseñas se cifran con AES-256-GCM. Los nombres de cuentas, sitios, URLs y otros metadatos quedan legibles en SQLite; la base completa no está cifrada.
- La clave se deriva de la contraseña maestra mediante PBKDF2-HMAC-SHA-256 con una sal aleatoria. La clave solo se mantiene en memoria mientras la app está abierta y desbloqueada.
- La contraseña maestra no se puede recuperar. Conserva una copia protegida de la base y recuerda la contraseña.
- La aplicación no sincroniza datos con GitHub ni necesita conexión a Internet.
- Los CSV exportados contienen contraseñas en texto claro. Guárdalos con cuidado y elimínalos después de usarlos.
- Esta versión crea una bóveda SQLite local nueva; no importa automáticamente los archivos de bóveda ni repositorios de versiones anteriores.

## Uso de la aplicación

1. Crea una cuenta para agrupar las contraseñas, por ejemplo «Personal» o «Trabajo».
2. Pulsa **Importar CSV** para cargar un archivo exportado desde Chrome y asignarlo a una cuenta.
3. Busca y consulta las entradas desde la ventana de la aplicación.
4. Pulsa **Exportar para Chrome** para elegir dónde guardar un CSV y luego impórtalo en Chrome.

La opción **Todas las cuentas** muestra y busca las entradas de toda la bóveda. El CSV de Chrome no conserva la organización por cuentas. Se reconocen los encabezados `url` y `password`; `name` y `username` son opcionales. Se admiten comas, comillas y saltos de línea dentro de los valores.

## Compilar

Se requiere Go 1.22 o posterior, Wails CLI 2.10.2 y Node.js no es necesario para este frontend. SQLite usa `modernc.org/sqlite` sin CGO. Wails requiere CGO y las bibliotecas de desarrollo de la plataforma.

### Windows amd64

En Windows, instala WebView2 Evergreen Runtime y Wails CLI, luego compila desde la raíz del proyecto:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
wails build -skipbindings -platform windows/amd64 -o PasswordAdmin-windows-amd64
```

El binario se genera en `build\bin\PasswordAdmin-windows-amd64.exe`. Copia el ejecutable a la carpeta portable y ejecútalo normalmente.

### Linux amd64

Compila en Linux (no es posible generar el ejecutable de escritorio Linux con un `GOOS=linux` cruzado desde Windows: Wails necesita CGO y GTK/WebKit del sistema). En Debian/Ubuntu instala las dependencias de compilación:

```sh
sudo apt-get install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
wails build -skipbindings -platform linux/amd64 -o password-admin-linux-amd64
```

El binario se genera en `build/bin/password-admin-linux-amd64`. En el pendrive, permite su ejecución y escritura en la carpeta de datos:

```sh
chmod 700 password-admin-linux-amd64
./password-admin-linux-amd64
```

El Linux de destino necesita GTK 3 y WebKit2GTK 4.0 compatibles. Para distribuciones que solo proveen WebKit2GTK 4.1, compila Wails con la etiqueta `webkit2_41`.

### Ejecutar la compilación web Go como servicio systemd (Linux)

Estas instrucciones son para el **binario de la variante web Go**, que sirve HTTP en `127.0.0.1:45678`; no para el ejecutable Wails de escritorio. En este árbol, la compilación web previa está en `dist/password-admin-linux-amd64`, mientras que `build/bin/password-admin-linux-amd64` es la app de escritorio. No intercambies esos binarios. El código fuente actual de este checkout es la variante Wails; para volver a compilar el servidor web necesitas la versión del código que incluye su `main.go` HTTP.

Instala el binario web en `/opt` y reserva `/var/lib` para la base persistente:

```sh
sudo useradd --system --no-create-home --shell /usr/sbin/nologin password-admin
sudo install -d -o root -g root -m 755 /opt/password-admin
sudo install -o root -g root -m 755 dist/password-admin-linux-amd64 /opt/password-admin/password-admin
sudo install -d -o password-admin -g password-admin -m 700 /var/lib/password-admin
```

Crea `/etc/systemd/system/password-admin.service`:

```ini
[Unit]
Description=Password Admin Go web server
After=network.target

[Service]
Type=simple
User=password-admin
Group=password-admin
WorkingDirectory=/opt/password-admin
ExecStart=/opt/password-admin/password-admin
Environment=PORT=45678
Environment=DB_PATH=/var/lib/password-admin/password-admin.sqlite
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/password-admin

[Install]
WantedBy=multi-user.target
```

Recarga systemd, habilita el inicio automático al arrancar Linux e inicia el servicio:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now password-admin
sudo systemctl status password-admin
curl http://127.0.0.1:45678/
```

La bóveda queda guardada en `/var/lib/password-admin/password-admin.sqlite`, incluso al reiniciar o actualizar el ejecutable. Para ver registros o administrar el proceso:

```sh
sudo journalctl -u password-admin -f
sudo systemctl restart password-admin
sudo systemctl stop password-admin
```

El servidor web escucha solo en `127.0.0.1`, así que no queda publicado directamente en la red. Para acceder desde otro equipo, pon Nginx delante con HTTPS y autenticación; no expongas este servidor sin protección, porque la aplicación no gestiona usuarios ni sesiones individuales.

### Generar los dos ejecutables

Windows puede compilar su ejecutable localmente. Linux debe compilarse en un host Linux con las bibliotecas indicadas (también puede usarse una máquina virtual Linux). Una vez obtenidos, coloca ambos archivos en el pendrive o en sus carpetas correspondientes:

```text
PasswordAdmin-windows-amd64.exe
password-admin-linux-amd64
data/
  password-admin.sqlite
```

La carpeta `data/` se crea en el primer inicio; no hace falta crearla manualmente. No uses el binario de un sistema operativo en el otro.

## Desarrollo

```sh
go test ./...
go vet ./...
```
