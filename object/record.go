// Copyright 2024 The casbin Authors. All Rights Reserved.
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
	"strconv"
	"time"

	"github.com/xorm-io/core"
)

type Record struct {
	Id          int64  `xorm:"int notnull pk autoincr" json:"id"`
	Owner       string `xorm:"varchar(100) notnull" json:"owner"`
	CreatedTime string `xorm:"varchar(100) notnull" json:"createdTime"`

	Method    string `xorm:"varchar(100)" json:"method"`
	Host      string `xorm:"varchar(100)" json:"host"`
	Path      string `xorm:"varchar(100)" json:"path"`
	ClientIp  string `xorm:"varchar(100)" json:"clientIp"`
	UserAgent string `xorm:"varchar(512)" json:"userAgent"`
}

func GetRecords(owner string) ([]*Record, error) {
	records := []*Record{}
	err := ormer.Engine.Asc("id").Asc("host").Find(&records, &Record{Owner: owner})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func AddRecord(record *Record) (bool, error) {
	affected, err := ormer.Engine.Insert(record)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteRecord(record *Record) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{record.Id}).Delete(&Record{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func UpdateRecord(owner string, id string, record *Record) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{record.Id}).AllCols().Update(record)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func GetRecord(owner string, id string) (*Record, error) {
	idNum, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	record, err := getRecord(owner, int64(idNum))
	if err != nil {
		return nil, err
	}

	return record, nil
}

func getRecord(owner string, id int64) (*Record, error) {
	record := Record{Owner: owner, Id: id}
	existed, err := ormer.Engine.Get(&record)
	if err != nil {
		return nil, err
	}

	if existed {
		return &record, nil
	}
	return nil, nil
}

type DataCount struct {
	Data  string `json:"data"`
	Count int64  `json:"count"`
}

func GetMetrics(dataType string, startAt time.Time, top int) (*[]DataCount, error) {
	var dataCounts []DataCount
	createdAfter := "UNIX_TIMESTAMP(created_time) > ?"
	if ormer.driverName == "sqlite" {
		createdAfter = "CAST(strftime('%s', created_time) AS INTEGER) > ?"
	}
	err := ormer.Engine.Table("record").
		Where(createdAfter, startAt.Unix()).
		Select(dataType + " as data, COUNT(*) as count").
		GroupBy("data").
		Desc("count").
		Limit(top).
		Find(&dataCounts)
	if err != nil {
		return nil, err
	}
	return &dataCounts, nil
}

func GetMetricsOverTime(startAt time.Time, timeType string) (*[]DataCount, error) {
	var dataCounts []DataCount
	createdTime := "DATE_FORMAT(created_time, '" + timeType2Format(timeType) + "')"
	createdAfter := "UNIX_TIMESTAMP(created_time) > ?"
	if ormer.driverName == "sqlite" {
		createdTime = "strftime('" + timeType2Format(timeType) + "', created_time)"
		createdAfter = "CAST(strftime('%s', created_time) AS INTEGER) > ?"
	}
	err := ormer.Engine.Table("record").
		Where(createdAfter, startAt.Unix()).
		GroupBy(createdTime).
		Select(createdTime + " as data, COUNT(*) as count").
		Asc("data").
		Find(&dataCounts)
	if err != nil {
		return nil, err
	}
	return &dataCounts, nil
}

func timeType2Format(timeType string) string {
	switch timeType {
	case "hour":
		return "%Y-%m-%d %H"
	case "day":
		return "%Y-%m-%d"
	case "month":
		return "%Y-%m"
	}
	return "%Y-%m-%d %H"
}

func GetRecordCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Record{})
}

func GetPaginationRecords(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Record, error) {
	records := []*Record{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&records)
	if err != nil {
		return records, err
	}

	return records, nil
}
