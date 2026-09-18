package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribirConfigTemporal(t *testing.T, contenido string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el config de prueba: %v", err)
	}
	return ruta
}

func TestCargarConfigCompletaFunciona(t *testing.T) {
	ruta := escribirConfigTemporal(t, `{
		"token": "abc123",
		"api_url": "https://helprisedesk.test",
		"ad_host": "10.10.15.10",
		"ad_dominio": "empresa.local",
		"ad_usuario": "cuenta-servicio",
		"ad_password": "clave",
		"ad_cuentas_excluidas": ["administrator"]
	}`)

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.Token != "abc123" || cfg.AdHost != "10.10.15.10" {
		t.Fatalf("config mal parseado: %+v", cfg)
	}
	if cfg.PollSegundos != 2 {
		t.Fatalf("esperaba el default de 2 segundos, dio %d", cfg.PollSegundos)
	}
}

// Una instalación vieja (de antes de que ad_password_cifrada existiera)
// tiene la contraseña en texto plano y sin ese campo -- Cargar() tiene
// que seguir funcionando con ella (nunca romper un conector ya andando
// por un intento de migración) aunque el intento de cifrar falle. En
// este SO (el stub, ver dpapi_stub.go) proteger() siempre falla a
// propósito -- DPAPI no existe acá -- así que el archivo queda IGUAL,
// sin migrar; el camino "se migra de verdad" solo se puede probar en
// Windows (ver dpapi_windows.go), acá se prueba el "no revienta nada
// si no se puede".
func TestCargarConfigViejaSinCifrarSiguienFuncionaAunqueNoPuedaMigrar(t *testing.T) {
	contenidoOriginal := `{
		"token": "abc123",
		"api_url": "https://helprisedesk.test",
		"ad_host": "10.10.15.10",
		"ad_dominio": "empresa.local",
		"ad_usuario": "cuenta-servicio",
		"ad_password": "clave-en-texto-plano"
	}`
	ruta := escribirConfigTemporal(t, contenidoOriginal)

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.AdPassword != "clave-en-texto-plano" {
		t.Fatalf("la contraseña en memoria tiene que seguir siendo la real, dio %q", cfg.AdPassword)
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo releer el archivo: %v", err)
	}
	if strings.Contains(string(datos), `"ad_password_cifrada": true`) {
		t.Fatalf("no debería haberse marcado como cifrado -- proteger() falla en este SO, quedó: %s", datos)
	}
}

// Un config.json cifrado con DPAPI en Windows, copiado/usado en
// cualquier otro SO, tiene que fallar EXPLÍCITAMENTE -- nunca fingir
// que lo descifró y devolver basura o la cadena cifrada como si fuera
// la contraseña real.
func TestCargarConfigCifradaEnOtroSoFallaExplicito(t *testing.T) {
	ruta := escribirConfigTemporal(t, `{
		"token": "abc123",
		"api_url": "https://helprisedesk.test",
		"ad_host": "10.10.15.10",
		"ad_dominio": "empresa.local",
		"ad_usuario": "cuenta-servicio",
		"ad_password": "blob-cifrado-de-windows-illegible-aca",
		"ad_password_cifrada": true
	}`)

	if _, err := Cargar(ruta); err == nil {
		t.Fatal("esperaba un error -- este SO no puede descifrar DPAPI")
	}
}

func TestCargarDeEnvCompletaFunciona(t *testing.T) {
	t.Setenv("CONECTOR_TOKEN", "abc123")
	t.Setenv("CONECTOR_API_URL", "https://helprisedesk.test")
	t.Setenv("CONECTOR_AD_HOST", "10.10.15.10")
	t.Setenv("CONECTOR_AD_DOMINIO", "empresa.local")
	t.Setenv("CONECTOR_AD_USUARIO", "cuenta-servicio")
	t.Setenv("CONECTOR_AD_PASSWORD", "clave")
	t.Setenv("CONECTOR_AD_CUENTAS_EXCLUIDAS", "administrator, invitado")
	t.Setenv("CONECTOR_POLL_SEGUNDOS", "5")

	cfg, err := CargarDeEnv()
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.Token != "abc123" || cfg.AdPassword != "clave" {
		t.Fatalf("config mal armada desde el entorno: %+v", cfg)
	}
	if len(cfg.AdCuentasExcluidas) != 2 || cfg.AdCuentasExcluidas[0] != "administrator" || cfg.AdCuentasExcluidas[1] != "invitado" {
		t.Fatalf("lista de cuentas excluidas mal parseada: %v", cfg.AdCuentasExcluidas)
	}
	if cfg.PollSegundos != 5 {
		t.Fatalf("esperaba poll_segundos=5, dio %d", cfg.PollSegundos)
	}
}

func TestCargarDeEnvSinPollSegundosUsaElDefault(t *testing.T) {
	t.Setenv("CONECTOR_TOKEN", "abc123")
	t.Setenv("CONECTOR_API_URL", "https://helprisedesk.test")
	t.Setenv("CONECTOR_AD_HOST", "10.10.15.10")
	t.Setenv("CONECTOR_AD_DOMINIO", "empresa.local")
	t.Setenv("CONECTOR_AD_USUARIO", "cuenta-servicio")
	t.Setenv("CONECTOR_AD_PASSWORD", "clave")

	cfg, err := CargarDeEnv()
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.PollSegundos != 2 {
		t.Fatalf("esperaba el default de 2 segundos, dio %d", cfg.PollSegundos)
	}
}

func TestCargarDeEnvIncompletaFalla(t *testing.T) {
	t.Setenv("CONECTOR_TOKEN", "abc123")
	// El resto de las variables quedan sin setear.

	if _, err := CargarDeEnv(); err == nil {
		t.Fatal("esperaba un error por variables faltantes")
	}
}

func TestCargarConfigIncompletaFalla(t *testing.T) {
	ruta := escribirConfigTemporal(t, `{"token": "abc123"}`)

	if _, err := Cargar(ruta); err == nil {
		t.Fatal("esperaba un error por campos faltantes")
	}
}

func TestCargarConfigInexistenteFalla(t *testing.T) {
	if _, err := Cargar(filepath.Join(t.TempDir(), "no-existe.json")); err == nil {
		t.Fatal("esperaba un error de archivo inexistente")
	}
}
