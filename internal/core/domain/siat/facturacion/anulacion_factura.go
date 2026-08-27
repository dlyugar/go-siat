package facturacion

import "encoding/xml"

type AnulacionFactura struct {
	XMLName            xml.Name           `xml:"ns:anulacionFactura" json:"-"`
	SolicitudAnulacion SolicitudAnulacion `xml:"SolicitudServicioAnulacionFactura" json:"solicitudservicioanulacionfactura"`
}

// El ORDEN de los campos importa: encoding/xml serializa en orden de declaración y el
// XSD del SIAT declara esta extensión como una <xs:sequence> con codigoMotivo primero y
// cuf después (ver ServicioFacturacionCompraVenta?wsdl=ServicioFacturacion.wsdl,
// complexType "solicitudAnulacion"). Estaban al revés.
type SolicitudAnulacion struct {
	SolicitudRecepcion
	CodigoMotivo int    `xml:"codigoMotivo" json:"codigomotivo"`
	Cuf          string `xml:"cuf" json:"cuf"`
}

type AnulacionFacturaResponse struct {
	XMLName                      xml.Name           `xml:"anulacionFacturaResponse" json:"-"`
	RespuestaServicioFacturacion RespuestaRecepcion `xml:"RespuestaServicioFacturacion" json:"respuestadelserviciofacturacion"`
}
