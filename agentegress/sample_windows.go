// Copyright 2026 The casbin Authors. All Rights Reserved.
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

//go:build windows

package agentegress

import (
	"net/netip"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var (
	iphlpapi                 = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable  = iphlpapi.NewProc("GetExtendedTcpTable")
	procSetPerTcpConnEStats  = iphlpapi.NewProc("SetPerTcpConnectionEStats")
	procGetPerTcpConnEStats  = iphlpapi.NewProc("GetPerTcpConnectionEStats")
	procSetPerTcp6ConnEStats = iphlpapi.NewProc("SetPerTcp6ConnectionEStats")
	procGetPerTcp6ConnEStats = iphlpapi.NewProc("GetPerTcp6ConnectionEStats")
	dnsapi                   = windows.NewLazySystemDLL("dnsapi.dll")
	procDnsGetCacheDataTable = dnsapi.NewProc("DnsGetCacheDataTable")
	procDnsFree              = dnsapi.NewProc("DnsFree")
)

const (
	afInet              = 2
	afInet6             = 23
	tcpTableOwnerPidAll = 5
	stateListen         = 2
	stateEstablished    = 5
	estatsData          = 1
	dnsTypeA            = 1
	dnsTypeCname        = 5
	dnsTypeAAAA         = 28
	// DNS_QUERY_NO_WIRE_QUERY plus the undocumented bit ipconfig /displaydns
	// passes. Without it the cache hides what getaddrinfo put there, which is
	// every name Node and Electron resolve.
	dnsQueryCacheOnly = 0x8010
)

// MIB_TCPROW and MIB_TCP6ROW: what the EStats calls take.
type tcpRow struct{ State, LocalAddr, LocalPort, RemoteAddr, RemotePort uint32 }

type tcp6Row struct {
	State         uint32
	LocalAddr     [16]byte
	LocalScopeId  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeId uint32
	RemotePort    uint32
}

type tcpRowOwnerPid struct {
	tcpRow
	Pid uint32
}

type tcp6RowOwnerPid struct {
	LocalAddr     [16]byte
	LocalScopeId  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeId uint32
	RemotePort    uint32
	State         uint32
	Pid           uint32
}

// TCP_ESTATS_DATA_ROD_v0; the call rejects any other size.
type estatsDataRod struct {
	DataBytesOut, DataSegsOut, DataBytesIn, DataSegsIn, SegsOut, SegsIn uint64
	Rest                                                                [48]byte
}

type dnsCacheEntry struct {
	Next   *dnsCacheEntry
	Name   *uint16
	Type   uint16
	Length uint16
	Flags  uint32
}

func port(raw uint32) uint16 { return uint16(raw>>8&0xff | raw&0xff<<8) }

func tcpTable(family uintptr) []byte {
	var size uint32
	procGetExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, family, tcpTableOwnerPidAll, 0)
	for i := 0; i < 4; i++ {
		size += 8192
		buf := make([]byte, size)
		r, _, _ := procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, family, tcpTableOwnerPidAll, 0)
		if r == 0 {
			return buf
		}
	}
	return nil
}

// listSockets returns the established connections of the given processes, and
// which process listens on each port, so a loopback peer can be named.
func listSockets(pids map[int]string) ([]socket, map[uint16]int) {
	var sockets []socket
	listeners := map[uint16]int{}

	if buf := tcpTable(afInet); buf != nil {
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		for _, row := range unsafe.Slice((*tcpRowOwnerPid)(unsafe.Pointer(&buf[4])), count) {
			if row.State == stateListen {
				listeners[port(row.LocalPort)] = int(row.Pid)
				continue
			}
			if _, ok := pids[int(row.Pid)]; !ok || row.State != stateEstablished {
				continue
			}
			sockets = append(sockets, socket{
				pid:    int(row.Pid),
				local:  netip.AddrPortFrom(ipv4(row.LocalAddr), port(row.LocalPort)),
				remote: netip.AddrPortFrom(ipv4(row.RemoteAddr), port(row.RemotePort)),
				row:    row.tcpRow,
			})
		}
	}

	if buf := tcpTable(afInet6); buf != nil {
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		for _, row := range unsafe.Slice((*tcp6RowOwnerPid)(unsafe.Pointer(&buf[4])), count) {
			if row.State == stateListen {
				if _, taken := listeners[port(row.LocalPort)]; !taken {
					listeners[port(row.LocalPort)] = int(row.Pid)
				}
				continue
			}
			if _, ok := pids[int(row.Pid)]; !ok || row.State != stateEstablished {
				continue
			}
			sockets = append(sockets, socket{
				pid:    int(row.Pid),
				local:  netip.AddrPortFrom(netip.AddrFrom16(row.LocalAddr).Unmap(), port(row.LocalPort)),
				remote: netip.AddrPortFrom(netip.AddrFrom16(row.RemoteAddr).Unmap(), port(row.RemotePort)),
				row: tcp6Row{
					State: row.State, LocalAddr: row.LocalAddr, LocalScopeId: row.LocalScopeId, LocalPort: row.LocalPort,
					RemoteAddr: row.RemoteAddr, RemoteScopeId: row.RemoteScopeId, RemotePort: row.RemotePort,
				},
			})
		}
	}
	return sockets, listeners
}

