// Package adldap es el puerto a Go de App\Services\ActiveDirectory\LdapEjecutor.php
// (repo helprisedesk, Laravel) -- mismos guardarraíles, mismo
// descubrimiento de hostname/CA. La diferencia real: acá se usa
// crypto/tls de la stdlib de Go para LDAPS en vez de la extensión ldap de
// PHP sobre GnuTLS -- una pila de TLS completamente distinta, así que el
// truco del directorio con symlink hasheado (LDAPTLS_CACERTDIR) de la
// versión PHP NO aplica acá. Go arma un x509.CertPool en memoria
// directamente. Esto se verificó de nuevo contra un AD real, no se
// asumió que "iba a andar solo" por ser Go.
package adldap

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"unicode/utf16"

	"github.com/go-ldap/ldap/v3"
)

type Cliente struct {
	Host             string
	Dominio          string
	Usuario          string
	Password         string
	CuentasExcluidas []string
}

type Usuario struct {
	DN                   string `json:"dn"`
	SamAccountName       string `json:"samAccountName"`
	Nombre               string `json:"nombre"`
	Correo               string `json:"correo,omitempty"`
	Habilitado           bool   `json:"habilitado"`
	UltimoCambioPassword string `json:"ultimoCambioPassword,omitempty"`
}

// BuscarUsuario busca por samAccountName, UPN o correo -- SOLO LECTURA.
// nil (sin error) significa "no encontrado", un resultado válido.
func (c *Cliente) BuscarUsuario(consulta string) (*Usuario, error) {
	conn, err := c.conectar()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return c.buscarEntradaUsuario(conn, consulta)
}

// ResetearPassword -- mismos guardarraíles que LdapEjecutor::resetearPassword():
// 1) el usuario tiene que existir, 2) no puede ser la propia cuenta de
// servicio, 3) no puede estar en CuentasExcluidas.
func (c *Cliente) ResetearPassword(consulta string) (string, error) {
	connLectura, err := c.conectar()
	if err != nil {
		return "", err
	}
	usuario, err := c.buscarEntradaUsuario(connLectura, consulta)
	connLectura.Close()
	if err != nil {
		return "", err
	}
	if usuario == nil {
		return "", fmt.Errorf("no se encontró ningún usuario para %q en Active Directory", consulta)
	}

	if strings.EqualFold(usuario.SamAccountName, c.Usuario) {
		return "", fmt.Errorf("no se puede resetear la contraseña de la propia cuenta de servicio de la integración")
	}
	for _, excluida := range c.CuentasExcluidas {
		if strings.EqualFold(usuario.SamAccountName, excluida) {
			return "", fmt.Errorf("la cuenta %q está en la lista de cuentas excluidas -- no se puede resetear desde acá", usuario.SamAccountName)
		}
	}

	nuevaPassword := generarPasswordTemporal()

	connLdaps, err := c.conectarLdaps()
	if err != nil {
		return "", err
	}
	defer connLdaps.Close()

	// unicodePwd exige el valor entre comillas, codificado en UTF-16LE --
	// así lo pide Active Directory, no es una decisión de esta implementación.
	// https://learn.microsoft.com/troubleshoot/windows-server/identity/set-user-password-with-ldifde
	codificada := codificarUnicodePwd(nuevaPassword)

	modReq := ldap.NewModifyRequest(usuario.DN, nil)
	modReq.Replace("unicodePwd", []string{string(codificada)})
	if err := connLdaps.Modify(modReq); err != nil {
		return "", fmt.Errorf("Active Directory rechazó la contraseña nueva: %w -- revisá que cumpla la política de complejidad del dominio", err)
	}

	// pwdLastSet=0 obliga a cambiarla en el próximo login -- una
	// contraseña temporal no debería quedar como definitiva.
	forzarCambioReq := ldap.NewModifyRequest(usuario.DN, nil)
	forzarCambioReq.Replace("pwdLastSet", []string{"0"})
	_ = connLdaps.Modify(forzarCambioReq) // best-effort, igual que en la versión PHP

	return nuevaPassword, nil
}

