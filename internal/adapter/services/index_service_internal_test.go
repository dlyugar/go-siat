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
