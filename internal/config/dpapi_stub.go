//go:build !windows

package config

// El conector solo se compila/corre de verdad para Windows (ver
// .github/workflows/release.yml, GOOS=windows) -- DPAPI no existe en
// ningún otro sistema operativo. Este stub existe solo para que
// "go build"/"go test" compilen en la máquina de un contribuidor que
// no usa Windows (y en el paso "Tests" del CI, que corre en
// ubuntu-latest antes de cruzar-compilar) -- nunca se ejecuta de
// verdad en producción. No cifra nada: en un sistema no-Windows no hay
// ninguna implementación real disponible para hacerlo.
func proteger(plano string) (string, error) {
	return plano, nil
}

func desproteger(cifrado string) (string, error) {
	return cifrado, nil
}