func (c *Cliente) conectar() (*ldap.Conn, error) {
	if c.Host == "" || c.Dominio == "" || c.Usuario == "" {
		return nil, fmt.Errorf("Active Directory no está configurado (falta host, dominio o usuario)")
	}
	if c.Password == "" {
		return nil, fmt.Errorf("Active Directory: falta la contraseña de la cuenta de servicio")
	}

	conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s", c.Host))
	if err != nil {
		return nil, fmt.Errorf("Active Directory: no se pudo iniciar la conexión a %q: %w", c.Host, err)
	}

	if err := conn.Bind(fmt.Sprintf("%s@%s", c.Usuario, c.Dominio), c.Password); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Active Directory: no se pudo autenticar: %w", err)
	}

	return conn, nil
}

// conectarLdaps -- LDAPS/636 para el reset. Descubre el hostname real
// (dnsHostName del RootDSE, IMPRESCINDIBLE: conectar por IP contra un
// cert emitido para un hostname puede fallar la verificación TLS según
// el cliente) y el certificado de CA del dominio (partición de
// Configuration), arma un x509.CertPool en memoria y listo -- sin
// archivos temporales ni symlinks, a diferencia de la versión PHP.
func (c *Cliente) conectarLdaps() (*ldap.Conn, error) {
	if c.Password == "" {
		return nil, fmt.Errorf("Active Directory: falta la contraseña de la cuenta de servicio")
	}

	connLectura, err := c.conectar()
	if err != nil {
		return nil, err
	}
	hostname, err := c.obtenerHostnameReal(connLectura)
	if err != nil {
		connLectura.Close()
		return nil, err
	}
	pool, err := c.obtenerCertPoolCA(connLectura)
	connLectura.Close()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName: hostname,
		RootCAs:    pool,
	}

	conn, err := ldap.DialURL(fmt.Sprintf("ldaps://%s:636", hostname), ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, fmt.Errorf("Active Directory: no se pudo iniciar la conexión LDAPS a %q: %w", hostname, err)
	}

	if err := conn.Bind(fmt.Sprintf("%s@%s", c.Usuario, c.Dominio), c.Password); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Active Directory (LDAPS): no se pudo autenticar: %w", err)
	}

	return conn, nil
}

func (c *Cliente) buscarEntradaUsuario(conn *ldap.Conn, consulta string) (*Usuario, error) {
	baseDn, err := c.obtenerBaseDn(conn)
	if err != nil {
		return nil, err
	}

	consultaEscapada := ldap.EscapeFilter(consulta)
	filtro := fmt.Sprintf(
		"(&(objectClass=user)(objectCategory=person)(|(sAMAccountName=%s)(userPrincipalName=%s)(mail=%s)))",
		consultaEscapada, consultaEscapada, consultaEscapada,
	)

	req := ldap.NewSearchRequest(
		baseDn, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filtro,
		[]string{"sAMAccountName", "displayName", "mail", "userAccountControl", "pwdLastSet", "distinguishedName"},
		nil,
	)

	resultado, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("Active Directory: falló la búsqueda: %w", err)
	}
	if len(resultado.Entries) == 0 {
		return nil, nil
	}

	entrada := resultado.Entries[0]

	// Bit 2 (valor 0x2) de userAccountControl = cuenta deshabilitada.
	// https://learn.microsoft.com/windows/win32/adschema/a-useraccountcontrol
	uac := entrada.GetAttributeValue("userAccountControl")
	habilitado := true
	if uac != "" {
		var v int
		fmt.Sscanf(uac, "%d", &v)
		habilitado = v&2 == 0
	}

	return &Usuario{
		DN:             entrada.DN,
		SamAccountName: entrada.GetAttributeValue("sAMAccountName"),
		Nombre:         entrada.GetAttributeValue("displayName"),
		Correo:         entrada.GetAttributeValue("mail"),
		Habilitado:     habilitado,
		// El cálculo de fecha desde pwdLastSet (Windows FILETIME) queda
		// en el servidor Laravel si hiciera falta mostrarlo -- acá solo
		// se reportan los campos que el dispatcher realmente usa.
	}, nil
}

