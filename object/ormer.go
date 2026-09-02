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

package object

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
	_ "github.com/denisenkom/go-mssqldb" // db = mssql
	_ "github.com/go-sql-driver/mysql"   // db = mysql
	_ "github.com/lib/pq"                // db = postgres
	"github.com/xorm-io/core"
	"github.com/xorm-io/xorm"
	sqlitedriver "modernc.org/sqlite" // db = sqlite
)

func init() {
	// modernc.org/sqlite registers itself as "sqlite" only, so "sqlite3" in
	// app.conf would otherwise fail with an unknown driver.
	sql.Register("sqlite3", &sqlitedriver.Driver{})
}

var (
	ormer                   *Ormer = nil
	isCreateDatabaseDefined        = false
	createDatabase                 = true
)

func InitFlag() {
	if !isCreateDatabaseDefined {
		isCreateDatabaseDefined = true
		createDatabase = getCreateDatabaseFlag()
	}
}

func getCreateDatabaseFlag() bool {
	res := flag.Bool("createDatabase", false, "true if you need to create database")
	flag.Parse()
	return *res
}

func InitConfig() {
	err := beego.LoadAppConfig("ini", "../conf/app.conf")
	if err != nil {
		panic(err)
	}

	beego.BConfig.WebConfig.Session.SessionOn = true

	InitAdapter()
	CreateTables()
}

func InitAdapter() {
	driverName := conf.GetConfigDriverName()

	if createDatabase {
		err := createDatabaseForPostgres(driverName, conf.GetConfigDataSourceName(), conf.GetConfigString("dbName"))
		if err != nil {
			panic(err)
		}
	}

	ormer = NewAdapter(driverName, conf.GetConfigDataSourceName(), conf.GetConfigString("dbName"))

	tableNamePrefix := conf.GetConfigString("tableNamePrefix")
	tbMapper := core.NewPrefixMapper(core.SnakeMapper{}, tableNamePrefix)
	ormer.Engine.SetTableMapper(tbMapper)
}

func CreateTables() {
	if createDatabase {
		err := ormer.CreateDatabase()
		if err != nil {
			panic(err)
		}
	}

	ormer.createTable()
}

// PingDatabase reports whether the configured database answers right now. It is
// used by the startup summary, so it returns the error instead of panicking.
func PingDatabase() error {
	if ormer == nil || ormer.Engine == nil {
		return fmt.Errorf("the database adapter is not initialized")
	}

	return ormer.Engine.Ping()
}

// Ormer represents the database adapter for policy storage.
type Ormer struct {
	driverName     string
	dataSourceName string
	dbName         string
	Engine         *xorm.Engine
}

// finalizer is the destructor for Ormer.
func finalizer(a *Ormer) {
	err := a.Engine.Close()
	if err != nil {
		panic(err)
	}
}

// NewAdapter is the constructor for Ormer.
func NewAdapter(driverName string, dataSourceName string, dbName string) *Ormer {
	a := &Ormer{}
	a.driverName = driverName
	a.dataSourceName = dataSourceName
	a.dbName = dbName

	// Open the DB, create it if not existed.
	a.open()

	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)

	return a
}

func createDatabaseForPostgres(driverName string, dataSourceName string, dbName string) error {
	if driverName == "postgres" {
		db, err := sql.Open(driverName, dataSourceName)
		if err != nil {
			return err
		}
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s;", dbName))
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}

		return nil
	} else {
		return nil
	}
}

func (a *Ormer) CreateDatabase() error {
	if a.driverName == "postgres" {
		return nil
	}

	// SQLite has no "CREATE DATABASE": open() creates the file.
	if conf.IsSqliteDriver(a.driverName) {
		return nil
	}

	engine, err := xorm.NewEngine(a.driverName, a.dataSourceName)
	if err != nil {
		return err
	}
	defer engine.Close()

	_, err = engine.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s default charset utf8mb4 COLLATE utf8mb4_general_ci", a.dbName))
	return err
}

func (a *Ormer) open() {
	dataSourceName := a.dataSourceName + a.dbName
	if a.driverName != "mysql" {
		dataSourceName = a.dataSourceName
	}

	isSqlite := conf.IsSqliteDriver(a.driverName)
	if isSqlite {
		err := prepareSqliteDir(dataSourceName)
		if err != nil {
			panic(err)
		}

		dataSourceName = withSqlitePragmas(dataSourceName)
	}

	engine, err := xorm.NewEngine(a.driverName, dataSourceName)
	if err != nil {
		panic(err)
	}

	if isSqlite {
		// SQLite takes a single writer at a time.
		engine.SetMaxOpenConns(1)
	}

	a.Engine = engine
}

// prepareSqliteDir creates the directory holding the SQLite file, since a fresh
// checkout has no ./data yet.
func prepareSqliteDir(dataSourceName string) error {
	dir := filepath.Dir(dataSourceName)
	if dir == "" || dir == "." {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}

// withSqlitePragmas keeps SQLite usable under concurrent requests: WAL lets
// readers run alongside the writer, and busy_timeout waits instead of failing
// with SQLITE_BUSY.
func withSqlitePragmas(dataSourceName string) string {
	if strings.Contains(dataSourceName, "_pragma=") {
		return dataSourceName
	}

	separator := "?"
	if strings.Contains(dataSourceName, "?") {
		separator = "&"
	}

	return dataSourceName + separator + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

func (a *Ormer) close() {
	_ = a.Engine.Close()
	a.Engine = nil
}

func (a *Ormer) createTable() {
	showSql := conf.GetConfigBool("showSql")
	a.Engine.ShowSQL(showSql)

	// The settings the web UI can change, held in one built-in row.
	err := a.Engine.Sync2(new(Setting))
	if err != nil {
		panic(err)
	}

	// Local users, used for sign-in when no Casdoor is configured.
	err = a.Engine.Sync2(new(User))
	if err != nil {
		panic(err)
	}

	// Register Provider table for LLM gateway milestone 1.1.
	err = a.Engine.Sync2(new(Provider))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync2(new(Agent))
	if err != nil {
		panic(err)
	}

	err = a.Engine.Sync2(new(LlmRecord))
	if err != nil {
		panic(err)
	}

	// Token prices edited on the Pricing page or written by a models.dev sync,
	// which override the built-in list prices.
	err = a.Engine.Sync2(new(LlmPriceEntry))
	if err != nil {
		panic(err)
	}
}
