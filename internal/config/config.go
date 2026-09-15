// Package config carga la configuración del conector desde un archivo
// JSON en vez de variables de entorno -- a diferencia del comando
// artisan original (que leía env()), un Servicio de Windows no hereda
// fácilmente las variables de entorno de quien lo instaló, así que
// install.ps1 escribe este archivo una sola vez al momento de instalar.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Token              string   `json:"token"`
	ApiURL             string   `json:"api_url"`
	AdHost             string   `json:"ad_host"`
	AdDominio          string   `json:"ad_dominio"`
	AdUsuario          string   `json:"ad_usuario"`
	AdPassword         string   `json:"ad_password"`
	AdCuentasExcluidas []string `json:"ad_cuentas_excluidas"`
	// Cada cuánto hace polling -- ver también el timeout bloqueante del
	// lado del servidor (config('services.ad_connector') en la app
	// Laravel), este valor tiene que ser bastante menor a ese timeout.
	PollSegundos int `json:"poll_segundos"`
}

const rutaPorDefecto = `C:\ProgramData\HelpriseDeskConectorAD\config.json`

func RutaPorDefecto() string {
	return rutaPorDefecto
}

func Cargar(ruta string) (*Config, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", ruta, err)
	}

	var cfg Config
	if err := json.Unmarshal(datos, &cfg); err != nil {
		return nil, fmt.Errorf("config.json inválido: %w", err)
	}

	if err := cfg.Validar(); err != nil {
		return nil, err
	}

	if cfg.PollSegundos <= 0 {
		cfg.PollSegundos = 2
	}

	return &cfg, nil
}

func (c *Config) Validar() error {
	faltantes := []string{}

	if c.Token == "" {
		faltantes = append(faltantes, "token")
	}
	if c.ApiURL == "" {
		faltantes = append(faltantes, "api_url")
	}
	if c.AdHost == "" {
		faltantes = append(faltantes, "ad_host")
	}
	if c.AdDominio == "" {
		faltantes = append(faltantes, "ad_dominio")
	}
	if c.AdUsuario == "" {
		faltantes = append(faltantes, "ad_usuario")
	}
	// Contraseña vacía: LDAP la acepta como "bind no autenticado" sin
	// validar nada (mismo hallazgo de LdapEjecutor.php) -- se rechaza
	// ACÁ, nunca se delega esa validación al servidor de AD.
	if c.AdPassword == "" {
		faltantes = append(faltantes, "ad_password")
	}

	if len(faltantes) > 0 {
		return fmt.Errorf("faltan campos en config.json: %v", faltantes)
	}

	return nil
}
