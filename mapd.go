package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
)

type State struct {
	Data       []uint8
	CurrentWay CurrentWay
	NextWay    NextWayResult
	Position   Position
}

type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Bearing   float64 `json:"bearing"`
}

type NextSpeedLimit struct {
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Speedlimit float64 `json:"speedlimit"`
}

type AdvisoryLimit struct {
	StartLatitude  float64 `json:"start_latitude"`
	StartLongitude float64 `json:"start_longitude"`
	EndLatitude    float64 `json:"end_latitude"`
	EndLongitude   float64 `json:"end_longitude"`
	Speedlimit     float64 `json:"speedlimit"`
}

type Hazard struct {
	StartLatitude  float64 `json:"start_latitude"`
	StartLongitude float64 `json:"start_longitude"`
	EndLatitude    float64 `json:"end_latitude"`
	EndLongitude   float64 `json:"end_longitude"`
	Hazard         string  `json:"hazard"`
}

func RoadName(way Way) string {
	name, err := way.Name()
	if err == nil {
		if len(name) > 0 {
			return name
		}
	}
	ref, err := way.Ref()
	if err == nil {
		if len(ref) > 0 {
			return ref
		}
	}
	return ""
}

func readOffline(data []uint8) Offline {
	msg, err := capnp.UnmarshalPacked(data)
	loge(err)
	if err == nil {
		offline, err := ReadRootOffline(msg)
		loge(err)
		return offline
	}
	return Offline{}
}

func readPosition(persistent bool) (Position, error) {
	path := LAST_GPS_POSITION
	if persistent {
		path = LAST_GPS_POSITION_PERSIST
	}

	pos := Position{}
	coordinates, err := GetParam(path)
	if err != nil {
		return pos, err
	}
	err = json.Unmarshal(coordinates, &pos)
	return pos, err
}

func main() {
	generatePtr := flag.Bool("generate", false, "Triggers a generation of map data from 'map.osm.pbf'")
	minGenLatPtr := flag.Int("minlat", -90, "the minimum latitude to generate")
	minGenLonPtr := flag.Int("minlon", -180, "the minimum longitude to generate")
	maxGenLatPtr := flag.Int("maxlat", -90, "the maximum latitude to generate")
	maxGenLonPtr := flag.Int("maxlon", -180, "the maximum longitude to generate")
	flag.Parse()
	if *generatePtr {
		GenerateOffline(*minGenLatPtr, *minGenLonPtr, *maxGenLatPtr, *maxGenLonPtr)
		return
	}
	EnsureParamDirectories()
	EnsureParamsExist()
	state := State{}

	pos, err := readPosition(true)
	loge(err)
	if err == nil {
		state.Data, err = FindWaysAroundLocation(pos.Latitude, pos.Longitude)
		loge(err)
	}

	for {
		time.Sleep(1 * time.Second)
		DownloadIfTriggered()

		pos, err := readPosition(false)
		if err != nil {
			loge(err)
			continue
		}
		offline := readOffline(state.Data)

		if !PointInBox(pos.Latitude, pos.Longitude, offline.MinLat(), offline.MinLon(), offline.MaxLat(), offline.MaxLon()) {
			state.CurrentWay = CurrentWay{}
			state.NextWay = NextWayResult{}
			state.Data, err = FindWaysAroundLocation(pos.Latitude, pos.Longitude)
			loge(err)
		}

		state.CurrentWay, err = GetCurrentWay(state.CurrentWay.Way, state.NextWay.Way, offline, pos)
		loge(err)

		state.NextWay, err = NextWay(state.CurrentWay, offline, pos.Latitude, pos.Longitude, pos.Bearing)
		loge(err)

		err = PutParam(ROAD_NAME, []byte(RoadName(state.CurrentWay.Way)))
		loge(err)

		err = PutParam(MAP_SPEED_LIMIT, []byte(fmt.Sprintf("%f", state.CurrentWay.Way.MaxSpeed())))
		loge(err)

		err = PutParam(MAP_ADVISORY_LIMIT, []byte(fmt.Sprintf("%f", state.CurrentWay.Way.AdvisorySpeed())))
		loge(err)

		hazard, err := state.CurrentWay.Way.Hazard()
		loge(err)
		data, err := json.Marshal(Hazard{
			StartLatitude:  state.CurrentWay.StartPosition.Latitude(),
			StartLongitude: state.CurrentWay.StartPosition.Longitude(),
			EndLatitude:    state.CurrentWay.EndPosition.Latitude(),
			EndLongitude:   state.CurrentWay.EndPosition.Longitude(),
			Hazard:         hazard,
		})
		loge(err)
		err = PutParam(MAP_HAZARD, data)
		loge(err)

		hazard, err = state.NextWay.Way.Hazard()
		loge(err)
		data, err = json.Marshal(Hazard{
			StartLatitude:  state.NextWay.StartPosition.Latitude(),
			StartLongitude: state.NextWay.StartPosition.Longitude(),
			EndLatitude:    state.NextWay.EndPosition.Latitude(),
			EndLongitude:   state.NextWay.EndPosition.Longitude(),
			Hazard:         hazard,
		})
		loge(err)
		err = PutParam(NEXT_MAP_HAZARD, data)
		loge(err)

		data, err = json.Marshal(AdvisoryLimit{
			StartLatitude:  state.CurrentWay.StartPosition.Latitude(),
			StartLongitude: state.CurrentWay.StartPosition.Longitude(),
			EndLatitude:    state.CurrentWay.EndPosition.Latitude(),
			EndLongitude:   state.CurrentWay.EndPosition.Longitude(),
			Speedlimit:     state.CurrentWay.Way.AdvisorySpeed(),
		})
		loge(err)
		err = PutParam(MAP_ADVISORY_LIMIT, data)
		loge(err)

		data, err = json.Marshal(NextSpeedLimit{
			Latitude:   state.NextWay.StartPosition.Latitude(),
			Longitude:  state.NextWay.StartPosition.Longitude(),
			Speedlimit: state.NextWay.Way.MaxSpeed(),
		})
		loge(err)
		err = PutParam(NEXT_MAP_SPEED_LIMIT, data)
		loge(err)

		data, err = json.Marshal(AdvisoryLimit{
			StartLatitude:  state.NextWay.StartPosition.Latitude(),
			StartLongitude: state.NextWay.StartPosition.Longitude(),
			EndLatitude:    state.NextWay.EndPosition.Latitude(),
			EndLongitude:   state.NextWay.EndPosition.Longitude(),
			Speedlimit:     state.NextWay.Way.AdvisorySpeed(),
		})
		loge(err)
		err = PutParam(NEXT_MAP_ADVISORY_LIMIT, data)
		loge(err)
	}
}
