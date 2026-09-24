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

// Package agentegress watches where each agent's own processes send data. The
// proxy only sees what an agent sends its model; an agent that packs up a
// repository and posts it to its vendor's storage goes around it, and this is
// where that shows.
package agentegress

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/beego/beego"
)

const (
	sampleInterval = 500 * time.Millisecond
	pidInterval    = 5 * time.Second
	// A burst is this much sent to one destination within burstWindow.
	burstBytes  = 5 << 20
	burstWindow = time.Minute
	// An episode ends, and is recorded, once its destination has been quiet
	// this long.
	quietAfter      = time.Minute
	maxDestinations = 200

	EventType          = "egress"
	ActionLargeUpload  = "large-upload"
	ActionObjectUpload = "object-storage"
)

// Destination is one place an agent's processes connected to since Gateway
// started watching.
type Destination struct {
	// Host is the name the agent resolved, empty when the DNS cache had none.
	Host    string `json:"host,omitempty"`
	Address string `json:"address"`
	Kind    string `json:"kind"`
	// Via names the local process a KindLocal connection was made to.
	Via         string    `json:"via,omitempty"`
	Connections int       `json:"connections"`
	BytesOut    int64     `json:"bytesOut"`
	BytesIn     int64     `json:"bytesIn"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	// Flagged means an episode is open against it right now.
	Flagged bool `json:"flagged,omitempty"`
}

// Report is what the watcher saw of one agent.
type Report struct {
	Supported bool `json:"supported"`
	// Counted means bytes are measured; without it only destinations are.
	Counted      bool          `json:"counted"`
	Since        time.Time     `json:"since"`
	Destinations []Destination `json:"destinations"`
}

// Detail is the body of an egress record.
type Detail struct {
	Host        string    `json:"host,omitempty"`
	Address     string    `json:"address"`
	Kind        string    `json:"kind"`
	Via         string    `json:"via,omitempty"`
	BytesOut    int64     `json:"bytesOut"`
	BytesIn     int64     `json:"bytesIn"`
	Connections int       `json:"connections"`
	Counted     bool      `json:"counted"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}

// socket is one established TCP connection of an agent process. row is the
// platform's own record of it, which its byte counters are read through.
type socket struct {
	pid           int
	local, remote netip.AddrPort
	row           any
}

func (s socket) key() string {
	return fmt.Sprintf("%d|%s|%s", s.pid, s.local, s.remote)
}

type sample struct {
	at    time.Time
	bytes int64
}

type episode struct {
	start, last time.Time
	bytesOut    int64
	bytesIn     int64
	conns       int
}

type destination struct {
	Destination
	window  []sample
	episode *episode
}

type conn struct {
	agentId  string
	dest     *destination
	sock     socket
	counting bool
	out, in  uint64
	seen     time.Time
}

type watcher struct {
	sync.Mutex
	since    time.Time
	counted  bool
	agents   map[string]map[string]*destination
	conns    map[string]*conn
	pids     map[int]string
	names    map[netip.Addr]string
	missed   map[netip.Addr]time.Time
	lookedUp time.Time
}

var state = &watcher{
	agents: map[string]map[string]*destination{},
	conns:  map[string]*conn{},
	pids:   map[int]string{},
	names:  map[netip.Addr]string{},
	missed: map[netip.Addr]time.Time{},
}

var stop context.CancelFunc

// Start watches until Stop. agentPids maps each live process of an agent to
// its agent id; it is read every few seconds.
func Start(agentPids func() map[int]string) {
	if !supported || stop != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	stop = cancel
	state.Lock()
	state.since = time.Now()
	state.counted = elevated()
	state.Unlock()

	go func() {
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		var pidsAt time.Time
		for {
			select {
			case <-ctx.Done():
				state.Lock()
				state.flush(time.Now(), true)
				state.Unlock()
				return
			case now := <-ticker.C:
				if now.Sub(pidsAt) >= pidInterval {
					pidsAt = now
					tick(func() {
						pids := agentPids()
						state.Lock()
						state.pids = pids
						state.Unlock()
					})
				}
				tick(func() { state.sample(now) })
			}
		}
	}()
}