func ipv4(raw uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(raw), byte(raw >> 8), byte(raw >> 16), byte(raw >> 24)})
}

// startCounting turns on the connection's byte counters, which only an
// administrator may do. They count from here on.
func startCounting(sock *socket) bool {
	enable := [1]byte{1}
	var r uintptr
	switch row := sock.row.(type) {
	case tcpRow:
		r, _, _ = procSetPerTcpConnEStats.Call(uintptr(unsafe.Pointer(&row)), estatsData, uintptr(unsafe.Pointer(&enable[0])), 0, 1, 0)
	case tcp6Row:
		r, _, _ = procSetPerTcp6ConnEStats.Call(uintptr(unsafe.Pointer(&row)), estatsData, uintptr(unsafe.Pointer(&enable[0])), 0, 1, 0)
	default:
		return false
	}
	return r == 0
}

func readCounts(sock *socket) (uint64, uint64, bool) {
	var rod estatsDataRod
	var r uintptr
	switch row := sock.row.(type) {
	case tcpRow:
		r, _, _ = procGetPerTcpConnEStats.Call(uintptr(unsafe.Pointer(&row)), estatsData, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&rod)), 0, unsafe.Sizeof(rod))
	case tcp6Row:
		r, _, _ = procGetPerTcp6ConnEStats.Call(uintptr(unsafe.Pointer(&row)), estatsData, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&rod)), 0, unsafe.Sizeof(rod))
	default:
		return 0, 0, false
	}
	return rod.DataBytesOut, rod.DataBytesIn, r == 0
}

func elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// cachedNames maps each address in the resolver cache to the name a program
// asked for. An alias beats the name it points at: bucket.oss-cn-beijing.
// aliyuncs.com says more than the CDN node behind it.
func cachedNames() map[netip.Addr]string {
	result := map[netip.Addr]string{}
	var head *dnsCacheEntry
	if ok, _, _ := procDnsGetCacheDataTable.Call(uintptr(unsafe.Pointer(&head))); ok == 0 {
		return result
	}

	alias := map[netip.Addr]bool{}
	for entry := head; entry != nil; {
		next := entry.Next
		if entry.Type == dnsTypeA || entry.Type == dnsTypeAAAA {
			name := windows.UTF16PtrToString(entry.Name)
			addrs, viaCname := cachedAddrs(name, entry.Type)
			for _, addr := range addrs {
				if _, ok := result[addr]; !ok || (viaCname && !alias[addr]) {
					result[addr] = name
					alias[addr] = viaCname
				}
			}
		}
		procDnsFree.Call(uintptr(unsafe.Pointer(entry.Name)), 0)
		procDnsFree.Call(uintptr(unsafe.Pointer(entry)), 0)
		entry = next
	}
	return result
}

func cachedAddrs(name string, kind uint16) ([]netip.Addr, bool) {
	var records *windows.DNSRecord
	if windows.DnsQuery(name, kind, dnsQueryCacheOnly, nil, &records, nil) != nil {
		return nil, false
	}
	defer windows.DnsRecordListFree(records, 1)

	var addrs []netip.Addr
	viaCname := false
	for record := records; record != nil; record = record.Next {
		switch record.Type {
		case dnsTypeCname:
			viaCname = true
		case dnsTypeA:
			addrs = append(addrs, netip.AddrFrom4([4]byte(record.Data[:4])))
		case dnsTypeAAAA:
			addrs = append(addrs, netip.AddrFrom16([16]byte(record.Data[:16])).Unmap())
		}
	}
	return addrs, viaCname
}

func processName(pid int) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(handle, 0, &buf[0], &size) != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}
