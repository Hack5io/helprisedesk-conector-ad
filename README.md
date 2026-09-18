# Conector de Active Directory de HelpriseDesk

Agente liviano que corre DENTRO de la red de tu empresa (al lado de tu
Active Directory) y abre una conexión saliente hacia HelpriseDesk -- así
podés usar la integración de Active Directory (búsqueda de usuarios,
reset de contraseñas desde un ticket) sin abrir ningún puerto de entrada
en tu firewall.

Se instala como un Servicio de Windows con un solo comando de PowerShell.
El token y los datos de tu AD salen de **HelpriseDesk → Integraciones →
Active Directory → Conector → Generar conector**.

## Instalar

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/Hack5io/helprisedesk-conector-ad/main/install.ps1" -OutFile install.ps1
.\install.ps1 `
    -Token "<el token que te mostró HelpriseDesk>" `
    -ApiUrl "https://helprisedesk.com.ar" `
    -AdHost "IP-O-HOSTNAME-DE-TU-AD" `
    -AdDominio "empresa.local" `
    -AdUsuario "cuenta-servicio" `
    -AdPassword "<contraseña de la cuenta de servicio>"
```

La contraseña de tu cuenta de servicio de AD queda **solo en este
servidor**, cifrada con DPAPI (en
`C:\ProgramData\HelpriseDeskConectorAD\config.json`) -- nunca se manda
ni se guarda en HelpriseDesk, ni queda en texto plano en el disco.

## Desinstalar

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/Hack5io/helprisedesk-conector-ad/main/uninstall.ps1" -OutFile uninstall.ps1
.\uninstall.ps1
```

Acordate de revocar también el conector desde HelpriseDesk (Integraciones
→ Active Directory → Conector → Revocar).

## Instalar en Linux (Docker)

Si preferís correrlo en un servidor Linux en vez de como Servicio de
Windows, hay una imagen publicada (amd64 y arm64):

```bash
docker run -d --name conector-ad --restart unless-stopped \
    -e CONECTOR_TOKEN="<el token que te mostró HelpriseDesk>" \
    -e CONECTOR_API_URL="https://helprisedesk.com.ar" \
    -e CONECTOR_AD_HOST="IP-O-HOSTNAME-DE-TU-AD" \
    -e CONECTOR_AD_DOMINIO="empresa.local" \
    -e CONECTOR_AD_USUARIO="cuenta-servicio" \
    -e CONECTOR_AD_PASSWORD="<contraseña de la cuenta de servicio>" \
    ghcr.io/hack5io/helprisedesk-conector-ad:latest
```

Acá la contraseña **nunca se escribe a un archivo** -- vive solo en la
variable de entorno del contenedor, mismo mecanismo que ya usás para
cualquier otro secreto en Docker (o pasala por un Docker secret /
Kubernetes Secret en vez de `-e`, si tu orquestador lo soporta).

Variables opcionales: `CONECTOR_AD_CUENTAS_EXCLUIDAS` (lista separada
por comas) y `CONECTOR_POLL_SEGUNDOS` (default 2).

## Requisitos

- Windows Server 2016+ (o Windows 10/11), con salida HTTPS (443) hacia
  internet -- nunca hace falta un puerto de entrada.
- Ejecutar `install.ps1` como Administrador.
- Línea de red desde este servidor hacia tu Active Directory (LDAP 389 y
  LDAPS 636).

## Desarrollo

```
go build ./cmd/conector-ad
go test ./...
```
