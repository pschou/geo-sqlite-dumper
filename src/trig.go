package main

import "math"

// degreesToRadians converts from degrees to radians.
func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180
}

// Distance calculates the shortest path between two coordinates on the surface
// of the Earth. This function returns two units of measure, the first is the
// distance in miles, the second is the distance in kilometers.
func ArcDistance(Lat1, Lon1, Lat2, Lon2 float64) float64 {
	lat1 := degreesToRadians(Lat1)
	lon1 := degreesToRadians(Lon1)
	lat2 := degreesToRadians(Lat2)
	lon2 := degreesToRadians(Lon2)

	diffLat := lat2 - lat1
	diffLon := lon2 - lon1

	a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*
		math.Pow(math.Sin(diffLon/2), 2)

	return 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func Sq(a float64) float64 {
	return a * a
}

// WGS 84
func EarthRadius(Lat float64) float64 {
	lat := degreesToRadians(Lat)
	s_2 := Sq(math.Sin(lat))
	c_2 := Sq(math.Cos(lat))
	r1_2 := 6378137.0 * 6378137
	r1_4 := 6378137.0 * 6378137 * 6378137 * 6378137
	r2_2 := 6356752.0 * 6356752
	r2_4 := 6356752.0 * 6356752 * 6356752 * 6356752

	//R = √ [ (r1² * cos(B))² + (r2² * sin(B))² ] / [ (r1 * cos(B))² + (r2 * sin(B))² ]
	return math.Sqrt((r1_4*c_2 + r2_4*s_2) / (r1_2*c_2 + r2_2*s_2))
}
