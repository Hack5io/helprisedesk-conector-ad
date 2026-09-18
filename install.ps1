#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Instala el Conector de Active Directory de HelpriseDesk como Servicio
    de Windows -- sin Docker, sin abrir ningun puerto de entrada.

.DESCRIPTION
    Descarga el ultimo release publicado en GitHub, escribe la
    configuracion, y registra + arranca el servicio de Windows
    "HelpriseDeskConectorAD". Los parametros salen de la pantalla
    Integraciones > Active Directory > Conector, dentro de HelpriseDesk.

.EXAMPLE
    .\install.ps1 -Token "..." -ApiUrl "https://helprisedesk.com.ar" `
        -AdHost "10.10.15.10" -AdDominio "empresa.local" `
        -AdUsuario "cuenta-servicio" -AdPassword "..."
#>
param(
    [Parameter(Mandatory = $true)][string]$Token,
    [Parameter(Mandatory = $true)][string]$ApiUrl,
    [Parameter(Mandatory = $true)][string]$AdHost,
    [Parameter(Mandatory = $true)][string]$AdDominio,
    [Parameter(Mandatory = $true)][string]$AdUsuario,
    [Parameter(Mandatory = $true)][string]$AdPassword,
    [string[]]$AdCuentasExcluidas = @(),
    [string]$InstallDir = "$env:ProgramFiles\HelpriseDeskConectorAD",
    [string]$ConfigDir = "$env:ProgramData\HelpriseDeskConectorAD"
)

$ErrorActionPreference = "Stop"

Write-Host "Instalando el Conector de Active Directory de HelpriseDesk..." -ForegroundColor Cyan

# 1) Descargar el ultimo release publicado (repo publico -- sin token de GitHub).
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/Hack5io/helprisedesk-conector-ad/releases/latest"
$asset = $release.assets | Where-Object { $_.name -like "*windows-amd64.exe" } | Select-Object -First 1
if (-not $asset) {
    throw "No se encontro un release para Windows. Revisa https://github.com/Hack5io/helprisedesk-conector-ad/releases"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

$rutaExe = Join-Path $InstallDir "conector-ad.exe"
Write-Host "Descargando $($asset.name)..."
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $rutaExe

# 2) Cifrar la contraseña del AD con DPAPI (scope de MÁQUINA, no de
#    usuario -- el servicio puede correr bajo una cuenta distinta de la
#    que instala) antes de escribirla a disco. Mismo mecanismo que usa
#    el propio binario para leerla (ver internal/config/dpapi_windows.go)
#    -- .NET y el syscall de Go llaman a la misma API de Windows por
#    debajo, un blob cifrado acá se descifra bien del otro lado. Nunca
#    se guarda en texto plano en ningún momento, ni siquiera un instante.
Add-Type -AssemblyName System.Security
$passwordBytes = [System.Text.Encoding]::UTF8.GetBytes($AdPassword)
$passwordCifrada = [System.Security.Cryptography.ProtectedData]::Protect(
    $passwordBytes, $null, [System.Security.Cryptography.DataProtectionScope]::LocalMachine
)

# 3) Escribir config.json -- nunca se guarda en el propio HelpriseDesk,
#    vive solo en este servidor, dentro de la red del tenant.
$config = @{
    token                = $Token
    api_url              = $ApiUrl
    ad_host              = $AdHost
    ad_dominio           = $AdDominio
    ad_usuario           = $AdUsuario
    ad_password          = [Convert]::ToBase64String($passwordCifrada)
    ad_password_cifrada  = $true
    ad_cuentas_excluidas = $AdCuentasExcluidas
    poll_segundos        = 2
} | ConvertTo-Json

$rutaConfig = Join-Path $ConfigDir "config.json"
Set-Content -Path $rutaConfig -Value $config -Encoding UTF8
Write-Host "Configuracion guardada en $rutaConfig"

# 4) Registrar y arrancar el servicio (ver internal/service en el binario, via kardianos/service).
& $rutaExe -config $rutaConfig install
Start-Service -Name "HelpriseDeskConectorAD"

Write-Host ""
Write-Host "Listo. El servicio 'HelpriseDeskConectorAD' esta instalado y corriendo." -ForegroundColor Green
Write-Host "Volve a la pantalla de Active Directory en HelpriseDesk -- deberia mostrar 'Conectado' en unos segundos."
Write-Host ""
Write-Host "Para desinstalarlo mas adelante: descarga y corre uninstall.ps1 desde el mismo repositorio."
