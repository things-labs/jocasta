package socks5

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/core/basicAuth"
)

const (
	Method_NO_AUTH         = uint8(0x00)
	Method_GSSAPI          = uint8(0x01)
	Method_USER_PASS       = uint8(0x02)
	Method_IANA            = uint8(0x7F)
	Method_RESVERVE        = uint8(0x80)
	Method_NONE_ACCEPTABLE = uint8(0xFF)
	VERSION_V5             = uint8(0x05)
	CMD_CONNECT            = uint8(0x01)
	CMD_BIND               = uint8(0x02)
	CMD_ASSOCIATE          = uint8(0x03)
	ATYP_IPV4              = uint8(0x01)
	ATYP_DOMAIN            = uint8(0x03)
	ATYP_IPV6              = uint8(0x04)
	REP_SUCCESS            = uint8(0x00)
	REP_REQ_FAIL           = uint8(0x01)
	REP_RULE_FORBIDDEN     = uint8(0x02)
	REP_NETWOR_UNREACHABLE = uint8(0x03)
	REP_HOST_UNREACHABLE   = uint8(0x04)
	REP_CONNECTION_REFUSED = uint8(0x05)
	REP_TTL_TIMEOUT        = uint8(0x06)
	REP_CMD_UNSUPPORTED    = uint8(0x07)
	REP_ATYP_UNSUPPORTED   = uint8(0x08)
	REP_UNKNOWN            = uint8(0x09)
	RSV                    = uint8(0x00)
)

type Server struct {
	target          string
	pAuth           proxy.Auth
	user            string
	password        string
	conn            net.Conn
	timeout         time.Duration
	basicAuthCenter *basicAuth.Center
	header          []byte
	ver             uint8
	//method
	methodsCount uint8
	methods      []uint8
	method       uint8
	//request
	cmd             uint8
	reserve         uint8
	addressType     uint8
	dstAddr         string
	dstPort         string
	dstHost         string
	UDPConnListener *net.UDPConn
	enableUDP       bool
	udpIP           string
}

func NewServer(conn net.Conn, timeout time.Duration, auth *basicAuth.Center, enableUDP bool, udpHost string, header []byte) *Server {
	return &Server{
		conn:            conn,
		timeout:         timeout,
		basicAuthCenter: auth,
		header:          header,
		ver:             VERSION_V5,
		enableUDP:       enableUDP,
		udpIP:           udpHost,
	}
}
func (s *Server) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}
func (s *Server) AuthData() proxy.Auth {
	return s.pAuth
}
func (s *Server) IsUDP() bool {
	return s.cmd == CMD_ASSOCIATE
}
func (s *Server) IsTCP() bool {
	return s.cmd == CMD_CONNECT
}
func (s *Server) Method() uint8 {
	return s.method
}
func (s *Server) Target() string {
	return s.target
}

