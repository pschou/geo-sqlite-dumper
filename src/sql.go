// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
	_ "github.com/twpayne/go-geom/encoding/kml"
)

func getTables(conn *sqlite3.Conn) ([]string, error) {
	var tables []string
	// Prepare can prepare a statement and optionally also bind arguments
	stmt, err := conn.Prepare(`SELECT name FROM sqlite_schema WHERE type='table';`)
	if err != nil {
		stmt, err = conn.Prepare(`SELECT name FROM sqlite_master WHERE type='table';`)
	}
	if err != nil {
		return tables, fmt.Errorf("failed to select table list: %v", err)
	}
	defer stmt.Close()

	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return tables, fmt.Errorf("failed stepping through table list: %v", err)
		}
		if !hasRow {
			break
		}

		// Use Scan to access column data from a row
		var name string
		err = stmt.Scan(&name)
		if err != nil {
			return tables, fmt.Errorf("failed scanning through table list: %v", err)
		}

		tables = append(tables, name)
	}
	return tables, nil
}
func getColumns(conn *sqlite3.Conn, table string) ([]string, error) {
	var columns []string
	// Prepare can prepare a statement and optionally also bind arguments
	stmt, err := conn.Prepare(`SELECT name FROM PRAGMA_TABLE_INFO(?);`, table)
	if err != nil {
		return columns, fmt.Errorf("failed to select column list: %v", err)
	}
	defer stmt.Close()

	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return columns, fmt.Errorf("failed stepping through column list: %v", err)
		}
		if !hasRow {
			break
		}

		// Use Scan to access column data from a row
		var name string
		err = stmt.Scan(&name)
		if err != nil {
			return columns, fmt.Errorf("failed scanning through column list: %v", err)
		}

		columns = append(columns, name)
	}
	return columns, nil
}
