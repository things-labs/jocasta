package sps

import (
	"crypto/md5"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/thinkgos/jocasta/core/socks5"
	"github.com/thinkgos/jocasta/lib/outil"
	"github.com/thinkgos/jocasta/pkg/sword"
)

func (sf *SPS) proxyUDP(inConn net.Conn, serverConn *socks5.Server) {
	defer func() {
		if e := recover(); e != nil {
			sf.log.DPanicf("udp local->out io copy crashed:\n%s\n%s", e, string(debug.Stack()))
		}
	}()
	if sf.cfg.ParentType == "ssh" {
		inConn.Close()
		return
	}
	srcIP, _, _ := net.SplitHostPort(inConn.RemoteAddr().String())
	inconnRemoteAddr := inConn.RemoteAddr().String()
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	udpListener := serverConn.UDPConnListener
	sf.log.Infof("proxy udp on %s , for %s", udpListener.LocalAddr(), inconnRemoteAddr)
	sf.userConns.Set(inconnRemoteAddr, inConn)
	var (
		outUDPConn       *net.UDPConn
		outconn          net.Conn
		outconnLocalAddr string
		isClosedErr      = func(err error) bool {
			return err != nil && strings.Contains(err.Error(), "use of closed network connection")
		}
		destAddr *net.UDPAddr
	)
	var clean = func(msg, err string) {
		raddr := ""
		if outUDPConn != nil {
			raddr = outUDPConn.RemoteAddr().String()
			outUDPConn.Close()
		}
		if msg != "" {
			if raddr != "" {
				sf.log.Errorf("%s , %s , %s -> %s", msg, err, inconnRemoteAddr, raddr)
			} else {
				sf.log.Infof("%s , %s , from : %s", msg, err, inconnRemoteAddr)
			}
		}
		inConn.Close()
		udpListener.Close()
		sf.userConns.Remove(inconnRemoteAddr)
		if outconn != nil {
			outconn.Close()
		}
		if outconnLocalAddr != "" {
			sf.userConns.Remove(outconnLocalAddr)
		}
	}
	defer clean("", "")
	go func() {
		defer func() {
			if e := recover(); e != nil {
				sf.log.DPanicf("udp related client tcp conn read crashed:\n%s\n%s", e, string(debug.Stack()))
			}
		}()
		buf := make([]byte, 1)
		inConn.SetReadDeadline(time.Time{})
		if _, err := inConn.Read(buf); err != nil {
			clean("udp related tcp conn disconnected with read", err.Error())
		}
	}()
	go func() {
		defer func() {
			if e := recover(); e != nil {
				sf.log.DPanicf("udp related client tcp conn write crashed:\n%s\n%s", e, string(debug.Stack()))
			}
		}()
		for {
			inConn.SetWriteDeadline(time.Now().Add(time.Second * 5))
			if _, err := inConn.Write([]byte{0x00}); err != nil {
				clean("udp related tcp conn disconnected with write", err.Error())
				return
			}
			inConn.SetWriteDeadline(time.Time{})
			time.Sleep(time.Second * 5)
		}
	}()
	//parent proxy
	lbAddr := sf.lb.Select(inConn.RemoteAddr().String(), sf.cfg.LoadBalanceOnlyHA)

	outconn, err := sf.dialParent(lbAddr)
	//outconn, err := s.dialParent(nil, nil, "", false)
	if err != nil {
		clean("connnect fail", fmt.Sprintf("%s", err))
		return
	}
	sf.log.Infof("connect %s for udp", serverConn.Target())
	//socks client

	client, err := sf.HandshakeSocksParent(sf.getParentAuth(lbAddr), outconn, "udp", serverConn.Target(), serverConn.AuthData(), false)
	if err != nil {
		clean("handshake fail", fmt.Sprintf("%s", err))
		return
	}

	//outconnRemoteAddr := outconn.RemoteAddr().String()
	outconnLocalAddr = outconn.LocalAddr().String()
	sf.userConns.Set(outconnLocalAddr, &outconn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				sf.log.DPanicf("udp related parent tcp conn read crashed:\n%s\n%s", e, string(debug.Stack()))
			}
		}()
		buf := make([]byte, 1)
		outconn.SetReadDeadline(time.Time{})
		if _, err := outconn.Read(buf); err != nil {
			clean("udp parent tcp conn disconnected", fmt.Sprintf("%s", err))
		}
	}()
	//forward to parent udp
	//s.log.Printf("parent udp address %s", client.UDPAddr)
	destAddr, _ = net.ResolveUDPAddr("udp", client.UDPAddr)
	//relay
	buf := sword.Binding.Get()
	defer sword.Binding.Put(buf)
	for {
		n, srcAddr, err := udpListener.ReadFromUDP(buf[:cap(buf)])
		if err != nil {
			sf.log.Errorf("udp listener read fail, %s", err.Error())
			if isClosedErr(err) {
				return
			}
			continue
		}
		srcIP0, _, _ := net.SplitHostPort(srcAddr.String())
		//IP not match drop it
		if srcIP != srcIP0 {
			continue
		}
		p := socks5.NewPacketUDP()
		//convert data to raw
		if len(sf.udpLocalKey) > 0 {
			var v []byte
			v, err = outil.DecryptCFB(sf.udpLocalKey, buf[:n])
			if err == nil {
				err = p.Parse(v)
			}
		} else {
			err = p.Parse(buf[:n])
		}
		//err = p.Parse(buf[:n])
		if err != nil {
			sf.log.Errorf("udp listener parse packet fail, %s", err.Error())
			continue
		}
		if v, ok := sf.udpRelatedPacketConns.Get(srcAddr.String()); !ok {
			outUDPConn, err = net.DialUDP("udp", localAddr, destAddr)
			if err != nil {
				sf.log.Errorf("create out udp conn fail , %s , from : %s", err, srcAddr)
				continue
			}
			sf.udpRelatedPacketConns.Set(srcAddr.String(), outUDPConn)
			sword.Submit(func() {
				defer func() {
					sf.udpRelatedPacketConns.Remove(srcAddr.String())
				}()
				sword.Binding.RunUDPCopy(udpListener, outUDPConn, srcAddr, 0, func(data []byte) []byte {
					//forward to local
					var v []byte
					//convert parent data to raw
					if len(sf.udpParentKey) > 0 {
						v, err = outil.DecryptCFB(sf.udpParentKey, data)
						if err != nil {
							sf.log.Errorf("udp outconn parse packet fail, %s", err.Error())
							return []byte{}
						}
					} else {
						v = data
					}
					//now v is raw, try convert v to local
					if len(sf.udpLocalKey) > 0 {
						v, _ = outil.EncryptCFB(sf.udpLocalKey, v)
					}
					return v
				})
			})

		} else {
			outUDPConn = v.(*net.UDPConn)
		}
		//local->out io copy
		//forward to parent
		//p is raw, now convert it to parent
		var v []byte
		if len(sf.udpParentKey) > 0 {
			v, _ = outil.EncryptCFB(sf.udpParentKey, p.Bytes())
		} else {
			v = p.Bytes()
		}
		_, err = outUDPConn.Write(v)
		// _, err = outUDPConn.Write(p.Bytes())
		if err != nil {
			if isClosedErr(err) {
				return
			}
			sf.log.Errorf("send out udp data fail , %s , from : %s", err, srcAddr)
			continue
		} else {
			//s.log.Printf("send udp data to remote success , len %d, for : %s", len(p.Data()), srcAddr)
		}
	}

}

func (sf *SPS) ParentUDPKey() (key []byte) {
	switch sf.cfg.ParentType {
	case "tcp":
		if sf.cfg.ParentKey != "" {
			v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.ParentKey)))
			return []byte(v)[:24]
		}
	case "tls":
		if sf.cfg.key != nil {
			return sf.cfg.key[:24]
		}
	case "kcp":
		v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.SKCPConfig.Key)))
		return []byte(v)[:24]
	}
	return
}
func (sf *SPS) LocalUDPKey() (key []byte) {
	switch sf.cfg.LocalType {
	case "tcp":
		if sf.cfg.LocalKey != "" {
			v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.LocalKey)))
			return []byte(v)[:24]
		}
	case "tls":
		return sf.cfg.key[:24]
	case "kcp":
		v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.SKCPConfig.Key)))
		return []byte(v)[:24]
	}
	return
}