// tick runs one step of the watch, which must not take Gateway down with it.
func tick(step func()) {
	defer func() {
		if err := recover(); err != nil {
			beego.Error("agent egress watch:", err)
		}
	}()
	step()
}

// Stop ends the watch and records every episode still open.
func Stop() {
	if stop != nil {
		stop()
		stop = nil
	}
}

// ReportOf is what the watcher has seen of one agent.
func ReportOf(agentId string) Report {
	state.Lock()
	defer state.Unlock()

	report := Report{Supported: supported, Counted: state.counted, Since: state.since, Destinations: []Destination{}}
	for _, dest := range state.agents[agentId] {
		item := dest.Destination
		item.Flagged = dest.episode != nil
		report.Destinations = append(report.Destinations, item)
	}
	sort.Slice(report.Destinations, func(i, j int) bool {
		a, b := report.Destinations[i], report.Destinations[j]
		if a.BytesOut != b.BytesOut {
			return a.BytesOut > b.BytesOut
		}
		return a.LastSeen.After(b.LastSeen)
	})
	return report
}

func (w *watcher) sample(now time.Time) {
	w.Lock()
	defer w.Unlock()
	if len(w.pids) == 0 {
		w.flush(now, false)
		return
	}

	sockets, listeners := listSockets(w.pids)
	w.resolve(sockets, now)
	self := os.Getpid()
	for i := range sockets {
		sock := sockets[i]
		agentId := w.pids[sock.pid]
		key := sock.key()
		c := w.conns[key]
		if c == nil {
			dest := w.destinationOf(agentId, sock, listeners, self)
			if dest == nil {
				continue
			}
			c = &conn{agentId: agentId, dest: dest, sock: sock}
			c.counting = w.counted && startCounting(&c.sock)
			w.conns[key] = c
			dest.Connections++
			if dest.FirstSeen.IsZero() {
				dest.FirstSeen = now
			}
			if dest.Kind == KindStorage && dest.episode == nil {
				dest.episode = &episode{start: now}
			}
			if dest.episode != nil {
				dest.episode.conns++
				dest.episode.last = now
			}
		}
		c.seen = now
		c.dest.LastSeen = now
		if c.dest.Host == "" && c.dest.Kind != KindLocal {
			if host := w.names[sock.remote.Addr()]; host != "" {
				c.dest.Host, c.dest.Kind = host, classify(host)
			}
		}
		if c.dest.episode != nil {
			c.dest.episode.last = now
		}
		if !c.counting {
			continue
		}
		out, in, ok := readCounts(&c.sock)
		if !ok {
			continue
		}
		deltaOut, deltaIn := int64(out-min(out, c.out)), int64(in-min(in, c.in))
		c.out, c.in = max(out, c.out), max(in, c.in)
		w.add(c.dest, now, deltaOut, deltaIn)
	}

	for key, c := range w.conns {
		if now.Sub(c.seen) > 2*sampleInterval {
			delete(w.conns, key)
		}
	}
	w.flush(now, false)
}

func (w *watcher) add(dest *destination, now time.Time, out, in int64) {
	if out == 0 && in == 0 {
		return
	}
	dest.BytesOut += out
	dest.BytesIn += in

	cut := 0
	for cut < len(dest.window) && now.Sub(dest.window[cut].at) > burstWindow {
		cut++
	}
	dest.window = append(dest.window[cut:], sample{at: now, bytes: out})

	if dest.episode == nil && dest.Kind != KindModel {
		var sum int64
		for _, s := range dest.window {
			sum += s.bytes
		}
		if sum >= burstBytes {
			dest.episode = &episode{start: dest.window[0].at, bytesOut: sum - out, conns: 1}
		}
	}
	if dest.episode != nil {
		dest.episode.bytesOut += out
		dest.episode.bytesIn += in
		dest.episode.last = now
	}
}

// flush records every episode whose destination has gone quiet, or all of them
// when the watch is ending.
func (w *watcher) flush(now time.Time, all bool) {
	for agentId, dests := range w.agents {
		for _, dest := range dests {
			ep := dest.episode
			if ep == nil || (!all && now.Sub(ep.last) < quietAfter) {
				continue
			}
			dest.episode = nil
			w.record(agentId, dest, ep)
		}
	}
}

