package datatype

import (
	"encoding/json"
	"encoding/xml"
	"testing"
	"time"
)

// El SIAT habla reloj de pared de Bolivia (UTC-4) sin huso en la cadena. Estos tests fijan
// ese contrato en el tipo: quien lo use maneja instantes REALES y la conversión pasa acá.
// Antes no era así y el parseo asumía UTC, lo que metía un "desfase" falso de -4h en todo
// el sistema (ver comentario de boliviaLocation).

const (
	// Mismo instante expresado de las dos formas.
	instanteUTC     = "2026-08-22T20:11:25.000Z"
	relojBoliviaStr = "2026-08-22T16:11:25.000"
)

func TestMarshalXMLUsaRelojDeBolivia(t *testing.T) {
	instante, err := time.Parse(time.RFC3339, instanteUTC)
	if err != nil {
		t.Fatalf("no se pudo preparar el instante: %v", err)
	}

	type envoltorio struct {
		Fecha TimeSiat `xml:"fecha"`
	}
	salida, err := xml.Marshal(envoltorio{Fecha: NewTimeSiat(instante)})
	if err != nil {
		t.Fatalf("marshal falló: %v", err)
	}

	esperado := "<envoltorio><fecha>" + relojBoliviaStr + "</fecha></envoltorio>"
	if string(salida) != esperado {
		t.Errorf("XML con huso incorrecto:\n  esperado %s\n  obtenido %s", esperado, string(salida))
	}
}

func TestMarshalJSONUsaRelojDeBolivia(t *testing.T) {
	instante, _ := time.Parse(time.RFC3339, instanteUTC)

	salida, err := json.Marshal(NewTimeSiat(instante))
	if err != nil {
		t.Fatalf("marshal falló: %v", err)
	}
	if string(salida) != `"`+relojBoliviaStr+`"` {
		t.Errorf("JSON con huso incorrecto: esperado %q, obtenido %s", relojBoliviaStr, string(salida))
	}
}

// Este es el que importa para sincronizarFechaHora: la cadena del SIAT es hora boliviana,
// y tiene que volver como el instante REAL, no 4h atrasado.
func TestUnmarshalInterpretaLaCadenaComoHoraBoliviana(t *testing.T) {
	esperado, _ := time.Parse(time.RFC3339, instanteUTC)

	var deJSON TimeSiat
	if err := json.Unmarshal([]byte(`"`+relojBoliviaStr+`"`), &deJSON); err != nil {
		t.Fatalf("unmarshal JSON falló: %v", err)
	}
	if !deJSON.ToTime().Equal(esperado) {
		t.Errorf("JSON: esperado el instante %s, obtenido %s (diferencia %v)",
			esperado, deJSON.ToTime().UTC(), deJSON.ToTime().Sub(esperado))
	}

	type envoltorio struct {
		Fecha TimeSiat `xml:"fecha"`
	}
	var deXML envoltorio
	entrada := "<envoltorio><fecha>" + relojBoliviaStr + "</fecha></envoltorio>"
	if err := xml.Unmarshal([]byte(entrada), &deXML); err != nil {
		t.Fatalf("unmarshal XML falló: %v", err)
	}
	if !deXML.Fecha.ToTime().Equal(esperado) {
		t.Errorf("XML: esperado el instante %s, obtenido %s (diferencia %v)",
			esperado, deXML.Fecha.ToTime().UTC(), deXML.Fecha.ToTime().Sub(esperado))
	}
}

// Un instante que entra tiene que salir igual: sin esto, cada viaje de ida y vuelta
// correría la fecha 4h y el error se acumularía en silencio.
func TestIdaYVueltaPreservaElInstante(t *testing.T) {
	original, _ := time.Parse(time.RFC3339, instanteUTC)

	serializado, err := json.Marshal(NewTimeSiat(original))
	if err != nil {
		t.Fatalf("marshal falló: %v", err)
	}
	var vuelta TimeSiat
	if err := json.Unmarshal(serializado, &vuelta); err != nil {
		t.Fatalf("unmarshal falló: %v", err)
	}
	if !vuelta.ToTime().Equal(original) {
		t.Errorf("ida y vuelta corrió el instante: %s -> %s", original, vuelta.ToTime().UTC())
	}
}

// El origen del time.Time no debe importar: dos representaciones del mismo instante
// tienen que producir la misma cadena. Si esto falla, volvimos a depender de que el
// llamador pre-convierta.
func TestElHusoDeEntradaNoCambiaLaSalida(t *testing.T) {
	enUTC, _ := time.Parse(time.RFC3339, instanteUTC)
	enOtroHuso := enUTC.In(time.FixedZone("TEST", 9*60*60))

	if NewTimeSiat(enUTC).String() != NewTimeSiat(enOtroHuso).String() {
		t.Errorf("mismo instante, cadenas distintas: %q vs %q",
			NewTimeSiat(enUTC).String(), NewTimeSiat(enOtroHuso).String())
	}
	if NewTimeSiat(enOtroHuso).String() != relojBoliviaStr {
		t.Errorf("esperado %q, obtenido %q", relojBoliviaStr, NewTimeSiat(enOtroHuso).String())
	}
}
