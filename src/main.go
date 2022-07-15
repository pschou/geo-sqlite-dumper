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
	"math"
	"os"
	"strings"
	"time"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
	"github.com/pschou/go-params"
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
		func() { // Anonymous function to make sure the defer close will work
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

			var total_dist, total_alt float64
			var start_time, end_time float64
			var tableFolders []kml.Element
			var kml_coords []kml.Coordinate
			var prev_kml_coord *kml.Coordinate
			var tbl_name string

			store_event := func() {
				total_time := end_time - start_time
				total_pts := float64(len(kml_coords))
				s_sec, s_dec := math.Modf(start_time)
				s_time := time.Unix(int64(s_sec), int64(s_dec*(1e9)))
				e_sec, e_dec := math.Modf(end_time)
				e_time := time.Unix(int64(e_sec), int64(e_dec*(1e9)))
				tableFolder := kml.Folder(
					kml.Name(tbl_name),
					kml.Placemark(
						kml.Name("Path "+s_time.Format(time.RFC3339Nano)+" - "+e_time.Format(time.RFC3339Nano)),
						kml.Description(fmt.Sprintf("Average {speed: %dm/s %dmph, altitude: %dm}", total_dist/total_time, total_dist/total_time*2.236936, total_alt/total_pts)),
						kml.StyleURL("#yellowLineGreenPoly"),
						kml.LineString(
							kml.Extrude(true),
							kml.Tessellate(true),
							kml.AltitudeMode(kml.AltitudeModeAbsolute),
							kml.Coordinates(kml_coords...)),
					),
				)
				tableFolders = append(tableFolders, tableFolder)

				total_dist, total_alt = 0, 0
				start_time, end_time = 0, 0
				tableFolders = []kml.Element{}
				kml_coords = []kml.Coordinate{}
				prev_kml_coord = nil
			}

			for _, tbl_name = range tbl_names {
				if *debug {
					log.Println("Table", tbl_name)
				}
				clm_names, err := getColumns(conn, tbl_name)
				if err != nil {
					panic(err)
				}
				if *debug {
					log.Println("cols:", clm_names)
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
					if *debug {
						log.Println("Missing lat or long in file")
					}
					continue
				}

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
					var kml_coord kml.Coordinate

					if len(idate) > 0 {
						cur_time, _, _ := stmt.ColumnDouble(idate[0])
						if start_time == 0 {
							start_time, end_time = cur_time, cur_time
						} else if cur_time-start_time > 600 {
							store_event()
							start_time, end_time = cur_time, cur_time
						} else {
							end_time = cur_time
						}
					}

					lat, ok, err := stmt.ColumnDouble(ilat[0])
					if *debug && (!ok || err != nil) {
						log.Println("nil long", err)
					}
					long, ok, err := stmt.ColumnDouble(ilat[0])
					if *debug && (!ok || err != nil) {
						log.Println("nil lat", err)
					}
					if len(ialt) > 0 {
						alt, ok, err := stmt.ColumnDouble(ilat[0])
						if *debug && (!ok || err != nil) {
							log.Println("nil alt", err)
						}
						kml_coord = kml.Coordinate{
							Lon: long,
							Lat: lat,
							Alt: alt,
						}
						total_alt += alt
					} else {
						kml_coord = kml.Coordinate{
							Lon: long,
							Lat: lat,
							Alt: 0,
						}
					}
					kml_coords = append(kml_coords, kml_coord)
					if prev_kml_coord == nil {
						// Center point for altitude
						r1 := EarthRadius(prev_kml_coord.Lat)
						r2 := EarthRadius(kml_coord.Lat)
						arc := ArcDistance(
							prev_kml_coord.Lat, prev_kml_coord.Lon,
							kml_coord.Lat, kml_coord.Lon,
						)
						// Using a first order cartesian approximation, and not the
						// incomplete elliptic intergral:
						total_dist += math.Sqrt(Sq(r1+prev_kml_coord.Alt-r2-kml_coord.Alt) +
							Sq(arc*(r1+prev_kml_coord.Alt+r2+kml_coord.Alt)/2))
					}
					prev_kml_coord = &kml_coord

					if err != nil {
						log.Fatalf("scan failed while querying data: %v", err)
					}
				}

				store_event()

			}
			sqlfileFolders = append(sqlfileFolders, kml.Folder(
				append([]kml.Element{
					kml.Name(f),
					kml.Open(true),
				},
					tableFolders...,
				)...,
			))
		}()
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
