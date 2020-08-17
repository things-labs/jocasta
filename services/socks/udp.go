package socks

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	sockv5 "github.com/thinkgos/go-socks5"
	"github.com/thinkgos/go-socks5/statute"
	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/core/socks5"
	"github.com/thinkgos/jocasta/lib/extnet"
	"github.com/thinkgos/jocasta/lib/outil"
	"github.com/thinkgos/jocasta/pkg/sword"
)

func (sf *Socks) proxyUDP(ctx context.Context, writer io.Writer, request *sockv5.Request) error {
	if sf.cfg.ParentType == "ssh" {
		return errors.New("ssh not support udp")
	}

	useProxy := sf.isUseProxy(request.DestAddr.String())

	outConn, targetUDP, err := sf.dialForUdp(ctx, useProxy, request)
	if err != nil {
		msg := err.Error()
		resp := statute.RepHostUnreachable
		if strings.Contains(msg, "refused") {
			resp = statute.RepConnectionRefused
		} else if strings.Contains(msg, "network is unreachable") {
			resp = statute.RepNetworkUnreachable
		}
		if err := sockv5.SendReply(writer, resp, nil); err != nil {
			return fmt.Errorf("failed to send reply, %v", err)
		}
		return fmt.Errorf("connect to %v failed, %v", request.RawDestAddr, err)
	}
	sf.userConns.Set(outConn.LocalAddr().String(), outConn)
	defer func() {
		if outConn != nil {
			sf.userConns.Remove(outConn.LocalAddr().String())
			outConn.Close()
		}
		targetUDP.Close()
	}()
	if outConn != nil {
		sf.userConns.Set(outConn.LocalAddr().String(), outConn)
		go func() {
			buf := make([]byte, 1)
			if _, err := outConn.Read(buf); err != nil {
			}
		}()
	}

	bindLn, err := net.ListenUDP("udp", nil)
	if err != nil {
		if err := sockv5.SendReply(writer, statute.RepServerFailure, nil); err != nil {
			return fmt.Errorf("failed to send reply, %v", err)
		}
		return fmt.Errorf("listen udp failed, %v", err)
	}

	// send BND.ADDR and BND.PORT, client must
	if err = sockv5.SendReply(writer, statute.RepSuccess, bindLn.LocalAddr()); err != nil {
		return fmt.Errorf("failed to send reply, %v", err)
	}

	srcAddr := request.RemoteAddr.String()
	targetAddr := targetUDP.RemoteAddr().String()

	sf.log.Infof("proxy udp on %s , for src %s", bindLn.LocalAddr(), srcAddr)
	sf.userConns.Set(srcAddr, writer)
	defer func() {
		sf.userConns.Remove(srcAddr)
		bindLn.Close()
	}()

	go func() {
		srcIP, _, _ := net.SplitHostPort(srcAddr)
		// read from client and write to remote server
		buf := sword.Binding.Get()
		defer func() {
			targetUDP.Close()
			bindLn.Close()
			sword.Binding.Put(buf)
		}()
		for {
			n, srcAddr, err := bindLn.ReadFrom(buf[:cap(buf)])
			if err != nil {
				sf.log.Errorf("udp listener read fail, %s", err.Error())
				if extnet.IsErrClosed(err) {
					return
				}
				continue
			}

			// IP not match drop it
			if srcIP0, _, _ := net.SplitHostPort(srcAddr.String()); srcIP0 != srcIP {
				continue
			}

			rawData, err := sf.localData2Raw(buf[:n])
			if err != nil {
				continue
			}

			pk, err := statute.ParseDatagram(rawData)
			if err != nil {
				sf.log.Errorf("udp listener parse packet fail, %s", err.Error())
				continue
			}

			if ok := sf.udpRelatedPacketConns.SetIfAbsent(srcAddr.String(), targetUDP); !ok {
				go func() {
					// out->local io copy
					buf := sword.Binding.Get()
					defer func() {
						targetUDP.Close()
						bindLn.Close()
						sf.udpRelatedPacketConns.Remove(srcAddr.String())
						sword.Binding.Put(buf)
					}()
					for {
						n, err := targetUDP.Read(buf[:cap(buf)])
						if err != nil {
							sf.log.Warnf("read out udp data fail , %s , from : %s", err, srcAddr)
							if extnet.IsErrClosed(err) {
								return
							}
							continue
						}

						var rawData []byte
						if useProxy {
							// forward to local, convert parent data to raw
							rawData, err = sf.parentData2Raw(buf[:n])
							if err != nil {
								continue
							}
						} else {
							rp, err := statute.NewDatagram(targetAddr, buf[:n])
							if err != nil {
								continue
							}
							rawData = append(rp.Header(), rp.Data...)
						}
						lData, err := sf.raw2LocalData(rawData)
						if err != nil {
							continue
						}
						_, err = bindLn.WriteTo(lData, srcAddr)
						if err != nil {
							sf.log.Errorf("write out data to local fail , %s , from : %s", err, srcAddr)
							if extnet.IsErrClosed(err) {
								return
							}
							continue
						}
					}
				}()
			}

			// local -> out io copy
			outData := pk.Data // user data
			if useProxy {      // forward to parent, convert raw to parent data
				if outData, err = sf.raw2ParentData(rawData); err != nil {
					continue
				}
			}
			_, err = targetUDP.Write(outData)
			if err != nil {
				sf.log.Errorf("send out udp data fail , %s , from : %s", err, srcAddr)
				if extnet.IsErrClosed(err) {
					return
				}
				continue
			}
		}
	}()

	go func() {
		for {
			if _, err := writer.Write([]byte{0x00}); err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
			}
			time.Sleep(time.Second * 5)
		}
	}()

	buf := make([]byte, 1)
	for {
		if _, err = request.Reader.Read(buf); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return err
			}
		}
	}
}

