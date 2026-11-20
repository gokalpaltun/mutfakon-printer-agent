// Network printers: a thermal printer reached by IP (raw TCP, port 9100) with no Windows driver. They are
// remembered in a small file next to the agent's other settings and listed with the Windows printers under the
// name "tcp://IP:PORT", so the rest of the flow (choose, test, print) treats them like any other printer.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const netPrefix = "tcp://"

var netMu sync.Mutex

func netFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "MutfakOn", "network-printers.json"), nil
}

func loadNetPrinters() []string {
	path, err := netFile()
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var names []string
	_ = json.Unmarshal(raw, &names)
	return names
}

func saveNetPrinters(names []string) error {
	path, err := netFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, _ := json.Marshal(names)
	return os.WriteFile(path, raw, 0o644)
}

// parseNetName turns "tcp://192.168.1.48:9100" into its dial address.
func parseNetName(name string) (string, bool) {
	if !strings.HasPrefix(name, netPrefix) {
		return "", false
	}
	return strings.TrimPrefix(name, netPrefix), true
}

// Only addresses on the local network: this agent must never become a way to reach the internet.
func privateIPv4(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func dialPrinter(addr string) (net.Conn, error) {
	return net.DialTimeout("tcp", addr, 3*time.Second)
}

func addNetPrinter(host string, port int) (string, error) {
	host = strings.TrimSpace(host)
	if !privateIPv4(host) {
		return "", fmt.Errorf("INVALID_ADDRESS")
	}
	if port == 0 {
		port = 9100
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("INVALID_ADDRESS")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := dialPrinter(addr)
	if err != nil {
		return "", fmt.Errorf("UNREACHABLE")
	}
	conn.Close()
	name := netPrefix + addr
	netMu.Lock()
	defer netMu.Unlock()
	names := loadNetPrinters()
	for _, n := range names {
		if n == name {
			return name, nil
		}
	}
	return name, saveNetPrinters(append(names, name))
}

func removeNetPrinter(name string) error {
	netMu.Lock()
	defer netMu.Unlock()
	kept := []string{}
	for _, n := range loadNetPrinters() {
		if n != name {
			kept = append(kept, n)
		}
	}
	return saveNetPrinters(kept)
}

func printNetwork(addr string, data []byte) error {
	conn, err := dialPrinter(addr)
	if err != nil {
		return fmt.Errorf("printer %s unreachable: %v", addr, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	_, err = conn.Write(data)
	return err
}

// allPrinters: the system's printers plus the network ones.
func allPrinters() ([]printerInfo, error) {
	list, err := listPrinters()
	if err != nil {
		return nil, err
	}
	for _, name := range loadNetPrinters() {
		addr, _ := parseNetName(name)
		port := "Network"
		list = append(list, printerInfo{Name: name, Port: &port, Driver: &addr})
	}
	return list, nil
}

// printTo sends raw bytes to a network printer by name, otherwise to the system printer.
func printTo(printer string, data []byte) error {
	if addr, ok := parseNetName(printer); ok {
		return printNetwork(addr, data)
	}
	return printRaw(printer, data)
}

type foundPrinter struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Raw-print ports: 9100 is the standard; 9101-9103 are the extra ports of multi-port print servers.
var rawPorts = []int{9100, 9101, 9102, 9103}

// discoverNetPrinters probes every address of the machine's local /24 networks for an open raw-print port.
// Thermal receipt printers listen there; hosts that are not printers simply refuse the connection.
func discoverNetPrinters() []foundPrinter {
	var targets []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || !privateIPv4(ipn.IP.String()) {
				continue
			}
			base := ipn.IP.To4()
			for i := 1; i < 255; i++ {
				ip := net.IPv4(base[0], base[1], base[2], byte(i)).String()
				if ip != ipn.IP.String() {
					targets = append(targets, ip)
				}
			}
		}
	}
	known := map[string]bool{}
	for _, n := range loadNetPrinters() {
		known[n] = true
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 256)
	found := []foundPrinter{}
	for _, ip := range targets {
		for _, port := range rawPorts {
			addr := net.JoinHostPort(ip, strconv.Itoa(port))
			if known[netPrefix+addr] {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(ip string, port int, addr string) {
				defer wg.Done()
				defer func() { <-sem }()
				conn, err := net.DialTimeout("tcp", addr, 600*time.Millisecond)
				if err != nil {
					return
				}
				conn.Close()
				mu.Lock()
				found = append(found, foundPrinter{Host: ip, Port: port})
				mu.Unlock()
			}(ip, port, addr)
		}
	}
	wg.Wait()
	return found
}
