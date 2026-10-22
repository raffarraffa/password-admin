# Password Admin

Aplicación de escritorio hecha con Go y Wails. La lógica y la base SQLite funcionan localmente; la interfaz de la ventana está hecha con HTML, CSS y JavaScript, sin un servidor web externo.

## Seguridad

- No envía contraseñas a servicios en línea. La interfaz tampoco descarga fuentes, scripts ni otros recursos remotos.
- Los nombres de usuario y las contraseñas se cifran localmente con AES-256-GCM. La clave se deriva de la contraseña maestra con PBKDF2-HMAC-SHA-256 y una sal aleatoria.
- La contraseña maestra no se puede recuperar. Guárdala en un lugar seguro.
- Los nombres de cuenta, nombres de sitio y URL no se cifran.
- El CSV de Chrome exportado contiene contraseñas en texto claro. Guárdalo con cuidado y elimínalo después de importarlo en Chrome.
- Respalda `vault.sqlite` con la aplicación cerrada y conserva el respaldo de forma segura.

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

La primera vez, crea una contraseña maestra de al menos 12 caracteres. En modo desarrollo (`wails dev`), la bóveda se guarda como `vault.sqlite` en la raíz del proyecto. En la aplicación compilada, se guarda en la misma carpeta que el ejecutable.

Si ya habías usado `wails dev` antes de este cambio, la aplicación mueve automáticamente `build/bin/vault.sqlite` a la raíz del proyecto cuando inicia, siempre que todavía no exista una bóveda allí.

Si PowerShell no encuentra `wails` después de instalar el CLI, agrégalo al `PATH` de esa terminal y vuelve a ejecutar:

```powershell
$env:Path += ";$(go env GOPATH)\bin"
wails dev
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

## Uso portátil desde un pendrive

En Windows, copia `Password Admin.exe` al pendrive y ejecútalo desde allí. La primera ejecución crea `vault.sqlite` junto al `.exe`. Para mover la bóveda a otro equipo, cierra la aplicación y lleva juntos el ejecutable y `vault.sqlite`; conserva también una copia de seguridad de ambos archivos en un lugar seguro.

La carpeta debe permitir escritura. Si el pendrive está protegido contra escritura o el ejecutable se encuentra en una carpeta sin permisos de escritura, la aplicación no podrá abrir o crear la bóveda. La bóveda cifra los usuarios y las contraseñas, pero el archivo SQLite sigue conteniendo nombres de cuentas y sitios. Usa la contraseña maestra y expulsa el pendrive de forma segura.

## Icono

La imagen fuente de la aplicación es `build/appicon.png`. Wails la usa para generar los iconos de la compilación para Windows y macOS; en Linux, la aplicación la usa como icono de ventana. Si ya existe `build/windows/icon.ico`, reemplázalo también con el ICO correspondiente para que Windows no siga usando el icono generado anteriormente. Después ejecuta `wails build` otra vez.
