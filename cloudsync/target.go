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

// Package cloudsync keeps a copy of Gateway's data somewhere that is not this
// machine. A target is a flat directory of named blobs and nothing more: that
// is all a snapshot needs, and it is the largest set of operations a synced
// folder, a WebDAV share and an S3 bucket all have in common. Supporting one
// more service - Dropbox, OneDrive or a NAS with an API of its own - is one
// file in this package that calls Register(), with nothing above it to change.
package cloudsync

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// File is one blob at the target. Snapshots are immutable and named after the
// moment they were taken, so the name is the whole identity and nothing is
// ever compared by content.
type File struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ModifiedTime time.Time `json:"modifiedTime"`
}

// Target is one place a copy of the data lives.
type Target interface {
	// Describe is where the copies land, in one line, for the page that says
	// where they went. It carries no credentials.
	Describe() string
	List(ctx context.Context) ([]File, error)
	Read(ctx context.Context, name string) ([]byte, error)
	Write(ctx context.Context, name string, data []byte) error
	Remove(ctx context.Context, name string) error
}

// Config is what the settings row stores: a kind, and the options that kind
// asks for. The options stay untyped on purpose - a new kind brings its own
// keys and nothing outside its own file has to learn them.
type Config struct {
	Kind    string            `json:"kind"`
	Options map[string]string `json:"options"`
	// HttpClient is handed in rather than built here, so every kind goes out
	// through Gateway's own transport, proxy setting and all.
	HttpClient *http.Client `json:"-"`
}

func (config Config) Option(name string) string {
	return strings.TrimSpace(config.Options[name])
}

func (config Config) OptionDefault(name string, fallback string) string {
	if value := config.Option(name); value != "" {
		return value
	}
	return fallback
}

// OptionBool reads a switch. An option nobody has set is neither true nor
// false, which is what the third return value is for: a kind that defaults to
// true has to tell "off" apart from "not answered".
func (config Config) OptionBool(name string) (value bool, set bool) {
	raw := strings.ToLower(config.Option(name))
	if raw == "" {
		return false, false
	}
	return raw == "true" || raw == "1" || raw == "yes", true
}

func (config Config) client() *http.Client {
	if config.HttpClient != nil {
		return config.HttpClient
	}
	return http.DefaultClient
}

// The kinds of thing a field holds. A secret is encrypted at rest and typed
// into a password box; the rest are plain.
const (
	FieldText   = "text"
	FieldSecret = "secret"
	FieldSwitch = "switch"
)

// Field is one thing a kind needs to be told, described well enough for the
// Settings page to draw the form without knowing the kind exists.
type Field struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Placeholder string `json:"placeholder"`
	Hint        string `json:"hint"`
	Required    bool   `json:"required"`
}

func (field Field) IsSecret() bool {
	return field.Type == FieldSecret
}

// Kind is one storage service, registered by the file that implements it.
type Kind struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
	// New builds a target from a configuration that has already been checked
	// against Fields, so it only has to fail on what it alone can judge.
	New func(config Config) (Target, error) `json:"-"`
}

var (
	kindMutex sync.RWMutex
	kinds     = map[string]*Kind{}
	kindOrder []string
)

// Register adds one kind. It is called from the init() of the file that
// implements it, so linking the file in is all it takes to offer the service.
func Register(kind *Kind) {
	kindMutex.Lock()
	defer kindMutex.Unlock()

	if _, exists := kinds[kind.Name]; !exists {
		kindOrder = append(kindOrder, kind.Name)
	}
	kinds[kind.Name] = kind
}

// Kinds is every registered kind in registration order, which is what the
// Settings page builds its form from.
func Kinds() []*Kind {
	kindMutex.RLock()
	defer kindMutex.RUnlock()

	res := make([]*Kind, 0, len(kindOrder))
	for _, name := range kindOrder {
		res = append(res, kinds[name])
	}
	return res
}

func GetKind(name string) *Kind {
	kindMutex.RLock()
	defer kindMutex.RUnlock()

	return kinds[strings.TrimSpace(name)]
}

// SecretOptions is which of a kind's options are credentials, so that whoever
// stores them knows what to encrypt without keeping a list of its own.
func SecretOptions(kindName string) []string {
	kind := GetKind(kindName)
	if kind == nil {
		return nil
	}

	res := []string{}
	for _, field := range kind.Fields {
		if field.IsSecret() {
			res = append(res, field.Name)
		}
	}
	sort.Strings(res)
	return res
}

// New builds the target one configuration describes.
func New(config Config) (Target, error) {
	kind := GetKind(config.Kind)
	if kind == nil {
		return nil, fmt.Errorf("unknown sync target: %s", config.Kind)
	}

	for _, field := range kind.Fields {
		if field.Required && config.Option(field.Name) == "" {
			return nil, fmt.Errorf("%s needs %s", kind.DisplayName, field.Label)
		}
	}
	return kind.New(config)
}

// Check is what the "Test" button asks: whether these credentials reach that
// storage. Listing is the cheapest operation every kind has, and the only one
// that proves the target both exists and can be read.
func Check(ctx context.Context, config Config) (Target, []File, error) {
	target, err := New(config)
	if err != nil {
		return nil, nil, err
	}

	files, err := target.List(ctx)
	if err != nil {
		return target, nil, err
	}
	return target, files, nil
}
