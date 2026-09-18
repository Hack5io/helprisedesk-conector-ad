//go:build !windows

package config

import "errors"

// DPAPI no existe fuera de Windows -- este stub existe para que
// "go build"/"go test" compilen en la máquina de un contribuidor que
// no usa Windows (y en el paso "Tests" del CI, que corre en
// ubuntu-latest antes de cruzar-compilar), y para el binario de Linux
// (ver Dockerfile) cuando alguien le pase un config.json con contraseña
// en texto plano por archivo en vez de por variables de entorno
// (config.CargarDeEnv(), el camino recomendado en Docker -- ahí no se
// escribe ningún archivo, no hace falta cifrar nada).
//
// A PROPÓSITO devuelve error en vez de simular que cifra/descifra:
// Cargar() ya sabe degradar a "sigue en texto plano, seguí andando" si
// proteger() falla (ver el comentario en config.go) -- fingir que cifró
// algo que en realidad quedó igual sería peor que decir la verdad.
func proteger(plano string) (string, error) {
	return "", errors.New("DPAPI no está disponible en este sistema operativo (solo Windows)")
}

func desproteger(cifrado string) (string, error) {
	return "", errors.New("DPAPI no está disponible en este sistema operativo (solo Windows) -- este config.json se cifró en una máquina Windows, no se puede leer acá")
}
