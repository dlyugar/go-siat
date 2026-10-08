package services

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/ron86i/go-siat/v2/internal/core/domain/datatype/soap"
	siatErrors "github.com/ron86i/go-siat/v2/internal/core/errors"
	"github.com/ron86i/go-siat/v2/internal/core/ports"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

// SiatService define los diferentes servicios disponibles en el SIAT.
type SiatService string

const (
	SiatCodigos            SiatService = "FacturacionCodigos"
	SiatOperaciones        SiatService = "FacturacionOperaciones"
	SiatSincronizacion     SiatService = "FacturacionSincronizacion"
	SiatCompraVenta        SiatService = "ServicioFacturacionCompraVenta"
	SiatComputarizada      SiatService = "ServicioFacturacionComputarizada"
	SiatElectronica        SiatService = "ServicioFacturacionElectronica"
	SiatDocumentoAjuste    SiatService = "ServicioFacturacionDocumentoAjuste"
	SiatTelecomunicaciones SiatService = "ServicioFacturacionTelecomunicaciones"
	SiatServicioBasico     SiatService = "ServicioFacturacionServicioBasico"
	SiatEntidadFinanciera  SiatService = "ServicioFacturacionEntidadFinanciera"
	SiatBoletoAereo        SiatService = "ServicioFacturacionBoletoAereo"
	SiatRecepcionCompras   SiatService = "ServicioRecepcionCompras"
	SiatHidrocarburos      SiatService = "ServicioFacturacionHidrocarburos"
)

// fullURL construye la URL completa para acceder a un servicio específico del SIAT,
// concatenando la URL base del ambiente con el endpoint del servicio solicitado.
func fullURL(baseURL string, service SiatService) string {
	return baseURL + "/" + string(service)
}

// buildRequest encapsula un objeto de solicitud genérico dentro de un sobre SOAP estándar (Envelope),
// añadiendo los namespaces requeridos por el SIAT y serializando el resultado a formato XML.
func buildRequest(req any) ([]byte, error) {
	requestBody := soap.Envelope[any]{
		XmlnsSoapenv: "http://schemas.xmlsoap.org/soap/envelope/",
		XmlnsNs:      "https://siat.impuestos.gob.bo/",
		XmlnsXsi:     "http://www.w3.org/2001/XMLSchema-instance",
		Body: soap.EnvelopeBody[any]{
			Content: req,
		},
	}

	xmlBody, err := xml.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, err
	}
	return []byte(xml.Header + string(xmlBody)), nil
}

// parseSoapResponse procesa y valida una respuesta HTTP proveniente del servicio para extraer el contenido SOAP esperado.
//
// Los tres fallos de acá son de TRANSPORTE, no rechazos del SIAT: la petición ya salió y lo
// que se rompió fue la respuesta. Devolverlos crudos los volvía no reintentables, porque
// errors.IsRetryable() responde false para todo lo que no sea un *SiatError — así que un
// parpadeo de red mataba la factura de forma permanente.
//
// Visto real el 2026-08-25: a las 19:08:40.486 se cortaron TODAS las conexiones en vuelo de
// golpe (duraciones de 4 s a 27 s fallando en el mismo milisegundo) y 108 facturas de una
// certificación quedaron RECHAZADA para siempre por un corte de un segundo del lado del SIAT.
// 78 con "EOF" y 30 con "expected element type <Envelope> but have <html>".
//
// Envolverlos como error de red los hace reintentables. Es seguro porque el reintento no
// reenvía a ciegas: el backend consulta antes con VerificacionEstadoFactura si el SIAT ya
// tiene el CUF (ver FacturaYaEnSiatChecker) — un EOF es ambiguo por definición y la petición
// pudo haberse procesado.
func parseSoapResponse[T any](resp *http.Response) (*soap.EnvelopeResponse[T], error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// La conexión se cortó mientras leíamos: no sabemos si el SIAT llegó a procesar.
		return nil, siatErrors.NewNetworkError("la conexión con el SIAT se cortó al leer la respuesta", err)
	}

	// Un status fuera de 2xx casi nunca trae un sobre SOAP: viene la página de error del
	// gateway o del portal. Detectarlo acá da un mensaje que nombra la causa, en vez del
	// confuso "expected element type <Envelope> but have <html>" que salía al intentar
	// parsear un HTML como XML.
	//
	// Cuando sí trae un sobre SOAP con Fault (el SIAT contesta así sus propios errores, con
	// HTTP 500), el mensaje lleva su faultstring. Antes se descartaba y el log solo decía
	// "HTTP 500 en vez de un sobre SOAP": el 2026-10-08 el piloto estuvo horas contestando
	// «Could not acquire a connection from DataSource» (su base caída) y para verlo hubo que
	// prender el volcado del tráfico. La clasificación no cambia: sigue siendo de red y
	// reintentable, salvo 401/403.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("el SIAT respondió HTTP %d (%s) en vez de un sobre SOAP",
			resp.StatusCode, http.StatusText(resp.StatusCode))
		if fault := faultDe(body); fault != "" {
			msg = fmt.Sprintf("el SIAT respondió HTTP %d (%s) con el error SOAP «%s»",
				resp.StatusCode, http.StatusText(resp.StatusCode), truncar([]byte(fault), 300))
		}
		// 401/403 no se arreglan reintentando: hace falta renovar credenciales.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, siatErrors.NewAuthError(msg)
		}
		return nil, siatErrors.NewNetworkError(msg, fmt.Errorf("cuerpo: %s", truncar(body, 200)))
	}

	var result soap.EnvelopeResponse[T]

	// Intentar parsear la respuesta XML en la estructura de respuesta SOAP
	if err := xml.Unmarshal(body, &result); err != nil {
		// 200 pero el cuerpo no es SOAP: típicamente una página de sesión/portal
		// intercalada. Tampoco es un rechazo del SIAT, así que se reintenta.
		return nil, siatErrors.NewNetworkError(
			fmt.Sprintf("la respuesta del SIAT no es un sobre SOAP válido (cuerpo: %s)", truncar(body, 200)), err)
	}

	return &result, nil
}

