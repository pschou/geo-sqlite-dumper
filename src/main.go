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
	"os"
	"strings"

	"github.com/alicebob/sqlittle/db"
	_ "github.com/twpayne/go-geom/encoding/kml"
)

func main() {
	for _, f := range os.Args {
		fmt.Println("Processing file", f)

		fdb, err := db.OpenFile(f)
		if err != nil {
			panic(err)
		}
		defer fdb.Close()

		tbl_names, err := fdb.Tables()
		if err != nil {
			panic(err)
		}
		for _, tbl_name := range tbl_names {
			fmt.Println("Table", tbl_name)
			schema, err := fdb.Schema(tbl_name)
			if err != nil {
				panic(err)
			}

			var long, lat, alt []db.TableColumn
			var ilong, ilat, ialt []int
			for i, col := range schema.Columns {
				switch {
				case strings.HasSuffix(strings.ToLower(col.Column), "latitude"):
					lat = append(lat, col)
					ilat = append(ilat, i)
				case strings.HasSuffix(strings.ToLower(col.Column), "longitude"):
					long = append(long, col)
					ilong = append(ilong, i)
				case strings.HasSuffix(strings.ToLower(col.Column), "altitude"):
					alt = append(alt, col)
					ialt = append(ialt, i)
				}
			}

			if len(long) == 1 && len(lat) == 1 {
				tbl, err := fdb.Table(tbl_name)
				if err != nil {
					panic(err)
				}

				switch len(alt) {
				case 0:
					tbl.Scan(func(j int64, rec db.Record) bool {
						fmt.Println("long:", rec[ilong[0]], "lat:", rec[ilat[0]])
						return false
					})
				case 1:
					tbl.Scan(func(j int64, rec db.Record) bool {
						fmt.Println("long:", rec[ilong[0]], "lat:", rec[ilat[0]], "alt:", rec[ialt[0]])
						return false
					})
				}
			}
		}
	}
}
