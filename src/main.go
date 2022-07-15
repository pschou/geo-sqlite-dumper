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

	for _, f := range params.Args() {
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
			var path_time []time.Time
			var tableFolders []kml.Element
			var eventFolders []kml.Element
			var kml_coords []kml.Coordinate
			var prev_kml_coord *kml.Coordinate
			var tbl_name string

			store_event := func() {
				defer func() {
					// When this function is returned, clear out the variables
					total_dist, total_alt = 0, 0
					prev_kml_coord = nil
					kml_coords = []kml.Coordinate{}
					path_time = []time.Time{}
				}()

				if len(kml_coords) == 0 {
					return
				}
				total_pts := float64(len(kml_coords))
				s_time := path_time[0]
				e_time := path_time[len(path_time)-1]
				var elements []kml.Element
				altMode := kml.AltitudeModeAbsolute
				if total_alt == 0 {
					altMode = kml.AltitudeModeClampToGround
				}
				// Make path entry if more than one point is specified
				if len(kml_coords) > 1 {
					elements = append(elements,
						kml.Placemark(
							kml.Name("Path"),
							kml.StyleURL("#yellowLineGreenPoly"),
							kml.LineString(
								kml.Extrude(true),
								kml.Tessellate(true),
								kml.AltitudeMode(altMode),
								kml.Coordinates(kml_coords...)),
						),
					)
				}

				var pointElements []kml.Element
				for i, kml_coord := range kml_coords {
					pointElements = append(pointElements,
						kml.Placemark(
							kml.Name(path_time[i].Format(time.RFC3339Nano)),
							kml.Point(kml.Coordinates(kml_coord)),
						),
					)
				}
				elements = append(elements, kml.Folder(
					append([]kml.Element{
						kml.Name("Points"),
					},
						pointElements...,
					)...,
				))

				eventFolders = append(eventFolders,
					kml.Folder(
						append([]kml.Element{
							kml.Name("Path " + s_time.Format(time.RFC3339Nano) + " - " + e_time.Format(time.RFC3339Nano)),
							kml.Description(fmt.Sprintf("{time: %s, dist: %dm, altitude: %dm}", e_time.Sub(s_time), total_dist, total_alt/total_pts)),
						},
							elements...,
						)...,
					),
				)

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
				count := 0

				for {
					hasRow, err := stmt.Step()
					if err != nil {
						log.Fatalf("step failed while querying data: %v", err)
					}
					if !hasRow {
						break
					}
					count++

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
						c_sec, c_dec := math.Modf(cur_time)
						c_time := time.Unix(int64(c_sec+978307200), int64(c_dec+1e9))
						if len(path_time) > 0 && c_time.Sub(path_time[len(path_time)-1]) > 10*time.Minute {
							store_event()
						}
						path_time = append(path_time, c_time)
					}

					lat, ok, err := stmt.ColumnDouble(ilat[0])
					if *debug && (!ok || err != nil) {
						log.Println("nil long", err)
					}
					long, ok, err := stmt.ColumnDouble(ilong[0])
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
					if prev_kml_coord != nil {
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

				tableFolders = append(tableFolders, kml.Folder(
					append([]kml.Element{
						kml.Name(fmt.Sprintf("%s (%d)", tbl_name, count)),
						kml.Open(false),
					},
						tableFolders...,
					)...,
				))

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
