package sps

import (
	"bytes"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/thinkgos/go-core-package/extnet"
	"golang.org/x/net/proxy"

	"github.com/thinkgos/jocasta/core/socks5"
	"github.com/thinkgos/jocasta/pkg/outil"
	"github.com/thinkgos/jocasta/pkg/sword"
)

func (sf *SPS) RunSSUDP(addr string) (err error) {
	a, _ := net.ResolveUDPAddr("udp", addr)
	listener, err := net.ListenUDP("udp", a)
	if err != nil {
		sf.log.Errorf("ss udp bind error %s", err)
		return
	}
	sf.log.Infof("ss udp on %s", listener.LocalAddr())
	sf.udpRelatedPacketConns.Set(addr, listener)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				sf.log.DPanicf("udp local->out io copy crashed:\n%s\n%s", e, string(debug.Stack()))
			}
		}()
		buf := sword.Binding.Get()
		defer sword.Binding.Put(buf)
		for {
			n, srcAddr, err := listener.ReadFrom(buf[:cap(buf)])
			if err != nil {
				sf.log.Errorf("read from client error %s", err)
				if extnet.IsErrClosed(err) {
					return
				}
				continue
			}
			var (
				inconnRemoteAddr = srcAddr.String()
				outUDPConn       *net.UDPConn
				outconn          net.Conn
				outconnLocalAddr string
				destAddr         *net.UDPAddr
				clean            = func(msg, err string) {
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
					sf.userConns.Remove(inconnRemoteAddr)
					if outconn != nil {
						outconn.Close()
					}
					if outconnLocalAddr != "" {
						sf.userConns.Remove(outconnLocalAddr)
					}
				}
			)
			defer clean("", "")

			var data []byte

			data, err = sf.localCipher.Decrypt(buf[:n])
			if err != nil {
				return
			}
			raw := bytes.NewBuffer([]byte{0x00, 0x00, 0x00})
			raw.Write(data)
			socksPacket := socks5.NewPacketUDP()
			err = socksPacket.Parse(raw.Bytes())
			raw = nil
			if err != nil {
				sf.log.Errorf("udp parse error %s", err)
				return
			}

			if v, ok := sf.udpRelatedPacketConns.Get(inconnRemoteAddr); !ok {
				//socks client
				lbAddr := sf.lb.Select(inconnRemoteAddr)
				outconn, err := sf.dialParent(lbAddr)
				if err != nil {
					clean("connnect fail", fmt.Sprintf("%s", err))
					return
				}

				client, err := sf.HandshakeSocksParent(sf.getParentAuth(lbAddr), outconn, "udp", socksPacket.Addr(), proxy.Auth{}, true)
				if err != nil {
					clean("handshake fail", fmt.Sprintf("%s", err))
					return
				}

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
				destAddr, _ = net.ResolveUDPAddr("udp", client.UDPAddr)
				localZeroAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
				outUDPConn, err = net.DialUDP("udp", localZeroAddr, destAddr)
				if err != nil {
					sf.log.Errorf("create out udp conn fail , %s , from : %s", err, srcAddr)
					return
				}
				sf.udpRelatedPacketConns.Set(srcAddr.String(), outUDPConn)
				sword.Go(func() {
					defer func() {
						sf.udpRelatedPacketConns.Remove(srcAddr.String())
					}()
					sword.Binding.RunUDPCopy(listener, outUDPConn, srcAddr, time.Second*5, func(data []byte) []byte {
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
						out, _ := sf.localCipher.Encrypt(v[3:])
						return out

					})
				})

			} else {
				outUDPConn = v.(*net.UDPConn)
			}
			//forward to parent
			//p is raw, now convert it to parent
			var v []byte
			if len(sf.udpParentKey) > 0 {
				v, _ = outil.EncryptCFB(sf.udpParentKey, socksPacket.Bytes())
			} else {
				v = socksPacket.Bytes()
			}
			_, err = outUDPConn.Write(v)
			socksPacket = socks5.PacketUDP{}
			if err != nil {
				if extnet.IsErrClosed(err) {
					return
				}
				sf.log.Errorf("send out udp data fail , %s , from : %s", err, srcAddr)
			}
		}
	}()
	return
}
