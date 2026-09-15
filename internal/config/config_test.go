package config

import (
	"os"
	"path/filepath"
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
