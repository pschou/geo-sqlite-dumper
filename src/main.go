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
	"encoding/json"
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
		fmt.Fprintf(os.Stderr, "SqlLite3 to KML (github.com/pschou/sqlite2kml)\n"+
			"Apache 2.0 license, provided AS-IS -- not responsible for loss.\nUsage implies agreement.  Version: %s\n\n"+
			"Usage: %s [options...] [files...]\n\n", version, os.Args[0])
		params.PrintDefaults()
	}
	debug = params.Pres("debug", "Verbose output")
	event := params.Duration("e event", 10*time.Minute, "Event qualifier, time between events to split on", "TIME")
	busy_timeout := params.Duration("timeout", 10*time.Second, "Busy timeout for SQLite calls", "TIME")
	name := params.String("N name", "sqlite2kml", "Name to use for base KML folder", "TEXT")
	qry := params.String("q query", "", "Custom query for SQLite", "SQL")
	open_file := params.Bool("open-file", false, "Open all the file-named-folders when KML is loaded", "BOOL")
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
			conn.BusyTimeout(*busy_timeout)

			// If no query is specified, dump all tables to file
			tbl_names := []string{""}
			if *qry == "" {
				tbl_names, err = getTables(conn)
			}

			if err != nil {
				panic(err)
			}

			var total_dist, total_alt float64
			var path_time []time.Time
			var tableFolders []kml.Element
			var eventFolders []kml.Element
			var kml_coords []kml.Coordinate
			var kml_desc []*kml.SimpleElement
			var prev_kml_coord *kml.Coordinate
			var tbl_name string

			// Function to take the values collected and store them into a KML element
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
				if *debug {
					log.Println("storing event", s_time, e_time)
				}

				var elements []kml.Element
				altMode := kml.AltitudeModeAbsolute
				if total_alt == 0 {
					altMode = kml.AltitudeModeClampToGround
				}
				// Create a path if more than one point is specified
				if len(kml_coords) > 1 && !strings.HasSuffix(tbl_name, "OFINTERESTMO") {
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

				details := []kml.Element{
					kml.Name(fmt.Sprintf("Event (%d) %s - %s", len(kml_coords), s_time.Format(time.RFC3339Nano), e_time.Format(time.RFC3339Nano))),
				}

				if len(kml_coords) > 1 {
					details = append(details,
						kml.Description(fmt.Sprintf("{time: %s, dist: %fm, mean altitude: %fm}", e_time.Sub(s_time), total_dist, total_alt/total_pts)),
					)
				}

				eventFolders = append(eventFolders,
					kml.Folder(
						append(details,
							elements...,
						)...,
					),
				)

			}

			// Loop over all the tables found in database, or call custom query
			for _, tbl_name = range tbl_names {
				func() { // Anonymous function to ensure the defer will close the statement as needed
					// Ensure the variables are cleared on new table
					total_dist, total_alt = 0, 0
					prev_kml_coord = nil
					kml_coords = []kml.Coordinate{}
					path_time = []time.Time{}

					if *debug {
						log.Println("Table", tbl_name)
					}

					var stmt *sqlite3.Stmt
					if *qry == "" {
						clm_names, err := getColumns(conn, tbl_name)
						if err != nil {
							panic(err)
						}

						var idate, idate_top []int
						for i, col := range clm_names {
							lcol := strings.ToLower(col)
							if strings.HasSuffix(lcol, "date") {
								switch {
								case strings.HasSuffix(lcol, "creationdate"):
									idate_top = append(idate_top, i)
								case strings.HasSuffix(lcol, "startdate"):
									idate_top = append([]int{i}, idate_top...)
								default:
									//date = append(date, col)
									idate = append(idate, i)
								}
							}
						}
						idate = append(idate_top, idate...)

						sel_tbl := `SELECT * FROM ` + tbl_name
						sel_prefix := ""

						if strings.HasSuffix(tbl_name, "TRANSITIONMO") && contains(tbl_names, strings.TrimSuffix(tbl_name, "TRANSITIONMO")+"MO") {
							join_tbl := strings.TrimSuffix(tbl_name, "TRANSITIONMO") + "MO"
							sel_tbl = `SELECT * FROM ` + tbl_name + ` AS a LEFT JOIN ` + join_tbl + ` AS b ON a.ZLOCATIONOFINTEREST = b.Z_PK`
							sel_prefix = "a."
						}

						// Prepare an SQL statement for data parsing
						if len(idate) > 0 {
							stmt, err = conn.Prepare(sel_tbl + ` ORDER BY ` + sel_prefix + clm_names[idate[0]])
						} else {
							stmt, err = conn.Prepare(sel_tbl)
						}
					} else {
						stmt, err = conn.Prepare(*qry)
					}
					if err != nil {
						log.Fatalf("failed to select data from table: %v", err)
					}
					defer stmt.Close()

					clm_names := stmt.ColumnNames()
					if err != nil {
						panic(err)
					}
					if *debug {
						log.Println("cols:", clm_names)
					}

					//var long, lat, alt, date []string
					var ilong, ilat, ialt, idate, idate_top []int
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
							switch {
							case strings.HasSuffix(lcol, "creationdate"):
								idate_top = append(idate_top, i)
							case strings.HasSuffix(lcol, "startdate"):
								idate_top = append([]int{i}, idate_top...)
							default:
								//date = append(date, col)
								idate = append(idate, i)
							}
						}
					}

					idate = append(idate_top, idate...)

					if !(len(ilong) > 0 && len(ilat) > 0) {
						if *debug {
							log.Println("Missing lat or long in file")
						}
						return
					}

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

						// Use Scan to access column data from a row
						data := make([]interface{}, len(clm_names))
						var ptr_data []interface{}
						for i := range data {
							ptr_data = append(ptr_data, &data[i])
						}
						err = stmt.Scan(ptr_data...)
						desc := fmt.Sprintf("i: %d", count)
						for icol, clm_name := range clm_names {
							strB, _ := json.Marshal(data[icol])
							desc += ",\n" + clm_name + ": " + string(strB)
						}

						var kml_coord kml.Coordinate
						var cur_time float64
						var c_time time.Time

						if len(idate) > 0 {
							cur_time, _, _ = stmt.ColumnDouble(idate[0])
							c_sec, c_dec := math.Modf(cur_time)
							c_time = time.Unix(int64(c_sec)+978307200, int64(c_dec+1e9))
							if len(path_time) > 0 && c_time.Sub(path_time[len(path_time)-1]) > *event {
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
						if *debug {
							log.Println("point: ", kml_coord, "@", cur_time, "/", c_time)
						}
						kml_coords = append(kml_coords, kml_coord)
						kml_desc = append(kml_desc, kml.Description(desc))
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

					if len(path_time) > 0 {
						store_event()
					}

					tableFolders = append(tableFolders, kml.Folder(
						append([]kml.Element{
							kml.Name(fmt.Sprintf("%s (%d)", tbl_name, count)),
							kml.Open(false),
						},
							eventFolders...,
						)...,
					))
					eventFolders = []kml.Element{}

				}()
			}
			if *qry == "" {
				sqlfileFolders = append(sqlfileFolders, kml.Folder(
					append([]kml.Element{
						kml.Name(f),
						kml.Open(*open_file),
					},
						tableFolders...,
					)...,
				))
			} else {
				sqlfileFolders = tableFolders
			}
		}()
	}

	result := kml.KML(
		kml.Document(
			append([]kml.Element{
				kml.Name(*name),
				kml.Description("Built using sqlite2kml, https://github.com/pschou/sqlite2kml"),
				kml.Open(true),
			},
				sqlfileFolders...,
			)...,
		))

	result.WriteIndent(os.Stdout, "", "  ")
}
