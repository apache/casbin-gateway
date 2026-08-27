// Copyright 2023 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"errors"
	"net"
	"os"
)

var hostname = ""

func init() {
	name, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	hostname = name
}

func GetHostname() string {
	return hostname
}

func IsIntranetIp(ip string) bool {
	ipStr, _, err := net.SplitHostPort(ip)
	if err != nil {
		ipStr = ip
	}

	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return false
	}

	return parsedIP.IsPrivate() ||
		parsedIP.IsLoopback() ||
		parsedIP.IsLinkLocalUnicast() ||
		parsedIP.IsLinkLocalMulticast()
}

// LanIPv4 is the address this host answers at on its own network, which is what
// a sandbox reaches Gateway at: loopback inside one is the sandbox itself. It is
// read from the route to a public address, so the interface a sandbox is
// bridged onto wins over the virtual ones a hypervisor adds. Nothing is sent —
// a UDP socket is never connected.
func LanIPv4() (string, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:53")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ip := addr.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
				return ip.String(), nil
			}
		}
	}
	return firstPrivateIPv4()
}

// firstPrivateIPv4 is the fallback for a host with no route to the internet.
func firstPrivateIPv4() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, item := range interfaces {
		if item.Flags&net.FlagUp == 0 || item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := item.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			network, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip := network.IP.To4(); ip != nil && ip.IsPrivate() {
				return ip.String(), nil
			}
		}
	}
	return "", errors.New("this host has no address of its own on any network, so an agent that runs in a sandbox cannot reach Gateway")
}
