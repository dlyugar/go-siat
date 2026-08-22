package operaciones

import (
	"encoding/xml"
	"time"

	"github.com/ron86i/go-siat/v2/internal/core/domain/datatype"
)

// RegistroEventoSignificativo es el wrapper para registrar un evento
type RegistroEventoSignificativo struct {
	XMLName                      xml.Name                     `xml:"ns:registroEventoSignificativo"`
	SolicitudEventoSignificativo SolicitudEventoSignificativo `xml:"SolicitudEventoSignificativo"`
}

// SolicitudEventoSignificativo representa los datos para registrar un evento
type SolicitudEventoSignificativo struct {
	CodigoAmbiente        int       `xml:"codigoAmbiente" json:"codigoAmbiente"`
	CodigoMotivoEvento    int       `xml:"codigoMotivoEvento" json:"codigoMotivoEvento"`
	CodigoPuntoVenta      int       `xml:"codigoPuntoVenta,omitempty" json:"codigoPuntoVenta,omitempty"`
	CodigoSistema         string    `xml:"codigoSistema" json:"codigoSistema"`
	CodigoSucursal        int       `xml:"codigoSucursal" json:"codigoSucursal"`
	Cufd                  string    `xml:"cufd" json:"cufd"`
	CufdEvento            string    `xml:"cufdEvento" json:"cufdEvento"`
	Cuis                  string    `xml:"cuis" json:"cuis"`
	Descripcion           string            `xml:"descripcion" json:"descripcion"`
	// [981] "RANGO DE FECHAS DE EVENTO SIGNIFICATIVO INVALIDO": time.Time simple se
	// serializa por defecto en RFC3339 CON offset de huso ("...-04:00"), no en el
	// formato bare "yyyy-MM-dd'T'HH:mm:ss.SSS" que el SIAT exige (ver doc oficial
	// "Registro Evento Significativo") — el mismo formato que datatype.TimeSiat ya usa
	// en el resto del SDK. Único lugar de este struct que se había quedado con time.Time
	// plano en vez de TimeSiat.
	FechaHoraFinEvento    datatype.TimeSiat `xml:"fechaHoraFinEvento" json:"fechaHoraFinEvento"`
	FechaHoraInicioEvento datatype.TimeSiat `xml:"fechaHoraInicioEvento" json:"fechaHoraInicioEvento"`
	Nit                   int64             `xml:"nit" json:"nit"`
}

// RegistroEventoSignificativoResponse es el wrapper para la respuesta de registro de evento
type RegistroEventoSignificativoResponse struct {
	Respuesta RespuestaListaEventos `xml:"RespuestaListaEventos"`
}

// RespuestaListaEventos representa el resultado de registro/consulta de eventos
type RespuestaListaEventos struct {
	CodigoRecepcionEventoSignificativo int64                      `xml:"codigoRecepcionEventoSignificativo" json:"codigoRecepcionEventoSignificativo"`
	ListaCodigos                       []EventosSignificativosDto `xml:"listaCodigos" json:"listaCodigos"`
	MensajesList                       []MensajeServicio          `xml:"mensajesList" json:"mensajesList"`
	Transaccion                        bool                       `xml:"transaccion" json:"transaccion"`
}

// EventosSignificativosDto representa la información de un evento significativo
type EventosSignificativosDto struct {
	CodigoEvento                       int    `xml:"codigoEvento" json:"codigoEvento"`
	CodigoRecepcionEventoSignificativo int64  `xml:"codigoRecepcionEventoSignificativo" json:"codigoRecepcionEventoSignificativo"`
	Descripcion                        string `xml:"descripcion" json:"descripcion"`
	FechaFin                           string `xml:"fechaFin" json:"fechaFin"`
	FechaInicio                        string `xml:"fechaInicio" json:"fechaInicio"`
}

// ConsultaEventoSignificativo es el wrapper para consultar eventos
type ConsultaEventoSignificativo struct {
	XMLName                 xml.Name                `xml:"ns:consultaEventoSignificativo"`
	SolicitudConsultaEvento SolicitudConsultaEvento `xml:"SolicitudConsultaEvento"`
}

// SolicitudConsultaEvento representa los datos para consultar eventos
type SolicitudConsultaEvento struct {
	CodigoAmbiente   int       `xml:"codigoAmbiente" json:"codigoAmbiente"`
	CodigoPuntoVenta int       `xml:"codigoPuntoVenta" json:"codigoPuntoVenta"`
	CodigoSistema    string    `xml:"codigoSistema" json:"codigoSistema"`
	CodigoSucursal   int       `xml:"codigoSucursal" json:"codigoSucursal"`
	Cuis             string    `xml:"cuis" json:"cuis"`
	FechaEvento      time.Time `xml:"fechaEvento" json:"fechaEvento"`
	Nit              int64     `xml:"nit" json:"nit"`
}

// ConsultaEventoSignificativoResponse es el wrapper para la respuesta de consulta de eventos
type ConsultaEventoSignificativoResponse struct {
	Respuesta RespuestaListaEventos `xml:"RespuestaListaEventos"`
}
