//go:build windows

package config

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI (Data Protection API de Windows) -- la contraseña se cifra con
// una clave que el propio sistema operativo administra, derivada de la
// MÁQUINA (CRYPTPROTECT_LOCAL_MACHINE), no del usuario: el servicio
// puede correr bajo una cuenta de servicio distinta de la que instaló
// el conector, y de todos modos tiene que poder descifrar en cada
// arranque. La contra de usar el scope de máquina en vez de usuario: si
// alguien más ya tiene ejecución de código en esa misma máquina (con
// cualquier usuario), técnicamente puede pedirle a Windows que
// descifre -- pero a esa altura ya tiene acceso total a la máquina
// igual, copiar config.json ya no es el problema real. Lo que SÍ evita
// DPAPI: que copiar config.json a OTRA máquina (un USB, una config que
// termina en un repositorio, un backup mal restringido) sirva de algo
// -- ahí el descifrado falla siempre, la clave nunca sale del equipo
// original.
//
// golang.org/x/sys/windows ya es una dependencia indirecta (la trae
// kardianos/service) -- no suma ninguna dependencia nueva al binario.

func proteger(plano string) (string, error) {
	entrada := windows.DataBlob{
		Size: uint32(len(plano)),
		Data: nil,
	}
	if len(plano) > 0 {
		entrada.Data = &([]byte(plano))[0]
	}

	var salida windows.DataBlob

	if err := windows.CryptProtectData(&entrada, nil, nil, 0, nil, windows.CRYPTPROTECT_LOCAL_MACHINE, &salida); err != nil {
		return "", fmt.Errorf("CryptProtectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(salida.Data)))

	cifrado := unsafe.Slice(salida.Data, salida.Size)

	return base64.StdEncoding.EncodeToString(cifrado), nil
}

func desproteger(cifradoBase64 string) (string, error) {
	cifrado, err := base64.StdEncoding.DecodeString(cifradoBase64)
	if err != nil {
		return "", fmt.Errorf("ad_password no es base64 válido: %w", err)
	}

	entrada := windows.DataBlob{
		Size: uint32(len(cifrado)),
		Data: nil,
	}
	if len(cifrado) > 0 {
		entrada.Data = &cifrado[0]
	}

	var salida windows.DataBlob

	if err := windows.CryptUnprotectData(&entrada, nil, nil, 0, nil, windows.CRYPTPROTECT_LOCAL_MACHINE, &salida); err != nil {
		return "", fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(salida.Data)))

	plano := unsafe.Slice(salida.Data, salida.Size)

	return string(plano), nil
}
