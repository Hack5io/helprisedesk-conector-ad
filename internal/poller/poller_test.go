package poller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hack5io/helprisedesk-conector-ad/internal/adldap"
)

// fakeAD reemplaza al cliente LDAP real -- las llamadas LDAP reales no
// se pueden automatizar en un test (mismo límite ya documentado del lado
// PHP: ldap_*() no se puede fakear). Acá se prueba el round-trip HTTP
// contra un httptest.Server, no el protocolo LDAP en sí.
type fakeAD struct {
	usuario       *adldap.Usuario
	errorBuscar   error
	passwordReset string
	errorReset    error
}

func (f *fakeAD) BuscarUsuario(consulta string) (*adldap.Usuario, error) {
	return f.usuario, f.errorBuscar
}

func (f *fakeAD) ResetearPassword(consulta string) (string, error) {
	return f.passwordReset, f.errorReset
}

func TestCicloBuscarUsuarioExitosoReportaCompletado(t *testing.T) {
	var reportado map[string]any

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-de-prueba" {
			t.Errorf("token inesperado: %s", r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/ad-connector/comandos":
			_ = json.NewEncoder(w).Encode(respuestaComandos{
				Comandos: []Comando{{ID: 1, Tipo: TipoBuscarUsuario, Payload: json.RawMessage(`{"consulta":"lsanabria"}`)}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/ad-connector/comandos/1":
			_ = json.NewDecoder(r.Body).Decode(&reportado)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("request inesperado: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer servidor.Close()

	fake := &fakeAD{usuario: &adldap.Usuario{SamAccountName: "lsanabria", Nombre: "Luis Sanabria"}}
	p := Nuevo("token-de-prueba", servidor.URL, fake)
	p.Ciclo()

	if reportado["estado"] != "completado" {
		t.Fatalf("esperaba estado completado, dio %v", reportado)
	}
	resultado, ok := reportado["resultado"].(map[string]any)
	if !ok || resultado["samAccountName"] != "lsanabria" {
		t.Fatalf("resultado inesperado: %v", reportado)
	}
}

func TestCicloBuscarUsuarioNoEncontradoReportaResultadoNulo(t *testing.T) {
	var reportado map[string]any

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(respuestaComandos{
				Comandos: []Comando{{ID: 2, Tipo: TipoBuscarUsuario, Payload: json.RawMessage(`{"consulta":"no-existe"}`)}},
			})
		case r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&reportado)
		}
	}))
	defer servidor.Close()

	fake := &fakeAD{usuario: nil}
	p := Nuevo("token", servidor.URL, fake)
	p.Ciclo()

	if reportado["estado"] != "completado" {
		t.Fatalf("usuario no encontrado no es un error, esperaba completado: %v", reportado)
	}
	// "resultado": null es un valor válido para el servidor (ver
	// 'resultado' => 'nullable|array' en AdConnectorController::resultado())
	// -- significa "no encontrado", no un error.
	if reportado["resultado"] != nil {
		t.Fatalf("esperaba resultado null cuando el usuario no existe: %v", reportado)
	}
}

func TestCicloConErrorDeLdapReportaError(t *testing.T) {
	var reportado map[string]any

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(respuestaComandos{
				Comandos: []Comando{{ID: 3, Tipo: TipoResetearPassword, Payload: json.RawMessage(`{"consulta":"lsanabria"}`)}},
			})
		case r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&reportado)
		}
	}))
	defer servidor.Close()

	fake := &fakeAD{errorReset: fmt.Errorf("no se pudo autenticar contra el AD local")}
	p := Nuevo("token", servidor.URL, fake)
	p.Ciclo()

	if reportado["estado"] != "error" {
		t.Fatalf("esperaba estado error, dio %v", reportado)
	}
	if reportado["error"] != "no se pudo autenticar contra el AD local" {
		t.Fatalf("mensaje de error inesperado: %v", reportado)
	}
}

func TestCicloResetearPasswordExitosoReportaLaPassword(t *testing.T) {
	var reportado map[string]any

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(respuestaComandos{
				Comandos: []Comando{{ID: 4, Tipo: TipoResetearPassword, Payload: json.RawMessage(`{"consulta":"lsanabria"}`)}},
			})
		case r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&reportado)
		}
	}))
	defer servidor.Close()

	fake := &fakeAD{passwordReset: "Xy7!kLp9qR2z"}
	p := Nuevo("token", servidor.URL, fake)
	p.Ciclo()

	resultado, ok := reportado["resultado"].(map[string]any)
	if !ok || resultado["password"] != "Xy7!kLp9qR2z" {
		t.Fatalf("esperaba la password en el resultado: %v", reportado)
	}
}
