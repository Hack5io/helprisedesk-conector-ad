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
    -AdHost "10.10.15.10" `
    -AdDominio "empresa.local" `
    -AdUsuario "cuenta-servicio" `
    -AdPassword "<contraseña de la cuenta de servicio>"
```

La contraseña de tu cuenta de servicio de AD queda **solo en este
servidor** (en `C:\ProgramData\HelpriseDeskConectorAD\config.json`) --
nunca se manda ni se guarda en HelpriseDesk.

## Desinstalar

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/Hack5io/helprisedesk-conector-ad/main/uninstall.ps1" -OutFile uninstall.ps1
.\uninstall.ps1
```

Acordate de revocar también el conector desde HelpriseDesk (Integraciones
→ Active Directory → Conector → Revocar).

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
