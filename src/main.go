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
	"log"
	"os"
	"strings"
	"time"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
	"github.com/pschou/go-params"
	"github.com/twpayne/go-geom"
	ekml "github.com/twpayne/go-geom/encoding/kml"
	"github.com/twpayne/go-kml"
)

var debug *bool
var version = ""

func main() {

	params.Usage = func() {
		fmt.Fprintf(os.Stderr, "SqlLite3 to KML (github.com/pschou/sqlite2kml)\nApache 2.0 license, provided AS-IS -- not responsible for loss.\nUsage implies agreement.  Version: %s\n\nUsage: %s [options...]\n\n", version, os.Args[0])
		params.PrintDefaults()
	}
	debug = params.Pres("debug", "Verbose output")
	params.CommandLine.Indent = 2
	params.Parse()

	var sqlfileFolders []kml.Element

	for _, f := range os.Args[1:] {
		conn, err := sqlite3.Open(f)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		// It's always a good idea to set a busy timeout
		conn.BusyTimeout(5 * time.Second)
		tbl_names, err := getTables(conn)
		if err != nil {
			panic(err)
		}

		var tableFolders []kml.Element
		for _, tbl_name := range tbl_names {
			if *debug {
				fmt.Println("Table", tbl_name)
			}
			clm_names, err := getColumns(conn, tbl_name)
			if err != nil {
				panic(err)
			}
			if *debug {
				fmt.Println("cols:", clm_names)
			}

			//var long, lat, alt, date []string
			var ilong, ilat, ialt, idate []int
			for i, col := range clm_names {
				lcol := strings.ToLower(col)
				switch {
				case strings.HasSuffix(lcol, "latitude"):
					//lat = append(lat, col)
					ilat = append(ilat, i)
				case strings.HasSuffix(lcol, "longitude"):
					//long = append(long, col)
					ilong = append(ilong, i)
				case strings.HasSuffix(lcol, "altitude"):
					//alt = append(alt, col)
					ialt = append(ialt, i)
				case strings.HasSuffix(lcol, "date"):
					//date = append(date, col)
					idate = append(idate, i)
				}
			}

			if !(len(ilong) > 0 && len(ilat) > 0) {
				log.Fatal("Missing lat or long in file")
			}

			layout := geom.XY
			if len(ialt) > 0 {
				layout = geom.XYZ
			}
			mp := geom.NewMultiPoint(layout)

			// Prepare can prepare a statement and optionally also bind arguments
			var stmt *sqlite3.Stmt
			if len(idate) > 0 {
				stmt, err = conn.Prepare(`SELECT * FROM ` + tbl_name + ` ORDER BY ` + clm_names[idate[0]])
			} else {
				stmt, err = conn.Prepare(`SELECT * FROM ` + tbl_name)
			}
			if err != nil {
				log.Fatalf("failed to select data from table: %v", err)
			}
			defer stmt.Close()

			for {
				hasRow, err := stmt.Step()
				if err != nil {
					log.Fatalf("step failed while querying data: %v", err)
				}
				if !hasRow {
					break
				}

				/*
					// Use Scan to access column data from a row
					data := make([]interface{}, len(clm_names))
					var pdata []interface{}
					for i := range data {
						pdata = append(pdata, &data[i])
					}
					err = stmt.Scan(pdata...)*/
				pt := geom.NewPoint(layout)

				lat, ok, err := stmt.ColumnDouble(ilat[0])
				if *debug && (!ok || err != nil) {
					fmt.Println("nil long", err)
				}
				long, ok, err := stmt.ColumnDouble(ilat[0])
				if *debug && (!ok || err != nil) {
					fmt.Println("nil lat", err)
				}
				if len(ialt) > 0 {
					alt, ok, err := stmt.ColumnDouble(ilat[0])
					if *debug && (!ok || err != nil) {
						fmt.Println("nil alt", err)
					}
					pt.SetCoords(geom.Coord{long, lat, alt})
				} else {
					pt.SetCoords(geom.Coord{long, lat})
				}
				mp.Push(pt)

				if err != nil {
					log.Fatalf("scan failed while querying data: %v", err)
				}
			}

			tableFolder := kml.Folder(
				kml.Name(tbl_name),
				kml.Placemark(
					ekml.EncodeMultiPoint(mp),
				),
			)
			tableFolders = append(tableFolders, tableFolder)
		}
		sqlfileFolders = append(sqlfileFolders, kml.Folder(
			append([]kml.Element{
				kml.Name(f),
				kml.Open(true),
			},
				tableFolders...,
			)...,
		))
	}

	result := kml.KML(
		kml.Document(
			append([]kml.Element{
				kml.Name("sqlite2kml"),
				kml.Open(true),
			},
				sqlfileFolders...,
			)...,
		))

	result.WriteIndent(os.Stdout, "", "  ")
}
