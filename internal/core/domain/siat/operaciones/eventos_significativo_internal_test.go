package operaciones

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/ron86i/go-siat/v2/internal/core/domain/datatype"
)

// El WSDL declara fechaEvento como xs:dateTime, y para esos campos el SIN pide el formato
// bare "yyyy-MM-dd'T'HH:mm:ss.SSS". Era el único campo del SDK que salía con offset de huso.
func TestConsultaEventoSignificativo_MandaLaFechaEnElFormatoDelSiat(t *testing.T) {
	req := SolicitudConsultaEvento{
		CodigoAmbiente: 2,
		Cuis:           "CUIS",
		Nit:            7817321012,
		FechaEvento: datatype.TimeSiat(
			time.Date(2026, 8, 25, 23, 54, 39, 957000000, time.FixedZone("BOT", -4*60*60))),
	}

	out, err := xml.Marshal(req)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, "<fechaEvento>2026-08-25T23:54:39.957</fechaEvento>") {
		t.Errorf("fechaEvento no salió en el formato del SIAT.\nSalió: %s", xmlStr)
	}
	if strings.Contains(xmlStr, "-04:00") || strings.Contains(xmlStr, "Z<") {
		t.Errorf("no debe llevar huso: el SIAT usa reloj de pared boliviano.\nSalió: %s", xmlStr)
	}
}

// Es xs:dateTime, no xs:date: mandar sólo el día violaría el esquema del WSDL.
func TestConsultaEventoSignificativo_LlevaHoraNoSoloElDia(t *testing.T) {
	req := SolicitudConsultaEvento{
		FechaEvento: datatype.TimeSiat(time.Date(2026, 8, 25, 23, 54, 39, 0, time.UTC)),
	}

	out, _ := xml.Marshal(req)

	if !strings.Contains(string(out), "T19:54:39") {
		t.Errorf("se esperaba un dateTime completo en hora boliviana (UTC-4).\nSalió: %s", string(out))
	}
}
