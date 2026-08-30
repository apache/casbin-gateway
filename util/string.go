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
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"strconv"
	"strings"
)

func IndexAt(s, sep string, n int) int {
	idx := strings.Index(s[n:], sep)
	if idx > -1 {
		idx += n
	}
	return idx
}

func ParseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}

	return i
}

func ParseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}

	return f
}

func GetOwnerAndNameFromId(id string) (string, string) {
	tokens := strings.Split(id, "/")
	if len(tokens) != 2 {
		panic(errors.New("GetOwnerAndNameFromId() error, wrong token count for ID: " + id))
	}

	return tokens[0], tokens[1]
}

func GetOwnerAndNameFromId3(id string) (string, string, string) {
	tokens := strings.Split(id, "/")
	if len(tokens) != 3 {
		panic(errors.New("GetOwnerAndNameFromId3() error, wrong token count for ID: " + id))
	}

	return tokens[0], fmt.Sprintf("%s/%s", tokens[0], tokens[1]), tokens[2]
}

func GetOwnerAndNameFromId3New(id string) (string, string, string) {
	tokens := strings.Split(id, "/")
	if len(tokens) != 3 {
		panic(errors.New("GetOwnerAndNameFromId3New() error, wrong token count for ID: " + id))
	}

	return tokens[0], tokens[1], tokens[2]
}

func GetIdFromOwnerAndName(owner string, name string) string {
	return fmt.Sprintf("%s/%s", owner, name)
}

func ReadStringFromPath(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func WriteStringToPath(s string, path string) {
	err := ioutil.WriteFile(path, []byte(s), 0644)
	if err != nil {
		panic(err)
	}
}

func ReadBytesFromPath(path string) []byte {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return data
}

func WriteBytesToPath(b []byte, path string) {
	err := ioutil.WriteFile(path, b, 0644)
	if err != nil {
		panic(err)
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		var c byte
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		c = charset[index.Int64()]
		b[i] = c
	}
	return string(b), nil
}

// GenerateToken returns a random opaque token, used where a value has to be
// unguessable but is never a password anyone types.
func GenerateToken(length int) string {
	token, err := generateRandomString(length)
	if err != nil {
		panic(err)
	}
	return token
}

func SnakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	num := len(s)
	for i := 0; i < num; i++ {
		d := s[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			data = append(data, '_')
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	result := strings.ToLower(string(data[:]))
	return strings.ReplaceAll(result, " ", "")
}