func (w *watcher) record(agentId string, dest *destination, ep *episode) {
	action := ActionObjectUpload
	if ep.bytesOut >= burstBytes {
		action = ActionLargeUpload
	}
	detail := Detail{
		Host: dest.Host, Address: dest.Address, Kind: dest.Kind, Via: dest.Via,
		BytesOut: ep.bytesOut, BytesIn: ep.bytesIn, Connections: ep.conns, Counted: w.counted,
		Start: ep.start, End: ep.last,
	}
	body, _ := json.Marshal(detail)
	object := dest.Host
	if object == "" {
		object = dest.Address
	}
	title := fmt.Sprintf("Connected to object storage at %s", object)
	if action == ActionLargeUpload {
		title = fmt.Sprintf("Sent %.1f MB to %s", float64(ep.bytesOut)/(1<<20), object)
	}
	agentmonitor.AddRecord(&agentmonitor.Record{
		CreatedTime: ep.last.Format(time.RFC3339Nano),
		Agent:       agentId,
		EventType:   EventType,
		Action:      action,
		Object:      object,
		Title:       title,
		Detail:      string(body),
		DurationMs:  ep.last.Sub(ep.start).Milliseconds(),
	})
}

func (w *watcher) destinationOf(agentId string, sock socket, listeners map[uint16]int, self int) *destination {
	var host, kind, via, key string
	remote := sock.remote.Addr()
	if remote.IsLoopback() {
		owner, ok := listeners[sock.remote.Port()]
		if !ok || owner == self {
			return nil
		}
		// A process of any agent: its own helpers talking among themselves.
		if _, internal := w.pids[owner]; internal {
			return nil
		}
		kind, via = KindLocal, processName(owner)
		key = fmt.Sprintf("local|%d", sock.remote.Port())
	} else {
		host = w.names[remote]
		kind = classify(host)
		key = host
		if key == "" {
			key = remote.String()
		}
	}

	dests := w.agents[agentId]
	if dests == nil {
		dests = map[string]*destination{}
		w.agents[agentId] = dests
	}
	dest := dests[key]
	// A destination first seen before its name was known moves under the name.
	if byAddr := dests[remote.String()]; dest == nil && host != "" && byAddr != nil {
		delete(dests, remote.String())
		byAddr.Host, byAddr.Kind = host, kind
		dests[key], dest = byAddr, byAddr
	}
	if dest == nil {
		if len(dests) >= maxDestinations {
			evictOldest(dests)
		}
		dest = &destination{Destination: Destination{Host: host, Kind: kind, Via: via}}
		dests[key] = dest
	}
	dest.Address = sock.remote.String()
	return dest
}

func evictOldest(dests map[string]*destination) {
	oldestKey := ""
	var oldest time.Time
	for key, dest := range dests {
		if dest.episode != nil {
			continue
		}
		if oldestKey == "" || dest.LastSeen.Before(oldest) {
			oldestKey, oldest = key, dest.LastSeen
		}
	}
	if oldestKey != "" {
		delete(dests, oldestKey)
	}
}

// resolve names the remote addresses the DNS cache knows, going back to the
// cache for a new address at most once a second and giving up on one after a
// few tries: an agent that connects by address has no name to find.
func (w *watcher) resolve(sockets []socket, now time.Time) {
	var missing []netip.Addr
	for _, sock := range sockets {
		addr := sock.remote.Addr()
		if addr.IsLoopback() {
			continue
		}
		if _, ok := w.names[addr]; ok {
			continue
		}
		// A new connection starts the retries over: its name was just looked up.
		if _, known := w.conns[sock.key()]; !known {
			w.missed[addr] = now
		} else if first, ok := w.missed[addr]; ok && now.Sub(first) > 10*time.Second {
			continue
		}
		missing = append(missing, addr)
	}
	if len(missing) == 0 || now.Sub(w.lookedUp) < time.Second {
		return
	}
	w.lookedUp = now
	found := cachedNames()
	for addr, name := range found {
		w.names[addr] = name
	}
	for _, addr := range missing {
		if _, ok := found[addr]; ok {
			delete(w.missed, addr)
		}
	}
	if len(w.names) > 4096 {
		w.names = found
	}
	if len(w.missed) > 4096 {
		w.missed = map[netip.Addr]time.Time{}
	}
}
