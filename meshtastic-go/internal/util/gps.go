package util

import (
	"fmt"
	"math"
)

const (
	// Earth radius in meters
	EarthRadius = 6371000.0
	// Degrees to radians conversion factor
	DegToRad = math.Pi / 180.0
	// Radians to degrees conversion factor
	RadToDeg = 180.0 / math.Pi
)

// GPSCoordinate represents a GPS position
type GPSCoordinate struct {
	Latitude  float64
	Longitude float64
	Altitude  int32
}

// IsValid returns true if the coordinate has valid GPS data
func (c GPSCoordinate) IsValid() bool {
	return c.Latitude != 0 || c.Longitude != 0
}

// DistanceTo calculates the distance in meters to another coordinate using Haversine formula
func (c GPSCoordinate) DistanceTo(other GPSCoordinate) float64 {
	if !c.IsValid() || !other.IsValid() {
		return 0
	}

	lat1 := c.Latitude * DegToRad
	lat2 := other.Latitude * DegToRad
	deltaLat := (other.Latitude - c.Latitude) * DegToRad
	deltaLon := (other.Longitude - c.Longitude) * DegToRad

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	cc := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadius * cc
}

// BearingTo calculates the initial bearing in degrees to another coordinate
func (c GPSCoordinate) BearingTo(other GPSCoordinate) float64 {
	if !c.IsValid() || !other.IsValid() {
		return 0
	}

	lat1 := c.Latitude * DegToRad
	lat2 := other.Latitude * DegToRad
	deltaLon := (other.Longitude - c.Longitude) * DegToRad

	y := math.Sin(deltaLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(deltaLon)

	bearing := math.Atan2(y, x) * RadToDeg

	// Normalize to 0-360
	bearing = math.Mod(bearing+360, 360)

	return bearing
}

// CalculateDistance calculates distance in meters between two lat/lon pairs
func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	c1 := GPSCoordinate{Latitude: lat1, Longitude: lon1}
	c2 := GPSCoordinate{Latitude: lat2, Longitude: lon2}
	return c1.DistanceTo(c2)
}

// CalculateBearing calculates bearing in degrees from point 1 to point 2
func CalculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	c1 := GPSCoordinate{Latitude: lat1, Longitude: lon1}
	c2 := GPSCoordinate{Latitude: lat2, Longitude: lon2}
	return c1.BearingTo(c2)
}

// FormatDistance formats a distance in meters to a human-readable string
func FormatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("%.0f m", meters)
	}
	return fmt.Sprintf("%.1f km", meters/1000)
}

// FormatDistanceImperial formats a distance in meters to imperial units
func FormatDistanceImperial(meters float64) string {
	feet := meters * 3.28084
	if feet < 5280 {
		return fmt.Sprintf("%.0f ft", feet)
	}
	return fmt.Sprintf("%.1f mi", feet/5280)
}

// BearingToCardinal converts a bearing in degrees to a cardinal direction
func BearingToCardinal(bearing float64) string {
	directions := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	index := int(math.Round(bearing/22.5)) % 16
	return directions[index]
}

// BearingToCardinal8 converts a bearing to 8-point cardinal direction
func BearingToCardinal8(bearing float64) string {
	directions := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	index := int(math.Round(bearing/45)) % 8
	return directions[index]
}

// FormatCoordinateDMS formats a coordinate as Degrees Minutes Seconds
func FormatCoordinateDMS(lat, lon float64) string {
	latDir := "N"
	if lat < 0 {
		latDir = "S"
		lat = -lat
	}
	lonDir := "E"
	if lon < 0 {
		lonDir = "W"
		lon = -lon
	}

	latDeg := int(lat)
	latMin := int((lat - float64(latDeg)) * 60)
	latSec := (lat - float64(latDeg) - float64(latMin)/60) * 3600

	lonDeg := int(lon)
	lonMin := int((lon - float64(lonDeg)) * 60)
	lonSec := (lon - float64(lonDeg) - float64(lonMin)/60) * 3600

	return fmt.Sprintf("%d°%d'%.1f\"%s %d°%d'%.1f\"%s",
		latDeg, latMin, latSec, latDir,
		lonDeg, lonMin, lonSec, lonDir)
}

// FormatCoordinateDecimal formats a coordinate as decimal degrees
func FormatCoordinateDecimal(lat, lon float64) string {
	return fmt.Sprintf("%.6f, %.6f", lat, lon)
}

// FormatCoordinateDM formats a coordinate as Degrees Decimal Minutes
func FormatCoordinateDM(lat, lon float64) string {
	latDir := "N"
	if lat < 0 {
		latDir = "S"
		lat = -lat
	}
	lonDir := "E"
	if lon < 0 {
		lonDir = "W"
		lon = -lon
	}

	latDeg := int(lat)
	latMin := (lat - float64(latDeg)) * 60

	lonDeg := int(lon)
	lonMin := (lon - float64(lonDeg)) * 60

	return fmt.Sprintf("%d°%.4f'%s %d°%.4f'%s",
		latDeg, latMin, latDir,
		lonDeg, lonMin, lonDir)
}

// PositionInfo contains calculated position information relative to another node
type PositionInfo struct {
	DistanceMeters float64 `json:"distanceMeters"`
	DistanceText   string  `json:"distanceText"`
	Bearing        float64 `json:"bearing"`
	Cardinal       string  `json:"cardinal"`
	CoordDMS       string  `json:"coordDMS"`
	CoordDecimal   string  `json:"coordDecimal"`
}

// CalculatePositionInfo calculates position info for a coordinate relative to a reference
func CalculatePositionInfo(lat, lon float64, refLat, refLon float64) *PositionInfo {
	if (lat == 0 && lon == 0) {
		return nil
	}

	info := &PositionInfo{
		CoordDMS:     FormatCoordinateDMS(lat, lon),
		CoordDecimal: FormatCoordinateDecimal(lat, lon),
	}

	// Calculate distance and bearing if reference is valid
	if refLat != 0 || refLon != 0 {
		info.DistanceMeters = CalculateDistance(refLat, refLon, lat, lon)
		info.DistanceText = FormatDistance(info.DistanceMeters)
		info.Bearing = CalculateBearing(refLat, refLon, lat, lon)
		info.Cardinal = BearingToCardinal8(info.Bearing)
	}

	return info
}
