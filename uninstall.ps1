#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Desinstala el Conector de Active Directory de HelpriseDesk.
#>
param(
    [string]$InstallDir = "$env:ProgramFiles\HelpriseDeskConectorAD",
    [string]$ConfigDir = "$env:ProgramData\HelpriseDeskConectorAD"
)

$ErrorActionPreference = "Stop"
$rutaExe = Join-Path $InstallDir "conector-ad.exe"

if (Get-Service -Name "HelpriseDeskConectorAD" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "HelpriseDeskConectorAD" -ErrorAction SilentlyContinue
    & $rutaExe uninstall
}

Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $ConfigDir -ErrorAction SilentlyContinue

Write-Host "Conector desinstalado. Acorda revocarlo tambien desde HelpriseDesk (Integraciones > Active Directory > Revocar conector)." -ForegroundColor Yellow