// faultDe devuelve el faultstring si el cuerpo es un sobre SOAP con Fault, o "" si no lo es
// (una página HTML del gateway, un cuerpo vacío o un sobre sin Fault).
func faultDe(body []byte) string {
	var sobre soap.EnvelopeResponse[struct{}]
	if err := xml.Unmarshal(body, &sobre); err != nil || sobre.Body.Fault == nil {
		return ""
	}
	return strings.TrimSpace(sobre.Body.Fault.FaultString)
}

// truncar acota el cuerpo que se adjunta al error: alcanza para reconocer si vino una página
// de login, un 502 del gateway o basura, sin volcar una respuesta entera al log.
func truncar(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// getInternalRequest desempaqueta la estructura de solicitud concreta desde una interfaz opaca
// delegando la operación al paquete models. Esto mantiene la opacidad hacia el usuario final
// mientras permite que las capas internas accedan a los datos necesarios para la comunicación.
func getInternalRequest[T any](req any) *T {
	return models.UnwrapInternalRequest[T](req)
}

// injectFields recorre recursivamente el struct para rellenar los campos comunes del contribuyente.
func injectFields(v reflect.Value, config ports.Config) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		ft := v.Type().Field(i)

		// Si es un struct anidado, procesarlo de forma recursiva
		if fv.Kind() == reflect.Struct {
			if fv.CanAddr() {
				injectFields(fv.Addr(), config)
			} else {
				injectFields(fv, config)
			}
			continue
		}

		if fv.CanSet() {
			switch ft.Name {
			case "Nit", "NIT":
				// Solo inyecta si el campo no fue seteado explícitamente en el request.
				if fv.Kind() == reflect.Int64 && fv.Int() == 0 {
					fv.SetInt(config.Nit)
				}
			case "CodigoSistema":
				// Solo inyecta si el campo no fue seteado explícitamente en el request.
				if fv.Kind() == reflect.String && fv.String() == "" {
					fv.SetString(config.CodigoSistema)
				}
			case "CodigoAmbiente":
				// Solo inyecta si el campo no fue seteado explícitamente en el request.
				switch fv.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if fv.Int() == 0 {
						fv.SetInt(int64(config.CodigoAmbiente))
					}
				}
			}
		}
	}
}

// injectCredenciales inyecta las credenciales del contribuyente en el request.
func injectCredenciales(req any, config ports.Config) {
	v := reflect.ValueOf(req)
	injectFields(v, config)
}

/*
performSoapRequest es una función genérica que encapsula el flujo completo de una solicitud SOAP al SIAT:

1. Obtiene la solicitud interna desde la interfaz opaca.

2. Inyecta automáticamente los parámetros globales de la sesión (NIT, Sistema, Ambiente, Modalidad).

3. Construye el cuerpo XML (Envelope SOAP).

4. Crea la solicitud HTTP POST con el contexto y headers necesarios (incluyendo el token de API).

5. Ejecuta la solicitud a través del cliente HTTP.

6. Procesa y decodifica la respuesta SOAP.
*/
func performSoapRequest[TReq any, TResp any](ctx context.Context, httpClient *http.Client, url string, config ports.Config, opaqueReq any) (*soap.EnvelopeResponse[TResp], error) {

	// Merge con config dinámica si existe en el contexto
	if dynCfg, ok := ports.GetConfigFromContext(ctx); ok {
		if dynCfg.Token != "" {
			config.Token = dynCfg.Token
		}
		if dynCfg.Nit != 0 {
			config.Nit = dynCfg.Nit
		}
		if dynCfg.CodigoSistema != "" {
			config.CodigoSistema = dynCfg.CodigoSistema
		}
		if dynCfg.CodigoAmbiente != 0 {
			config.CodigoAmbiente = dynCfg.CodigoAmbiente
		}
		if dynCfg.TraceId != "" {
			config.TraceId = dynCfg.TraceId
		}
		if dynCfg.CredentialSign.GetType() != "UNKNOWN" {
			config.CredentialSign = dynCfg.CredentialSign
		}
	}

	req := getInternalRequest[TReq](opaqueReq)
	injectCredenciales(req, config)

	xmlBody, err := buildRequest(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(xmlBody))
	if err != nil {
		return nil, err
	}

	ua := config.UserAgent
	if strings.TrimSpace(ua) == "" {
		ua = "go-siat"
	}
	httpReq.Header.Set("User-Agent", ua)
	httpReq.Header.Set("Content-Type", "application/xml")
	httpReq.Header.Set("apiKey", fmt.Sprintf("TokenApi %s", config.Token))

	// Inyectar X-Trace-ID si está disponible
	if strings.TrimSpace(config.TraceId) != "" {
		httpReq.Header.Set("X-Trace-ID", config.TraceId)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, siatErrors.NewNetworkError("fallo en solicitud HTTP al SIAT", err)
	}
	return parseSoapResponse[TResp](resp)
}
