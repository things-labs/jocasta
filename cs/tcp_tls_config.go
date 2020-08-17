package cs

// TCPTlsConfig tcp tls config
type TCPTlsConfig struct {
	CaCert []byte
	Cert   []byte
	Key    []byte
	Single bool
}
