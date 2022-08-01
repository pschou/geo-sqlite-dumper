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
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
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
		fmt.Fprintf(os.Stderr, "geo-sqlite-dumper - Tool to view the contents of a geo sqlite file (github.com/pschou/geo-sqlite-dumper)\n"+
			"Apache 2.0 license, provided AS-IS -- not responsible for loss.\nUsage implies agreement.  Version: %s\n\n"+
			"Usage: %s [options...] [files...]\n\n", version, os.Args[0])
		params.PrintDefaults()
	}
	debug = params.Pres("debug", "Verbose output")
	event_time := params.Duration("e event-time", 2*time.Hour, "Event qualifier, time between events to split on", "TIME")
	event_bool := params.Pres("E show-event-lines", "Show event lines for a series of points within event-time")
	busy_timeout := params.Duration("timeout", 10*time.Second, "Busy timeout for SQLite calls", "TIME")
	qry := params.String("q query", "", "Custom query for SQLite", "SQL")
	file_list := params.String("list", "", "File with list of files to process, one line per file", "FILE")

	params.GroupingSet("KML")
	name := params.String("N name", "geo-sqlite-dumper", "Name to use for base KML folder", "TEXT")
	kml_file := params.String("kml", "", "Export to KML file", "FILENAME")
	params.GroupingSet("CSV")
	csv_file := params.String("csv", "", "Export to CSV file", "FILENAME")
	delimiter := params.String("delimiter", ",", "Delimiter for CSV output", "DELIM")
	params.CommandLine.Indent = 2
	params.Parse()

	var csvf, kmlf *os.File
	if *csv_file != "" {
		var err error
		csvf, err = os.Create(*csv_file)
		if err != nil {
			panic(err)
		}
		defer csvf.Close()
	}

	if *kml_file != "" {
		var err error
		kmlf, err = os.Create(*kml_file)
		if err != nil {
			panic(err)
		}
		defer kmlf.Close()
	}

	list := params.Args()
	if *file_list != "" {
		fl, err := os.Open(*file_list)
		if err != nil {
			log.Fatalf("Error reading in list file %q, %s", *file_list, err)
		}
		scanner := bufio.NewScanner(fl)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			list = append(list, strings.TrimSpace(scanner.Text()))
		}
		fl.Close()
	}

	var sqlfileFolders []kml.Element
	var all_clm_names []string
	all_clm_names_used := make(map[string]bool)
	var all_entries []*entry

	// Loop over the file names and load them into the sqlfileFolders slice
	for _, f := range list {
		if f == "" {
			continue
		}
		func() {
			// Anonymous function to make sure the defer close will work after every file
			// is done processing

			{
				header := make([]byte, 16)
				test, err := os.Open(f)
				if err != nil {
					log.Fatalf("Unable to open file %q, err: %s\n", f, err)
				}
				n, err := test.Read(header)
				if err != nil {
					log.Fatalf("Unable to read file %q, err: %s\n", f, err)
				}
				if n == 0 {
					log.Fatalf("Empty file or unaable to read bytes in file %q\n", f)
				}
				if string(header) != "SQLite format 3\x00" {
					log.Fatalf("Header of file is not in \"SQLite format 3\", %q\n", f)
				}
				test.Close()
			}

			ef := ""
			for _, c := range []byte(f) {
				switch c {
				case '/':
					ef += "/"
				default:
					// Escape name so the sql open call will be sanitized
					ef += fmt.Sprintf("%%%x", c)
				}
			}
			// Open command reference:  https://www.sqlite.org/c3ref/open.html
			conn, err := sqlite3.Open("file:"+ef+"?mode=ro&nolock=1&immutable=1", sqlite3.OPEN_READONLY)
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
				fmt.Println("Make sure the file is in SQLite file format.", err)
				return
				//panic(err)
			}

			// Declare all the variables for the tables in the file
			var total_dist, total_alt float64
			var tableFolders, eventFolders []kml.Element
			var entries []*entry
			var prev_kml_coord *kml.Coordinate
			var tbl_name string

			// Function to take the values collected and store them into a KML element
			store_event := func() {
				defer func() {
					// When this function is returned, clear out the variables
					total_dist, total_alt = 0, 0
					prev_kml_coord = nil
					entries = []*entry{}
				}()

				if len(entries) == 0 {
					return
				}
				total_pts := float64(len(entries))
				s_time := entries[0].time
				e_time := entries[len(entries)-1].time
				if *debug {
					log.Println("storing event", s_time, e_time)
				}

				var elements []kml.Element
				altMode := kml.AltitudeModeAbsolute
				if total_alt == 0 {
					altMode = kml.AltitudeModeClampToGround
				}
				// Create a path if more than one point is specified
				if *event_bool && len(entries) > 1 && !strings.HasSuffix(tbl_name, "OFINTERESTMO") {
					elements = append(elements,
						kml.Placemark(
							kml.Name("Path"),
							kml.StyleURL("#yellowLineGreenPoly"),
							kml.LineString(
								kml.Extrude(true),
								kml.Tessellate(true),
								kml.AltitudeMode(altMode),
								kml.Coordinates(coords(entries)...)),
						),
					)
				}

				var pointElements []kml.Element
				for _, entry := range entries {
					if entry.coords != nil {
						// Set the point title to the date
						title := entry.time.Format(time.RFC3339Nano)
						// If Z_PK exists, use that instead
						if v, ok := entry.data["Z_PK"]; ok {
							title = v
						}

						pointElements = append(pointElements,
							kml.Placemark(
								kml.Name(title),
								entry.desc,
								kml.Point(kml.Coordinates(*entry.coords)),
							),
						)
					}
				}
				elements = append(elements, kml.Folder(
					append([]kml.Element{
						kml.Name("Points"),
					},
						pointElements...,
					)...,
				))

				details := []kml.Element{}

				if e_time.Sub(s_time) > 0 {
					details = append(details,
						kml.Name(fmt.Sprintf("Event (%d) %s - %s", len(entries), s_time.Format(time.RFC3339Nano), e_time.Format(time.RFC3339Nano))),
					)
				} else {
					details = append(details,
						kml.Name(fmt.Sprintf("Event (%d) %s", len(entries), s_time.Format(time.RFC3339Nano))))
				}

				if len(entries) > 1 {
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
					entries = []*entry{}
					desc_top := ""

					if *debug {
						log.Println("Table", tbl_name)
					}

					joined := false
					var stmt *sqlite3.Stmt
					if *qry == "" {
						clm_names, err := getColumns(conn, tbl_name)
						if err != nil {
							panic(err)
						}

						var idate, idate_top []int
						for i, col := range clm_names {
							lcol := strings.ToLower(col)
							if strings.HasSuffix(lcol, "date") || strings.HasSuffix(lcol, "timestamp") {
								switch {
								case strings.HasSuffix(lcol, "entrydate"):
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

						if strings.HasSuffix(tbl_name, "TRANSITIONMO") && contains(tbl_names, strings.TrimSuffix(tbl_name, "TRANSITIONMO")+"MO") {
							join_tbl := strings.TrimSuffix(tbl_name, "TRANSITIONMO") + "MO"
							desc_top = "Table " + tbl_name + " left joined with " + join_tbl + "\n"
							sel_join_tbl := `SELECT * FROM ` + tbl_name + ` AS a LEFT JOIN ` + join_tbl + ` AS b ON a.ZLOCATIONOFINTEREST = b.Z_PK`
							joined = true

							// Prepare an SQL statement for data parsing with join operation
							if len(idate) > 0 {
								stmt, err = conn.Prepare(sel_join_tbl + ` ORDER BY a.` + clm_names[idate[0]])
							} else {
								stmt, err = conn.Prepare(sel_join_tbl)
							}
							if err != nil {
								// Clear out statement on error
								joined = false
								stmt = nil
							}
						}

						// Build the SQL statement if the join did not apply
						if stmt == nil {
							// Prepare an SQL statement for data parsing
							if len(idate) > 0 {
								stmt, err = conn.Prepare(sel_tbl + ` ORDER BY ` + clm_names[idate[0]])
							} else {
								stmt, err = conn.Prepare(sel_tbl)
							}
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
					for i, clm_name := range clm_names {
						lcol := strings.ToLower(clm_name)
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
							case strings.HasSuffix(lcol, "entrydate"):
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
						if desc_top != "" {
							desc = desc_top + desc
						}

						// Load the data into memory with a data_map for csv file and
						// populate the description for the kml file
						data_map := make(map[string]string)

						{ // Store the file path in the csv output
							data_map["SOURCE_FILE_PATH"] = fmt.Sprintf("%q", f)
							if !contains(all_clm_names, "SOURCE_FILE_PATH") {
								all_clm_names = append(all_clm_names, "SOURCE_FILE_PATH")
								all_clm_names_used["SOURCE_FILE_PATH"] = true
							}
						}

						{ // Store the table name in the csv output
							data_map["SOURCE_TABLE"] = fmt.Sprintf("%q", tbl_name)
							if !contains(all_clm_names, "SOURCE_TABLE") {
								all_clm_names = append(all_clm_names, "SOURCE_TABLE")
								all_clm_names_used["SOURCE_TABLE"] = true
							}
						}

						for icol, clm_name := range clm_names {
							if data[icol] == nil {
								continue
							}

							// fill up the all_clm_names for csv headers
							if !contains(all_clm_names, clm_name) {
								all_clm_names = append(all_clm_names, clm_name)
							}

							if _, ok := data_map[clm_name]; ok {
								// don't overwrite values, useful when two tables are left joined
								continue
							}
							// ensure the column is exported in csv output
							all_clm_names_used[clm_name] = true
							var data_str, data_suffix string
							switch val := data[icol].(type) {
							case int, int64:
								data_str = fmt.Sprintf("%d", val)
							case float64:
								if strings.HasSuffix(strings.ToLower(clm_name), "date") ||
									strings.HasSuffix(strings.ToLower(clm_name), "timestamp") {
									v_sec, v_dec := math.Modf(val)
									v_time := time.Unix(int64(v_sec)+978307200, int64(v_dec+1e9)).UTC()
									data_suffix = fmt.Sprintf(" (%s)", v_time)
									data_map[clm_name+"_PARSED"] = v_time.Format("2006-01-02 15:04:05")
									if !contains(all_clm_names, clm_name+"_PARSED") {
										all_clm_names = append(all_clm_names, clm_name+"_PARSED")
										all_clm_names_used[clm_name+"_PARSED"] = true
									}
								}
								data_str = fmt.Sprintf("%f", val)
							case string:
								data_str = strconv.Quote(val)
							case []uint8:
								data_str = strconv.Quote(string(val))
							default:
								if *debug {
									fmt.Printf("%s type: %T %q\n", clm_name, val, val)
								}
								strB, _ := json.Marshal(val)
								data_str = string(strB)
							}
							// put the value in the datamap
							data_map[clm_name] = data_str
							// put the value in the kml description blob
							desc += ",\n" + clm_name + ": " + data_str + data_suffix
						}

						var kml_coord *kml.Coordinate
						var cur_time float64
						var c_time time.Time

						if len(idate) > 0 {
							cur_time, _, _ = stmt.ColumnDouble(idate[0])
							c_sec, c_dec := math.Modf(cur_time)
							c_time = time.Unix(int64(c_sec)+978307200, int64(c_dec+1e9)).UTC()
							if len(entries) > 0 && c_time.Sub(entries[len(entries)-1].time) > *event_time {
								store_event()
							}
						}

						loc_ok := true

						lat, ok, err := stmt.ColumnDouble(ilat[0])
						loc_ok = loc_ok && ok && err == nil
						if *debug && (!ok || err != nil) {
							log.Println("nil long", err)
						}

						long, ok, err := stmt.ColumnDouble(ilong[0])
						loc_ok = loc_ok && ok && err == nil
						if *debug && (!ok || err != nil) {
							log.Println("nil lat", err)
						}

						if loc_ok {
							if len(ialt) > 0 {
								alt, ok, err := stmt.ColumnDouble(ilat[0])
								if *debug && (!ok || err != nil) {
									log.Println("nil alt", err)
								}
								kml_coord = &kml.Coordinate{
									Lon: long,
									Lat: lat,
									Alt: alt,
								}
								total_alt += alt
							} else {
								kml_coord = &kml.Coordinate{
									Lon: long,
									Lat: lat,
									Alt: 0,
								}
							}
						}
						if *debug {
							log.Println("point: ", kml_coord, "@", cur_time, "/", c_time)
						}

						count := -1
						if i, ok := find(tbl_names, "ZDATAPOINTCOUNT"); ok {
							if val, ok, _ := stmt.ColumnInt(i); ok {
								count = val
							}
						}
						id := -1
						if i, ok := find(tbl_names, "Z_PK"); ok {
							if val, ok, _ := stmt.ColumnInt(i); ok {
								id = val
							}
						}

						c_entry := entry{
							time:   c_time,
							desc:   kml.Description(desc),
							coords: kml_coord,
							id:     id,
							count:  count,
							data:   data_map,
						}
						entries = append(entries, &c_entry)
						if !joined {
							all_entries = append(all_entries, &c_entry)
						}
						if prev_kml_coord != nil && kml_coord != nil {
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
						prev_kml_coord = kml_coord

						if err != nil {
							log.Fatalf("scan failed while querying data: %v", err)
						}
					}

					if len(entries) > 0 {
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
						kml.Open(false),
					},
						tableFolders...,
					)...,
				))
			} else {
				sqlfileFolders = tableFolders
			}
			tableFolders = []kml.Element{}
		}()
	}

	// Write out KML
	if kmlf != nil {
		result := kml.KML(
			kml.Document(
				append([]kml.Element{
					kml.Name(*name),
					kml.Description("Built using geo-sqlite-dumper, https://github.com/pschou/geo-sqlite-dumper"),
					kml.Open(true),
				},
					sqlfileFolders...,
				)...,
			))
		result.WriteIndent(kmlf, "", "  ")
	}

	// Write out CSV
	if csvf != nil {
		co := bufio.NewWriter(csvf)
		for i, clm_name := range all_clm_names {
			if a, b := all_clm_names_used[clm_name]; !(a && b) {
				continue
			}
			if i > 0 {
				co.Write([]byte(*delimiter))
			}
			fmt.Fprintf(co, "%q", clm_name)
		}
		co.Write([]byte{'\n'})
		for _, e := range all_entries {
			for i, clm_name := range all_clm_names {
				if a, b := all_clm_names_used[clm_name]; !(a && b) {
					continue
				}
				if i > 0 {
					co.Write([]byte(*delimiter))
				}
				if edat, ok := e.data[clm_name]; ok {
					fmt.Fprintf(co, "%q", edat)
				}
			}
			co.Write([]byte{'\n'})
		}
	}
}
