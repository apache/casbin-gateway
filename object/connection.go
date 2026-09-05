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

package object

import (
	"errors"
	"fmt"
	"sort"

	"github.com/apache/casbin-gateway/connector"
	"github.com/apache/casbin-gateway/mcpproxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// Connection is one connector the operator has connected, and the credentials
// it was connected with. Holding them here is what makes one connection a
// single thing rather than a copy per agent: connecting writes it into every
// agent that was picked, and disconnecting takes it out of all of them.
//
// What is written into an agent is Gateway standing in front of the server, so
// the credential here is the only copy: it is handed to the proxy over loopback
// when a session starts and never reaches the agent's own file.
type Connection struct {
	Owner string `xorm:"varchar(100) notnull pk" json:"owner"`
	// Name is the connector's catalog id: one connection per connector.
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// Credentials holds the fields the connector asked for. Secret values are
	// ciphertext at rest when "apiKeyEncryptionKey" is set, like a provider's
	// API key, and are masked on the way out.
	Credentials map[string]string `xorm:"mediumtext json" json:"credentials"`
	// Agents is every agent id this connection's MCP server was written into.
	Agents []string `xorm:"mediumtext json" json:"agents"`
	// Endpoint and Executable are the Gateway the written entries name. A
	// Gateway whose port changed, or that moved on disk, leaves every one of
	// those entries starting a program that is not there or reporting to a port
	// nothing answers on; comparing these is what notices, so the entries can be
	// written again rather than failing silently in each agent.
	Endpoint   string `xorm:"varchar(255)" json:"endpoint"`
	Executable string `xorm:"varchar(500)" json:"executable"`

	// What the server said about itself the last time it was tested. Tools is
	// what turns one switch for the whole connection into one per tool, so a
	// connection nobody has tested is governed more coarsely than one that has.
	ServerName string               `xorm:"varchar(200)" json:"serverName"`
	Tools      []mcpproxy.ProbeTool `xorm:"mediumtext json" json:"tools"`
	ProbedTime string               `xorm:"varchar(100)" json:"probedTime"`
	// ProbeError is why the last test failed, empty when it worked. It is kept
	// so the page can say what is wrong without testing again.
	ProbeError string `xorm:"varchar(1000)" json:"probeError"`
}

func (connection *Connection) GetId() string {
	return fmt.Sprintf("%s/%s", connection.Owner, connection.Name)
}

// credentialAad binds one field's ciphertext to its own row and key, so a value
// copied into another connection, or into another field of the same one, no
// longer decrypts.
func credentialAad(connection *Connection, key string) string {
	return connection.GetId() + "/" + key
}

func encryptCredentials(connection *Connection) error {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return fmt.Errorf("no connector named %q", connection.Name)
	}

	for _, key := range found.SecretKeys() {
		value, ok := connection.Credentials[key]
		if !ok || value == "" {
			continue
		}
		encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), value, credentialAad(connection, key))
		if err != nil {
			return err
		}
		connection.Credentials[key] = encrypted
	}
	return nil
}

// decryptConnection leaves a value that will not decrypt as it is, matching how
// a provider's key behaves when the encryption key changed: the row is still
// listed, and using it is what reports the problem.
func decryptConnection(connection *Connection) {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return
	}

	for _, key := range found.SecretKeys() {
		value, ok := connection.Credentials[key]
		if !ok || value == "" {
			continue
		}
		decrypted, err := util.DecryptWithKey(apiKeyEncryptionSecret(), value, credentialAad(connection, key))
		if err == nil {
			connection.Credentials[key] = decrypted
		}
	}
}

// Masked is a copy safe to send to the browser: every secret field is replaced
// by ApiKeyMask, so a stored credential never leaves this process.
func (connection *Connection) Masked() *Connection {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return connection
	}

	masked := *connection
	masked.Credentials = map[string]string{}
	for key, value := range connection.Credentials {
		masked.Credentials[key] = value
	}
	for _, key := range found.SecretKeys() {
		if masked.Credentials[key] != "" {
			masked.Credentials[key] = ApiKeyMask
		}
	}
	return &masked
}

func GetConnections(owner string) ([]*Connection, error) {
	connections := []*Connection{}
	err := ormer.Engine.Where("owner = ?", owner).Find(&connections)
	if err != nil {
		return nil, err
	}

	for _, connection := range connections {
		decryptConnection(connection)
	}
	sort.Slice(connections, func(i int, j int) bool { return connections[i].Name < connections[j].Name })
	return connections, nil
}

// getAllConnections is every connection on this machine, whoever owns it. The
// repair pass uses it: an entry naming a Gateway that has moved is stale for
// its owner as much as for anybody else.
func getAllConnections() ([]*Connection, error) {
	connections := []*Connection{}
	if err := ormer.Engine.Find(&connections); err != nil {
		return nil, err
	}

	for _, connection := range connections {
		decryptConnection(connection)
	}
	return connections, nil
}

func GetConnection(owner string, name string) (*Connection, error) {
	connection := Connection{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&connection)
	if err != nil || !existed {
		return nil, err
	}

	decryptConnection(&connection)
	return &connection, nil
}

// SaveConnection writes one connection, creating it the first time. A secret
// left at ApiKeyMask keeps whatever is already stored, so an edit that only
// changes the agent list never has to resend the credential.
func SaveConnection(connection *Connection) error {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return fmt.Errorf("no connector named %q", connection.Name)
	}
	if connection.Credentials == nil {
		connection.Credentials = map[string]string{}
	}

	existing, err := GetConnection(connection.Owner, connection.Name)
	if err != nil {
		return err
	}
	for _, key := range found.SecretKeys() {
		if connection.Credentials[key] != ApiKeyMask {
			continue
		}
		if existing == nil {
			return errors.New("a masked credential has nothing to keep")
		}
		connection.Credentials[key] = existing.Credentials[key]
	}

	// Rendering here is what stops a half-filled connection from being stored
	// at all, rather than failing later when an agent tries to start it.
	if _, err := found.Render(connection.Credentials); err != nil {
		return err
	}

	connection.UpdatedTime = util.GetCurrentTime()
	if err := encryptCredentials(connection); err != nil {
		return err
	}

	if existing == nil {
		connection.CreatedTime = util.GetCurrentTime()
		_, err = ormer.Engine.Insert(connection)
		return err
	}
	return updateConnection(connection)
}

// updateConnection writes every column of an existing row. AllCols is what
// makes a credential that was cleared actually clear, rather than xorm skipping
// the empty value and leaving the old one in place.
func updateConnection(connection *Connection) error {
	_, err := ormer.Engine.ID(core.PK{connection.Owner, connection.Name}).AllCols().Update(connection)
	return err
}

func DeleteConnection(owner string, name string) error {
	_, err := ormer.Engine.ID(core.PK{owner, name}).Delete(&Connection{})
	return err
}
