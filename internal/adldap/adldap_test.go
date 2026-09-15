package adldap

import (
	"bytes"
	"testing"
	"unicode/utf16"
)

// Active Directory exige unicodePwd como la contraseña ENTRE COMILLAS,
// codificada en UTF-16LE -- https://learn.microsoft.com/troubleshoot/
// windows-server/identity/set-user-password-with-ldifde. Este test
// existe porque es un detalle fácil de romper en silencio (una password
// mal codificada hace que AD rechace el cambio sin un mensaje claro).
func TestCodificarUnicodePwdEnvuelveEntreComillasYCodificaUTF16LE(t *testing.T) {
	resultado := codificarUnicodePwd("Xy7!kLp9")

	esperado := utf16.Encode([]rune(`"Xy7!kLp9"`))
	var esperadoBytes []byte
	for _, u := range esperado {
		esperadoBytes = append(esperadoBytes, byte(u), byte(u>>8))
	}

	if !bytes.Equal(resultado, esperadoBytes) {
		t.Fatalf("codificación incorrecta:\n  got  %v\n  want %v", resultado, esperadoBytes)
	}
}

func TestGenerarPasswordTemporalCumpleLaLongitudEsperada(t *testing.T) {
	password := generarPasswordTemporal()

	if len(password) != 12 {
		t.Fatalf("esperaba 12 caracteres, dio %d (%q)", len(password), password)
	}
}

func TestGenerarPasswordTemporalNoRepiteSiempreLoMismo(t *testing.T) {
	a := generarPasswordTemporal()
	b := generarPasswordTemporal()

	if a == b {
		t.Fatalf("dos llamadas seguidas dieron la misma password -- ¿falta aleatoriedad?: %q", a)
	}
}
