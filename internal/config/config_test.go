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
// por una migración) Y dejar el archivo reescrito con la contraseña
// cifrada para la próxima vez. La criptografía REAL de DPAPI no se
// puede probar acá (solo existe en Windows, ver dpapi_windows.go) --
// esto prueba la lógica de migración en sí con el stub
// (proteger/desproteger no-op en cualquier SO que no sea Windows).
func TestCargarConfigViejaSinCifrarSeMigra(t *testing.T) {
	ruta := escribirConfigTemporal(t, `{
		"token": "abc123",
		"api_url": "https://helprisedesk.test",
		"ad_host": "10.10.15.10",
		"ad_dominio": "empresa.local",
		"ad_usuario": "cuenta-servicio",
		"ad_password": "clave-en-texto-plano"
	}`)

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.AdPassword != "clave-en-texto-plano" {
		t.Fatalf("la contraseña en memoria tiene que seguir siendo la real, dio %q", cfg.AdPassword)
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo releer el archivo migrado: %v", err)
	}
	if !strings.Contains(string(datos), `"ad_password_cifrada": true`) {
		t.Fatalf("el archivo debería haber quedado marcado como cifrado tras la migración, quedó: %s", datos)
	}
}

func TestCargarConfigYaCifradaSeDescifra(t *testing.T) {
	ruta := escribirConfigTemporal(t, `{
		"token": "abc123",
		"api_url": "https://helprisedesk.test",
		"ad_host": "10.10.15.10",
		"ad_dominio": "empresa.local",
		"ad_usuario": "cuenta-servicio",
		"ad_password": "ya-viene-cifrada-en-el-stub-es-igual",
		"ad_password_cifrada": true
	}`)

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if cfg.AdPassword != "ya-viene-cifrada-en-el-stub-es-igual" {
		t.Fatalf("esperaba la contraseña descifrada (passthrough en el stub), dio %q", cfg.AdPassword)
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
