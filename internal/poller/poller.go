// Package poller es el puerto a Go de App\Console\Commands\EjecutarConectorAD.php
// -- el loop que hace polling de la API de HelpriseDesk y ejecuta cada
// comando contra el AD local. El protocolo (JSON + Bearer token) no
// cambió nada al pasar de PHP a Go: el servidor no le importa qué
// implementa al cliente.
package poller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Hack5io/helprisedesk-conector-ad/internal/adldap"
)

const (
	TipoBuscarUsuario     = "buscar_usuario"
	TipoResetearPassword  = "resetear_password"
)

type Comando struct {
	ID      int             `json:"id"`
	Tipo    string          `json:"tipo"`
	Payload json.RawMessage `json:"payload"`
}

type respuestaComandos struct {
	Comandos []Comando `json:"comandos"`
}

// EjecutorAD es lo mínimo que el poller necesita del cliente LDAP --
// *adldap.Cliente lo cumple; en los tests se reemplaza por un fake, ya
// que las llamadas LDAP reales no se pueden simular (mismo límite ya
// documentado del lado PHP de esta integración).
type EjecutorAD interface {
	BuscarUsuario(consulta string) (*adldap.Usuario, error)
	ResetearPassword(consulta string) (string, error)
}

type Poller struct {
	Token   string
	ApiURL  string
	Cliente EjecutorAD
	HTTP    *http.Client
}

func Nuevo(token, apiURL string, cliente EjecutorAD) *Poller {
	return &Poller{
		Token:   token,
		ApiURL:  strings.TrimRight(apiURL, "/"),
		Cliente: cliente,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Ciclo hace UN pase de polling: pide los comandos pendientes y ejecuta
// cada uno. No loopea acá adentro -- el que llama decide el sleep entre
// ciclos (así los tests pueden correr un solo ciclo sin esperar).
func (p *Poller) Ciclo() {
	comandos, err := p.pedirComandos()
	if err != nil {
		log.Printf("no se pudo consultar comandos pendientes: %v", err)
		return
	}

	for _, comando := range comandos {
		p.ejecutar(comando)
	}
}

func (p *Poller) pedirComandos() ([]Comando, error) {
	req, err := http.NewRequest(http.MethodGet, p.ApiURL+"/api/ad-connector/comandos", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("estado %d", resp.StatusCode)
	}

	var datos respuestaComandos
	if err := json.NewDecoder(resp.Body).Decode(&datos); err != nil {
		return nil, err
	}

	return datos.Comandos, nil
}

func (p *Poller) ejecutar(comando Comando) {
	var payload struct {
		Consulta string `json:"consulta"`
	}
	_ = json.Unmarshal(comando.Payload, &payload)

	var resultado any
	var err error

	switch comando.Tipo {
	case TipoBuscarUsuario:
		resultado, err = p.Cliente.BuscarUsuario(payload.Consulta)
	case TipoResetearPassword:
		var password string
		password, err = p.Cliente.ResetearPassword(payload.Consulta)
		if err == nil {
			resultado = map[string]string{"password": password}
		}
	default:
		err = fmt.Errorf("tipo de comando desconocido: %s", comando.Tipo)
	}

	if err != nil {
		p.reportar(comando.ID, "error", nil, err.Error())
		return
	}

	p.reportar(comando.ID, "completado", resultado, "")
}

func (p *Poller) reportar(id int, estado string, resultado any, mensajeError string) {
	body := map[string]any{"estado": estado}
	if resultado != nil {
		body["resultado"] = resultado
	}
	if mensajeError != "" {
		body["error"] = mensajeError
	}

	datos, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/ad-connector/comandos/%d", p.ApiURL, id), bytes.NewReader(datos))
	if err != nil {
		log.Printf("no se pudo armar el reporte del comando %d: %v", id, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTP.Do(req)
	if err != nil {
		log.Printf("no se pudo reportar el resultado del comando %d: %v", id, err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("la API respondió %d al reportar el comando %d", resp.StatusCode, id)
	}
}