func (c *Cliente) obtenerBaseDn(conn *ldap.Conn) (string, error) {
	valor, err := leerRootDSE(conn, "defaultNamingContext")
	if err != nil {
		return "", fmt.Errorf("Active Directory: no se pudo leer el RootDSE para encontrar el base DN: %w", err)
	}
	if valor == "" {
		return "", fmt.Errorf("Active Directory: el RootDSE no trajo un defaultNamingContext")
	}
	return valor, nil
}

func (c *Cliente) obtenerHostnameReal(conn *ldap.Conn) (string, error) {
	valor, err := leerRootDSE(conn, "dnsHostName")
	if err != nil {
		return "", fmt.Errorf("Active Directory: no se pudo leer el RootDSE para encontrar el hostname real: %w", err)
	}
	if valor == "" {
		return "", fmt.Errorf("Active Directory: el RootDSE no trajo un dnsHostName -- no se puede armar la conexión LDAPS")
	}
	return valor, nil
}

func leerRootDSE(conn *ldap.Conn, atributo string) (string, error) {
	req := ldap.NewSearchRequest(
		"", ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)", []string{atributo}, nil,
	)
	resultado, err := conn.Search(req)
	if err != nil {
		return "", err
	}
	if len(resultado.Entries) == 0 {
		return "", nil
	}
	return resultado.Entries[0].GetAttributeValue(atributo), nil
}

func (c *Cliente) obtenerCertPoolCA(conn *ldap.Conn) (*x509.CertPool, error) {
	baseDn, err := c.obtenerBaseDn(conn)
	if err != nil {
		return nil, err
	}
	baseConfig := fmt.Sprintf("CN=Certification Authorities,CN=Public Key Services,CN=Services,CN=Configuration,%s", baseDn)

	req := ldap.NewSearchRequest(
		baseConfig, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=certificationAuthority)", []string{"cACertificate"}, nil,
	)

	resultado, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("Active Directory: no se pudo leer el certificado del CA del dominio: %w", err)
	}
	if len(resultado.Entries) == 0 {
		return nil, fmt.Errorf("Active Directory: no se encontró ningún certificado de CA publicado en el dominio")
	}

	der := resultado.Entries[0].GetRawAttributeValue("cACertificate")
	if len(der) == 0 {
		return nil, fmt.Errorf("Active Directory: no se encontró ningún certificado de CA publicado en el dominio")
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("Active Directory: el certificado de CA del dominio no se pudo interpretar: %w", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	return pool, nil
}

// codificarUnicodePwd envuelve la contraseña entre comillas y la
// codifica en UTF-16LE -- así lo exige el atributo unicodePwd de AD.
func codificarUnicodePwd(password string) []byte {
	conComillas := `"` + password + `"`
	unidades := utf16.Encode([]rune(conComillas))

	buf := make([]byte, len(unidades)*2)
	for i, u := range unidades {
		binary.LittleEndian.PutUint16(buf[i*2:], u)
	}

	return buf
}

func generarPasswordTemporal() string {
	const mayusculas = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const minusculas = "abcdefghijkmnpqrstuvwxyz"
	const numeros = "23456789"
	const simbolos = "!@#$%&*"
	const todos = mayusculas + minusculas + numeros + simbolos

	letra := func(alfabeto string) byte {
		return alfabeto[rand.Intn(len(alfabeto))]
	}

	password := []byte{
		letra(mayusculas), letra(minusculas), letra(numeros), letra(simbolos),
	}
	for i := 0; i < 8; i++ {
		password = append(password, letra(todos))
	}

	rand.Shuffle(len(password), func(i, j int) {
		password[i], password[j] = password[j], password[i]
	})

	return string(password)
}
