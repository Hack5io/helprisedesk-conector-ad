// Package config carga la configuración del conector desde un archivo
// JSON en vez de variables de entorno -- a diferencia del comando
// artisan original (que leía env()), un Servicio de Windows no hereda
// fácilmente las variables de entorno de quien lo instaló, así que
// install.ps1 escribe este archivo una sola vez al momento de instalar.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Token      string `json:"token"`
	ApiURL     string `json:"api_url"`
	AdHost     string `json:"ad_host"`
	AdDominio  string `json:"ad_dominio"`
	AdUsuario  string `json:"ad_usuario"`
	AdPassword string `json:"ad_password"`
	// true si AdPassword (en el archivo, no en memoria una vez cargado)
	// está cifrada con DPAPI en vez de en texto plano -- ver
	// dpapi_windows.go. Ausente/false en config.json escritos por una
	// versión vieja de install.ps1 (de antes de que esto existiera):
	// Cargar() migra esos automáticamente la primera vez que el
	// servicio arranca con el binario nuevo, sin que el cliente tenga
	// que reinstalar nada a mano.
	AdPasswordCifrada  bool     `json:"ad_password_cifrada"`
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

	if cfg.AdPasswordCifrada {
		plano, err := desproteger(cfg.AdPassword)
		if err != nil {
			return nil, fmt.Errorf(
				"no se pudo descifrar ad_password (¿config.json copiado de otra máquina? DPAPI solo descifra en la máquina que cifró): %w", err,
			)
		}
		cfg.AdPassword = plano
	} else {
		// Contraseña en texto plano, de una instalación vieja -- se
		// cifra y se reescribe el archivo ahora, así queda protegida en
		// disco desde el próximo arranque en adelante. Si la migración
		// falla por lo que sea (permisos, disco de solo lectura), el
		// conector sigue funcionando igual con la contraseña en texto
		// plano -- nunca por esto deja de conectarse al AD.
		if err := cfg.migrarACifrada(ruta); err != nil {
			log.Printf("aviso: no se pudo migrar ad_password a formato cifrado, sigue en texto plano en disco por ahora: %v", err)
		}
	}

	return &cfg, nil
}

// CargarDeEnv arma la config a partir de variables de entorno en vez de
// un archivo -- pensado para Docker (ver Dockerfile), donde el
// mecanismo estándar para pasar secretos es el entorno del proceso
// (env var / Docker secret / Kubernetes secret), no un archivo en
// disco. A propósito NUNCA escribe nada a disco ni pasa por
// proteger()/desproteger() -- no hay archivo que cifrar, la contraseña
// vive solo en la memoria de este proceso mientras corre, como
// cualquier otro secreto inyectado por variable de entorno en un
// contenedor.
func CargarDeEnv() (*Config, error) {
	cfg := &Config{
		Token:      os.Getenv("CONECTOR_TOKEN"),
		ApiURL:     os.Getenv("CONECTOR_API_URL"),
		AdHost:     os.Getenv("CONECTOR_AD_HOST"),
		AdDominio:  os.Getenv("CONECTOR_AD_DOMINIO"),
		AdUsuario:  os.Getenv("CONECTOR_AD_USUARIO"),
		AdPassword: os.Getenv("CONECTOR_AD_PASSWORD"),
	}

	if excluidas := strings.TrimSpace(os.Getenv("CONECTOR_AD_CUENTAS_EXCLUIDAS")); excluidas != "" {
		for _, cuenta := range strings.Split(excluidas, ",") {
			if cuenta = strings.TrimSpace(cuenta); cuenta != "" {
				cfg.AdCuentasExcluidas = append(cfg.AdCuentasExcluidas, cuenta)
			}
		}
	}

	cfg.PollSegundos = 2
	if poll := os.Getenv("CONECTOR_POLL_SEGUNDOS"); poll != "" {
		n, err := strconv.Atoi(poll)
		if err != nil {
			return nil, fmt.Errorf("CONECTOR_POLL_SEGUNDOS tiene que ser un número entero, llegó %q: %w", poll, err)
		}
		if n > 0 {
			cfg.PollSegundos = n
		}
	}

	if err := cfg.Validar(); err != nil {
		return nil, err
	}

	return cfg, nil
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
		return fmt.Errorf("faltan campos de configuración: %v", faltantes)
	}

	return nil
}

// migrarACifrada cifra la contraseña en texto plano ya cargada en "c" y
// reescribe el archivo de config con ad_password_cifrada=true. "c" en
// memoria NO se modifica (Cargar ya usa el valor plano para el resto de
// la corrida) -- esto solo cambia lo que queda guardado en disco.
func (c *Config) migrarACifrada(ruta string) error {
	cifrada, err := proteger(c.AdPassword)
	if err != nil {
		return err
	}

	copia := *c
	copia.AdPassword = cifrada
	copia.AdPasswordCifrada = true

	// 0600: solo el dueño del archivo (la cuenta bajo la que corre el
	// servicio) puede leerlo -- ni siquiera este permiso es la defensa
	// real (esa es DPAPI), pero reduce quién puede intentarlo.
	datos, err := json.MarshalIndent(copia, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ruta, datos, 0o600)
}
