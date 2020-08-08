package sword

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

type systemNetworkIPs struct {
	mux sync.Mutex
	ips atomic.Value
}

var snIPs systemNetworkIPs

// SystemNetworkIPs system network ip slice
func SystemNetworkIPs() ([]net.IP, error) {
	if v := snIPs.ips.Load(); v != nil {
		return v.([]net.IP), nil
	}

	snIPs.mux.Lock()
	defer snIPs.mux.Unlock()

	// may be block before read again to check value my store by other goroutine
	if v := snIPs.ips.Load(); v != nil {
		return v.([]net.IP), nil
	}

	// get a list of the system's network interfaces.
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ips := []net.IP{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 { // interface is down,ignore
			continue
		}
		// if iface.Flags&net.FlagLoopback != 0 {  // interface is a loopback interface
		// 	continue // loopback interface
		// }

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// if ip.IsLoopback() {
			// 	continue
			// }
			if ip.To4() != nil { // should an ipv4 address
				ips = append(ips, ip)
			}
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no address Found, net.Interface : %v", ifaces)
	}
	snIPs.ips.Store(ips)
	return ips, nil
}