func (sf *Socks) dialForUdp(ctx context.Context, useProxy bool, request *sockv5.Request) (conn net.Conn, target *net.UDPConn, err error) {
	srcAddr := request.RemoteAddr.String()
	targetAddr := request.DestAddr.String()

	if useProxy {
		//parent proxy
		lbAddr := sf.lb.Select(srcAddr)
		conn, err = sf.dialParent(lbAddr)
		if err != nil {
			return nil, nil, err
		}
		var client *socks5.Client

		client, err = sf.HandshakeSocksParent(conn, "udp", targetAddr,
			proxy.Auth{
				User:     request.AuthContext.Payload["username"],
				Password: request.AuthContext.Payload["password"],
			}, false)
		if err != nil {
			return
		}
		//forward to parent udp
		targetAddr = client.UDPAddr
	}
	sf.log.Infof("use proxy %v : udp %s", useProxy, targetAddr)

	destAddr, _ := net.ResolveUDPAddr("udp", targetAddr)
	target, err = net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0}, destAddr)
	return
}

//convert data to raw
func (sf *Socks) localData2Raw(b []byte) ([]byte, error) {
	if len(sf.udpLocalKey) > 0 {
		return outil.DecryptCFB(sf.udpLocalKey, b)
	}
	return b, nil
}

func (sf *Socks) raw2LocalData(b []byte) ([]byte, error) {
	if len(sf.udpLocalKey) > 0 {
		return outil.EncryptCFB(sf.udpLocalKey, b)
	}
	return b, nil
}

func (sf *Socks) parentData2Raw(b []byte) ([]byte, error) {
	if len(sf.udpParentKey) > 0 {
		return outil.DecryptCFB(sf.udpParentKey, b)
	}
	return b, nil
}

func (sf *Socks) raw2ParentData(b []byte) ([]byte, error) {
	if len(sf.udpParentKey) > 0 {
		return outil.EncryptCFB(sf.udpParentKey, b)
	}
	return b, nil
}

func (sf *Socks) parentUDPKey() (key []byte) {
	switch sf.cfg.ParentType {
	case "tcp", "stcp":
		if sf.cfg.ParentKey != "" {
			v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.ParentKey)))
			return []byte(v)[:24]
		}
	case "tls":
		return sf.cfg.tcpTlsConfig.Key[:24]
	case "kcp":
		v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.SKCPConfig.Key)))
		return []byte(v)[:24]
	}
	return
}

func (sf *Socks) localUDPKey() (key []byte) {
	switch sf.cfg.LocalType {
	case "tcp", "stcp":
		if sf.cfg.LocalKey != "" {
			v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.LocalKey)))
			return []byte(v)[:24]
		}
	case "tls":
		return sf.cfg.tcpTlsConfig.Key[:24]
	case "kcp":
		v := fmt.Sprintf("%x", md5.Sum([]byte(sf.cfg.SKCPConfig.Key)))
		return []byte(v)[:24]
	}
	return
}

func (sf *Socks) HandshakeSocksParent(outConn net.Conn, network, dstAddr string, auth proxy.Auth, fromSS bool) (*socks5.Client, error) {
	realAuth := sf.proxyAuth(auth, fromSS)

	client := socks5.NewClient(outConn, network, dstAddr, sf.cfg.Timeout, realAuth, nil)
	err := client.Handshake()
	return client, err
}