func (s *Server) Handshake() (err error) {
	remoteAddr := s.conn.RemoteAddr()
	localAddr := s.conn.LocalAddr()
	//协商开始
	//method select request
	var methodReq MethodsRequest
	s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	methodReq, e := NewMethodsRequest(s.conn, s.header)
	s.conn.SetReadDeadline(time.Time{})
	if e != nil {
		s.conn.SetReadDeadline(time.Now().Add(s.timeout))
		methodReq.Reply(Method_NONE_ACCEPTABLE)
		s.conn.SetReadDeadline(time.Time{})
		err = fmt.Errorf("new methods request fail,ERR: %s", e)
		return
	}

	if s.basicAuthCenter == nil && methodReq.Select(Method_NO_AUTH) && !methodReq.Select(Method_USER_PASS) {
		s.method = Method_NO_AUTH
		//method select reply
		s.conn.SetReadDeadline(time.Now().Add(s.timeout))
		err = methodReq.Reply(Method_NO_AUTH)
		s.conn.SetReadDeadline(time.Time{})
		if err != nil {
			err = fmt.Errorf("reply answer data fail,ERR: %s", err)
			return
		}
	} else {
		if !methodReq.Select(Method_USER_PASS) {
			s.conn.SetReadDeadline(time.Now().Add(s.timeout))
			methodReq.Reply(Method_NONE_ACCEPTABLE)
			s.conn.SetReadDeadline(time.Time{})
			err = fmt.Errorf("none method found : Method_USER_PASS")
			return
		}
		s.method = Method_USER_PASS
		s.conn.SetReadDeadline(time.Now().Add(s.timeout))
		err = methodReq.Reply(Method_USER_PASS)
		s.conn.SetReadDeadline(time.Time{})
		if err != nil {
			err = fmt.Errorf("reply answer data fail,ERR: %s", err)
			return
		}
		//read auth
		buf := make([]byte, 500)
		var n int
		s.conn.SetReadDeadline(time.Now().Add(s.timeout))
		n, err = s.conn.Read(buf)
		s.conn.SetReadDeadline(time.Time{})
		if err != nil {
			err = fmt.Errorf("read auth info fail,ERR: %s", err)
			return
		}
		r := buf[:n]
		s.pAuth.User = string(r[2 : r[1]+2])
		s.pAuth.Password = string(r[2+r[1]+1:])
		//auth
		_userAddr := strings.Split(remoteAddr.String(), ":")
		_localAddr := strings.Split(localAddr.String(), ":")
		if s.basicAuthCenter == nil || s.basicAuthCenter.Verify(basicAuth.Format(s.user, s.password), _userAddr[0], _localAddr[0], "") {
			s.conn.SetDeadline(time.Now().Add(s.timeout))
			_, err = s.conn.Write([]byte{0x01, 0x00})
			s.conn.SetDeadline(time.Time{})
			if err != nil {
				err = fmt.Errorf("answer auth success to %s fail,ERR: %s", remoteAddr, err)
				return
			}
		} else {
			s.conn.SetDeadline(time.Now().Add(s.timeout))
			_, err = s.conn.Write([]byte{0x01, 0x01})
			s.conn.SetDeadline(time.Time{})
			if err != nil {
				err = fmt.Errorf("answer auth fail to %s fail,ERR: %s", remoteAddr, err)
				return
			}
			err = fmt.Errorf("auth fail from %s", remoteAddr)
			return
		}
	}
	//request detail
	s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	request, e := NewRequest(s.conn)
	s.conn.SetReadDeadline(time.Time{})
	if e != nil {
		err = fmt.Errorf("read request data fail,ERR: %s", e)
		return
	}
	//协商结束

	switch request.CMD() {
	case CMD_BIND:
		err = request.TCPReply(REP_UNKNOWN)
		if err != nil {
			err = fmt.Errorf("TCPReply REP_UNKNOWN to %s fail,ERR: %s", remoteAddr, err)
			return
		}
		err = fmt.Errorf("cmd bind not supported, form: %s", remoteAddr)
		return
	case CMD_CONNECT:
		err = request.TCPReply(REP_SUCCESS)
		if err != nil {
			err = fmt.Errorf("TCPReply REP_SUCCESS to %s fail,ERR: %s", remoteAddr, err)
			return
		}
	case CMD_ASSOCIATE:
		if !s.enableUDP {
			err = request.UDPReply(REP_UNKNOWN, "0.0.0.0:0")
			if err != nil {
				err = fmt.Errorf("UDPReply REP_UNKNOWN to %s fail,ERR: %s", remoteAddr, err)
				return
			}
			err = fmt.Errorf("cmd associate not supported, form: %s", remoteAddr)
			return
		}
		a, _ := net.ResolveUDPAddr("udp", ":0")
		s.UDPConnListener, err = net.ListenUDP("udp", a)
		if err != nil {
			request.UDPReply(REP_UNKNOWN, "0.0.0.0:0")
			err = fmt.Errorf("udp bind fail,ERR: %s , for %s", err, remoteAddr)
			return
		}
		_, port, _ := net.SplitHostPort(s.UDPConnListener.LocalAddr().String())
		err = request.UDPReply(REP_SUCCESS, net.JoinHostPort(s.udpIP, port))
		if err != nil {
			err = fmt.Errorf("UDPReply REP_SUCCESS to %s fail,ERR: %s", remoteAddr, err)
			return
		}

	}

	//fill socks info
	s.target = request.Addr()
	s.methodsCount = methodReq.MethodsCount()
	s.methods = methodReq.Methods()
	s.cmd = request.CMD()
	s.reserve = request.reserve
	s.addressType = request.addressType
	s.dstAddr = request.dstAddr
	s.dstHost = request.dstHost
	s.dstPort = request.dstPort
	return
}
