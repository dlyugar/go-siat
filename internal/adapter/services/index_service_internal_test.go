package services

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	siatErrors "github.com/ron86i/go-siat/v2/internal/core/errors"
)

// cuerpoQueFalla simula una conexión que se corta mientras se lee la respuesta.
type cuerpoQueFalla struct{}

func (cuerpoQueFalla) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (cuerpoQueFalla) Close() error             { return nil }

func respuesta(status int, cuerpo string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(cuerpo)),
	}
}

type contenidoCualquiera struct {
	Valor string `xml:"valor"`
}

// El caso del 2026-08-25: a las 19:08:40.486 se cortaron todas las conexiones en vuelo y 108
// facturas quedaron RECHAZADA para siempre porque el EOF salía como error crudo — y
// errors.IsRetryable() responde false para todo lo que no sea *SiatError.
func TestParseSoapResponse_ElCorteDeConexionEsReintentable(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Body: cuerpoQueFalla{}}

	_, err := parseSoapResponse[contenidoCualquiera](resp)

	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if !siatErrors.IsRetryable(err) {
		t.Errorf("un corte de conexión tiene que ser reintentable; salió: %v", err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("el error original tiene que quedar envuelto para diagnóstico; salió: %v", err)
	}
}

// Antes, un 502 con página HTML del gateway se intentaba parsear como XML y salía como
// "expected element type <Envelope> but have <html>", que no nombra la causa.
func TestParseSoapResponse_ElStatusDeErrorSeNombraYEsReintentable(t *testing.T) {
	resp := respuesta(http.StatusBadGateway, "<html><body>502 Bad Gateway</body></html>")

	_, err := parseSoapResponse[contenidoCualquiera](resp)

	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if !siatErrors.IsRetryable(err) {
		t.Errorf("un 502 del gateway tiene que ser reintentable; salió: %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("el mensaje tiene que nombrar el status HTTP; salió: %v", err)
	}
}

// 200 pero el cuerpo no es SOAP: típicamente una página de sesión/portal intercalada.
// Tampoco es un rechazo del SIAT.
func TestParseSoapResponse_CuerpoNoSoapConStatus200EsReintentable(t *testing.T) {
	resp := respuesta(http.StatusOK, "<html><body>Iniciar sesión</body></html>")

	_, err := parseSoapResponse[contenidoCualquiera](resp)

	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if !siatErrors.IsRetryable(err) {
		t.Errorf("un cuerpo no-SOAP tiene que ser reintentable; salió: %v", err)
	}
}

// Reintentar no arregla credenciales inválidas: hace falta renovarlas.
func TestParseSoapResponse_ElRechazoDeAutenticacionNoEsReintentable(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		_, err := parseSoapResponse[contenidoCualquiera](respuesta(status, "denied"))
		if err == nil {
			t.Fatalf("status %d: se esperaba un error", status)
		}
		if siatErrors.IsRetryable(err) {
			t.Errorf("status %d no tiene que ser reintentable; salió: %v", status, err)
		}
	}
}

// Las dos respuestas reales del piloto del SIN del 2026-10-08, tal como llegaron: HTTP 500 con un
// sobre SOAP cuyo faultstring dice la causa. Antes el mensaje decía solo "HTTP 500 en vez de un
// sobre SOAP" y la causa no aparecía en ningún log.
const (
	faultBaseCaida = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><soap:Fault>` +
		`<faultcode>soap:Server</faultcode><faultstring>Could not acquire a connection from DataSource - The connection attempt failed.</faultstring>` +
		`</soap:Fault></soap:Body></soap:Envelope>`
	faultApiKey = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><soap:Fault>` +
		`<faultcode>soap:Server</faultcode><faultstring>API KEY NO VALIDO</faultstring></soap:Fault></soap:Body></soap:Envelope>`
)

func TestParseSoapResponse_ElFaultDelSiatQuedaEnElMensaje(t *testing.T) {
	casos := []struct {
		cuerpo, esperado string
	}{
		{faultBaseCaida, "Could not acquire a connection from DataSource - The connection attempt failed."},
		{faultApiKey, "API KEY NO VALIDO"},
	}
	for _, c := range casos {
		_, err := parseSoapResponse[contenidoCualquiera](respuesta(http.StatusInternalServerError, c.cuerpo))
		if err == nil {
			t.Fatalf("%q: se esperaba un error", c.esperado)
		}
		if !strings.Contains(err.Error(), c.esperado) {
			t.Errorf("el mensaje tiene que llevar el faultstring %q; salió: %v", c.esperado, err)
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("el mensaje tiene que seguir nombrando el status HTTP; salió: %v", err)
		}
		if strings.Contains(err.Error(), "en vez de un sobre SOAP") {
			t.Errorf("trajo un sobre SOAP: el mensaje no puede decir que no; salió: %v", err)
		}
		// La clasificación no cambia: el backend decide contingencia y reintentos con esto.
		if !siatErrors.IsRetryable(err) || !siatErrors.IsNetworkError(err) {
			t.Errorf("un 500 con Fault sigue siendo de red y reintentable; salió: %v", err)
		}
	}
}

// El Fault también se nombra en un 401/403, que sigue sin ser reintentable.
func TestParseSoapResponse_ElFaultDeUnRechazoDeAutenticacionSeNombra(t *testing.T) {
	_, err := parseSoapResponse[contenidoCualquiera](respuesta(http.StatusUnauthorized, faultApiKey))
	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if !strings.Contains(err.Error(), "API KEY NO VALIDO") {
		t.Errorf("el mensaje tiene que llevar el faultstring; salió: %v", err)
	}
	if siatErrors.IsRetryable(err) {
		t.Errorf("un 401 no tiene que ser reintentable; salió: %v", err)
	}
}

// Sin sobre SOAP (la página HTML del gateway) el mensaje queda como antes.
func TestParseSoapResponse_SinSobreElMensajeNoCambia(t *testing.T) {
	for _, cuerpo := range []string{"<html><body>502 Bad Gateway</body></html>", "", "no es xml"} {
		_, err := parseSoapResponse[contenidoCualquiera](respuesta(http.StatusBadGateway, cuerpo))
		if err == nil || !strings.Contains(err.Error(), "en vez de un sobre SOAP") {
			t.Errorf("cuerpo %q: se esperaba el mensaje de siempre; salió: %v", cuerpo, err)
		}
	}
}

// Un faultstring enorme no inunda el log: se acota.
func TestParseSoapResponse_UnFaultLargoSeAcota(t *testing.T) {
	largo := strings.Repeat("x", 5000)
	cuerpo := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><soap:Fault>` +
		`<faultcode>soap:Server</faultcode><faultstring>` + largo + `</faultstring></soap:Fault></soap:Body></soap:Envelope>`
	_, err := parseSoapResponse[contenidoCualquiera](respuesta(http.StatusInternalServerError, cuerpo))
	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if len(err.Error()) > 500 {
		t.Errorf("el mensaje tiene que quedar acotado; mide %d caracteres", len(err.Error()))
	}
}

// La ruta feliz no cambia.
func TestParseSoapResponse_SobreValidoSeParsea(t *testing.T) {
	xmlOk := `<?xml version="1.0"?>
	<Envelope><Body><respuesta><valor>ok</valor></respuesta></Body></Envelope>`

	out, err := parseSoapResponse[contenidoCualquiera](respuesta(http.StatusOK, xmlOk))

	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if out.Body.Content.Valor != "ok" {
		t.Errorf("se esperaba valor=ok, salió %q", out.Body.Content.Valor)
	}
}
