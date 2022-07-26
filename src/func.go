package main

import (
	"time"

	"github.com/twpayne/go-kml"
)

// contains checks if a string is present in a slice
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// find the entry in the slice with said value
func find(s []string, str string) (int, bool) {
	for i, v := range s {
		if v == str {
			return i, true
		}
	}
	return -1, false
}

type entry struct {
	coords *kml.Coordinate
	desc   *kml.SimpleElement
	data   map[string]string
	id     int
	count  int
	time   time.Time
}

// build a slice with all the coordinates
func coords(elms []*entry) (ret []kml.Coordinate) {
	for _, e := range elms {
		ret = append(ret, *e.coords)
	}
	return ret
}

var top10Count []entry
var top10Visit []entry

/*
func topCount(e []*entry, size int) []*entry {
	if len(entry) <= size {
		return e
	}

	topCount = e[:1]
topCountLoop:
	for i := 1; i < len(e); i++ {
		if len(topCount) == size && topCount[size-1].count > e[i].count {
			// short circuit when the record is smaller than our smallest
			continue
		}
		j_size := len(j)
		if j_size >= size {
			j_size = size
		}
		for j, t := range topCount {
			if t.count <= e[i].count {
				topCount = append(
					topCount[:j],
					e[i],
					topCount[j:j_size]...)
				continue topCountLoop
			}
		}
		if top10Count[smallest_count].count > top10Count[i].count {
			smallest_count = i
		}
		if top10Visit[smallest_visit].visit > top10Visit[i].count {
			smallest_count = i
		}
	}
}
*/
