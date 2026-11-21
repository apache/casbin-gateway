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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/apache/casbin-gateway/util"
	"github.com/google/uuid"
)

type WorkspaceState string

const (
	WorkspaceStopped WorkspaceState = "stopped"
	WorkspaceRunning WorkspaceState = "running"
)

var (
	ErrWorkspaceNotFound          = errors.New("workspace not found")
	ErrWorkspaceStaleRevision     = errors.New("workspace revision is stale")
	ErrWorkspaceCreateKeyConflict = errors.New("workspace creation key was already used for a different request")
)

type WorkspaceSpec struct {
	Name       string
	Repository string
	Ref        string
	Profile    string
}

type Workspace struct {
	Id             string         `xorm:"varchar(36) notnull pk" json:"id"`
	OwnerKey       string         `xorm:"char(64) notnull index" json:"-"`
	CreateKey      string         `xorm:"char(64) notnull unique" json:"-"`
	CreateSpecHash string         `xorm:"char(64) notnull" json:"-"`
	Name           string         `xorm:"varchar(100) notnull" json:"name"`
	Repository     string         `xorm:"varchar(500)" json:"repository"`
	Ref            string         `xorm:"varchar(200)" json:"ref"`
	Profile        string         `xorm:"varchar(100) notnull" json:"profile"`
	DesiredState   WorkspaceState `xorm:"varchar(20) notnull" json:"desiredState"`
	Revision       int64          `xorm:"bigint notnull" json:"revision"`
	Generation     int64          `xorm:"bigint notnull" json:"generation"`
	CreatedTime    string         `xorm:"varchar(100) notnull" json:"createdTime"`
	UpdatedTime    string         `xorm:"varchar(100) notnull" json:"updatedTime"`
}

// CreateWorkspace scopes the request key to the caller's authenticated principal.
func CreateWorkspace(principal string, requestKey string, spec WorkspaceSpec) (*Workspace, error) {
	ownerKey, err := workspaceOwnerKey(principal)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(requestKey) == "" || !validWorkspaceField(requestKey, 128) {
		return nil, errors.New("workspace creation key must be 1 to 128 bytes")
	}
	if err := normalizeWorkspaceSpec(&spec); err != nil {
		return nil, err
	}

	createKey := workspaceDigest(ownerKey + requestKey)
	specHash := workspaceDigest(strings.Join([]string{spec.Name, spec.Repository, spec.Ref, spec.Profile}, "\x00"))
	existing, err := workspaceByCreateKey(ownerKey, createKey)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		now := util.GetCurrentTime()
		workspace := &Workspace{
			Id:             uuid.NewString(),
			OwnerKey:       ownerKey,
			CreateKey:      createKey,
			CreateSpecHash: specHash,
			Name:           spec.Name,
			Repository:     spec.Repository,
			Ref:            spec.Ref,
			Profile:        spec.Profile,
			DesiredState:   WorkspaceStopped,
			Revision:       1,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		_, insertErr := ormer.Engine.Insert(workspace)
		if insertErr == nil {
			return workspace, nil
		}
		// A concurrent writer may have inserted the same key first.
		existing, err = workspaceByCreateKey(ownerKey, createKey)
		if err != nil || existing == nil {
			return nil, errors.Join(insertErr, err)
		}
	}
	if existing.CreateSpecHash != specHash {
		return nil, ErrWorkspaceCreateKeyConflict
	}
	return existing, nil
}

func workspaceByCreateKey(ownerKey string, createKey string) (*Workspace, error) {
	workspace := &Workspace{}
	found, err := ormer.Engine.Where("owner_key = ? AND create_key = ?", ownerKey, createKey).Get(workspace)
	if err != nil || !found {
		return nil, err
	}
	return workspace, nil
}

func GetWorkspace(principal string, id string) (*Workspace, error) {
	ownerKey, err := workspaceOwnerKey(principal)
	if err != nil {
		return nil, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return nil, ErrWorkspaceNotFound
	}
	workspace := &Workspace{}
	found, err := ormer.Engine.ID(id).Where("owner_key = ?", ownerKey).Get(workspace)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrWorkspaceNotFound
	}
	return workspace, nil
}

func ListWorkspaces(principal string) ([]*Workspace, error) {
	ownerKey, err := workspaceOwnerKey(principal)
	if err != nil {
		return nil, err
	}
	workspaces := []*Workspace{}
	if err := ormer.Engine.Where("owner_key = ?", ownerKey).Desc("created_time").Asc("id").Find(&workspaces); err != nil {
		return nil, err
	}
	return workspaces, nil
}

// UpdateWorkspace advances the runtime generation only when a start is requested.
func UpdateWorkspace(principal string, id string, revision int64, name string, desiredState WorkspaceState) (*Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" || !validWorkspaceField(name, 100) {
		return nil, errors.New("workspace name must be 1 to 100 bytes")
	}
	if desiredState != WorkspaceStopped && desiredState != WorkspaceRunning {
		return nil, errors.New("invalid desired workspace state")
	}

	workspace, err := GetWorkspace(principal, id)
	if err != nil {
		return nil, err
	}
	if workspace.Revision != revision {
		return nil, ErrWorkspaceStaleRevision
	}
	if workspace.Name == name && workspace.DesiredState == desiredState {
		return workspace, nil
	}

	if desiredState == WorkspaceRunning && workspace.DesiredState != WorkspaceRunning {
		workspace.Generation++
	}
	workspace.Name = name
	workspace.DesiredState = desiredState
	workspace.Revision++
	workspace.UpdatedTime = util.GetCurrentTime()
	affected, err := ormer.Engine.ID(workspace.Id).
		Where("owner_key = ? AND revision = ?", workspace.OwnerKey, revision).
		Cols("name", "desired_state", "revision", "generation", "updated_time").
		Update(workspace)
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, ErrWorkspaceStaleRevision
	}
	return workspace, nil
}

func normalizeWorkspaceSpec(spec *WorkspaceSpec) error {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Repository = strings.TrimSpace(spec.Repository)
	spec.Ref = strings.TrimSpace(spec.Ref)
	spec.Profile = strings.TrimSpace(spec.Profile)
	if spec.Name == "" || !validWorkspaceField(spec.Name, 100) {
		return errors.New("workspace name must be 1 to 100 bytes")
	}
	if spec.Profile == "" || !validWorkspaceField(spec.Profile, 100) {
		return errors.New("workspace profile must be 1 to 100 bytes")
	}
	if !validWorkspaceField(spec.Ref, 200) || (spec.Ref != "" && spec.Repository == "") {
		return errors.New("a workspace ref requires a repository and must be at most 200 bytes")
	}
	if spec.Repository != "" {
		repo, err := url.Parse(spec.Repository)
		if err != nil || !validWorkspaceField(spec.Repository, 500) || repo.Scheme != "https" ||
			repo.Hostname() == "" || repo.User != nil || repo.ForceQuery || repo.RawQuery != "" || repo.Fragment != "" {
			return errors.New("workspace repository must be an HTTPS URL without credentials, query or fragment")
		}
	}
	return nil
}

func validWorkspaceField(value string, maxBytes int) bool {
	return len(value) <= maxBytes && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func workspaceOwnerKey(principal string) (string, error) {
	if strings.TrimSpace(principal) == "" {
		return "", errors.New("workspace principal is required")
	}
	// The digest keeps opaque principal IDs distinct on case-insensitive databases.
	return workspaceDigest(principal), nil
}

func workspaceDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
