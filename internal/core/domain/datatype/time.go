package datatype

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

const layoutSiat = "2006-01-02T15:04:05.000"

// boliviaLocation es UTC-4 fijo (Bolivia no tiene horario de verano desde 1932).
// FixedZone en vez de LoadLocation("America/La_Paz") para no depender de que la imagen
// del contenedor traiga tzdata.
//
// El formato del SIAT no lleva huso horario: es reloj de pared de Bolivia, y punto. Antes
// esta conversión no existía acá, así que el tipo formateaba/parseaba el reloj de pared del
// time.Time tal cual — es decir, exigía que CADA quien lo usara ya viniera en hora boliviana.
// Eso hacía dos daños:
//
//   - Al PARSEAR (sincronizarFechaHora): time.Parse sin zona asume UTC, así que la hora que
//     el SIAT manda como local de Bolivia se leía como UTC y salía un Instant 4h atrasado.
//     Java lo tomaba como "desfase de reloj" y arrastraba ese -4h a todo (visto real:
//     "Desfase SIAT: -14400152ms"), dejando dos relojes distintos conviviendo en el sistema.
//   - Al FORMATEAR: obligaba a cada call site a pre-convertir. Uno solo lo hacía
//     (fechaEnvioSiat en el conector); el resto compensaba pasando instantes ya corridos.
//
// Con la conversión acá adentro, el resto del sistema maneja instantes REALES y la hora
// boliviana existe únicamente en el cable hacia el SIAT, que es donde corresponde.
var boliviaLocation = time.FixedZone("BOT", -4*60*60)

// TimeSiat es un tipo personalizado que envuelve time.Time para manejar
// la serialización y deserialización XML/JSON con el formato específico del SIAT ("2006-01-02T15:04:05.000").
// Implementa las interfaces de marshaling para asegurar el formato requerido por el SIAT.
type TimeSiat time.Time

// MarshalXML Codificador de XML
func (t TimeSiat) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	v := time.Time(t)
	if v.IsZero() {
		return e.EncodeElement("", start)
	}
	return e.EncodeElement(v.In(boliviaLocation).Format(layoutSiat), start)
}

// UnmarshalXML Decodificador de XML
func (t *TimeSiat) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	elem, err := time.ParseInLocation(layoutSiat, s, boliviaLocation)
	if err != nil {
		return err
	}

	*t = TimeSiat(elem)
	return nil
}

// MarshalJSON Codificador de JSON
func (t TimeSiat) MarshalJSON() ([]byte, error) {
	v := time.Time(t)
	if v.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", v.In(boliviaLocation).Format(layoutSiat))), nil
}

// UnmarshalJSON Decodificador de JSON
func (t *TimeSiat) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == "\"\"" {
		return nil
	}

	s = strings.Trim(s, "\"")
	elem, err := time.ParseInLocation(layoutSiat, s, boliviaLocation)
	if err != nil {
		return err
	}

	*t = TimeSiat(elem)
	return nil
}

// ToTime Helper para convertir de vuelta a time.Time fácilmente
func (t TimeSiat) ToTime() time.Time {
	return time.Time(t)
}

// String devuelve la representación en cadena de texto de la fecha en formato SIAT.
// Esto permite que al imprimir el objeto (ej. en logs) se vea la fecha formateada
// en lugar de los campos internos de time.Time.
func (t TimeSiat) String() string {
	v := time.Time(t)
	if v.IsZero() {
		return ""
	}
	return v.In(boliviaLocation).Format(layoutSiat)
}

func NewTimeSiat(t time.Time) TimeSiat {
	return TimeSiat(t)
}
