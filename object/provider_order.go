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

	"github.com/xorm-io/core"
)

// MoveProvider takes absolute positions so a repeated request cannot move the card twice.
func MoveProvider(id string, from, to int) error {
	if from < 0 || to < 0 || (to != from-1 && to != from+1) {
		return errors.New("choose an adjacent provider position")
	}

	session := ormer.Engine.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return err
	}
	defer session.Rollback()

	var rows []Provider
	err := session.Cols("owner", "name", "display_rank").Desc("display_rank", "created_time").
		Asc("owner", "name").ForUpdate().Find(&rows)
	if err != nil {
		return err
	}
	if from >= len(rows) || to >= len(rows) || rows[from].GetId() != id {
		return errors.New("the provider list changed; refresh and try again")
	}

	rows[from], rows[to] = rows[to], rows[from]
	for index, row := range rows {
		rank := len(rows) - index
		if row.DisplayRank == rank {
			continue
		}
		if _, err = session.ID(core.PK{row.Owner, row.Name}).Cols("display_rank").
			Update(&Provider{DisplayRank: rank}); err != nil {
			return err
		}
	}
	return session.Commit()
}
