package internal

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Get the local ip address
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("core:network: %s", err)
	}

	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("core:network: unable to obtain an ip")
}

// Get a random open port
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Try to find the address of a service by port number
func GetServiceAddress(port int, protocol string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// Check all network interfaces
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			// fmt.Println(err)
			continue
		}

		// handle err
		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			for {
				nextIp := ip.String()
				serviceIp := net.JoinHostPort(nextIp, strconv.Itoa(port))

				conn, err := net.DialTimeout(protocol, serviceIp, time.Second)
				if err != nil {
					return "", err
				}
				if conn != nil {
					conn.Close()
					return nextIp, nil
				}

				nextIp, err = incrementIP(ip.String(), addr.String())
				if err != nil {
					return "", err
				}
			}
		}
	}

	return "", nil
}

func incrementIP(address string, netRange string) (string, error) {
	ip := net.ParseIP(address)
	_, subnet, err := net.ParseCIDR(netRange)
	if err != nil {
		return "", err
	}
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}

	if !subnet.Contains(ip) {
		return "", fmt.Errorf("overflowed CIDR while incrementing IP")
	}

	return ip.String(), nil
}
