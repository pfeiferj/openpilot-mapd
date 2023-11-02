package main

import (
	"cmp"
	"errors"
	"math"
	"slices"
)

func OnWay(way Way, lat float64, lon float64) (bool, Coordinates, Coordinates, error) {

	if lat < float64(way.MaxLat())+PADDING && lat > float64(way.MinLat())-PADDING && lon < float64(way.MaxLon())+PADDING && lon > float64(way.MinLon())-PADDING {
		d, nodeStart, nodeEnd, err := DistanceToWay(lat, lon, way)
		if err != nil {
			return false, nodeStart, nodeEnd, err
		}
		lanes := way.Lanes()
		if lanes == 0 {
			lanes = 2
		}
		road_width_estimate := float64(lanes) * LANE_WIDTH
		max_dist := 5 + road_width_estimate

		if d < max_dist {
			return true, nodeStart, nodeEnd, nil
		}
	}
	return false, Coordinates{}, Coordinates{}, nil
}

func DistanceToWay(lat float64, lon float64, way Way) (float64, Coordinates, Coordinates, error) {
	var minNodeStart Coordinates
	var minNodeEnd Coordinates
	minDistance := math.MaxFloat64
	nodes, err := way.Nodes()
	if err != nil {
		return minDistance, minNodeStart, minNodeEnd, err
	}
	if nodes.Len() < 2 {
		return minDistance, minNodeStart, minNodeEnd, nil
	}

	latRad := lat * TO_RADIANS
	lonRad := lon * TO_RADIANS
	for i := 0; i < nodes.Len()-1; i++ {
		nodeStart := nodes.At(i)
		nodeEnd := nodes.At(i + 1)
		lineLat, lineLon := PointOnLine(float64(nodeStart.Latitude()), float64(nodeStart.Longitude()), float64(nodeEnd.Latitude()), float64(nodeEnd.Longitude()), lat, lon)
		distance := DistanceToPoint(latRad, lonRad, lineLat*TO_RADIANS, lineLon*TO_RADIANS)
		if distance < minDistance {
			minDistance = distance
			minNodeStart = nodeStart
			minNodeEnd = nodeEnd
		}
	}
	return minDistance, minNodeStart, minNodeEnd, nil
}

type CurrentWay struct {
	Way       Way
	StartNode Coordinates
	EndNode   Coordinates
}

func GetCurrentWay(state *State, lat float64, lon float64) (CurrentWay, error) {
	if state.Way.Way.HasNodes() {
		onWay, nodeStart, nodeEnd, err := OnWay(state.Way.Way, lat, lon)
		loge(err)
		if onWay {
			return CurrentWay{
				Way:       state.Way.Way,
				StartNode: nodeStart,
				EndNode:   nodeEnd,
			}, nil
		}
	}

	// check ways that have the same name/ref
	for _, way := range state.MatchingWays {
		onWay, nodeStart, nodeEnd, err := OnWay(way, lat, lon)
		loge(err)
		if onWay {
			return CurrentWay{
				Way:       way,
				StartNode: nodeStart,
				EndNode:   nodeEnd,
			}, nil
		}
	}

	// finally check all other ways
	ways, err := state.Result.Ways()
	if err != nil {
		return CurrentWay{}, err
	}
	for i := 0; i < ways.Len(); i++ {
		way := ways.At(i)
		onWay, nodeStart, nodeEnd, err := OnWay(way, lat, lon)
		loge(err)
		if onWay {
			return CurrentWay{
				Way:       way,
				StartNode: nodeStart,
				EndNode:   nodeEnd,
			}, nil
		}
	}

	return CurrentWay{}, errors.New("Could not find way")
}

func MatchingWays(state *State) ([]Way, Coordinates, error) {
	matchingWays := []Way{}
	nodes, err := state.Way.Way.Nodes()
	if err != nil {
		return matchingWays, Coordinates{}, err
	}
	if !state.Way.Way.HasNodes() || nodes.Len() == 0 {
		return matchingWays, Coordinates{}, nil
	}

	wayBearing := Bearing(float64(state.Way.StartNode.Latitude()), float64(state.Way.StartNode.Longitude()), float64(state.Way.EndNode.Latitude()), float64(state.Way.EndNode.Longitude()))
	bearingDelta := math.Abs((state.Position.Bearing * TO_RADIANS) - wayBearing)
	isForward := math.Cos(bearingDelta) >= 0
	var matchNode Coordinates
	if isForward {
		matchNode = nodes.At(nodes.Len() - 1)
	} else {
		matchNode = nodes.At(0)
	}

	ways, err := state.Result.Ways()
	if err != nil {
		return matchingWays, matchNode, err
	}
	for i := 0; i < ways.Len(); i++ {
		w := ways.At(i)
		if !w.HasNodes() {
			continue
		}
		wNodes, err := w.Nodes()
		if err != nil {
			return matchingWays, matchNode, err
		}
		if wNodes.Len() < 2 {
			continue
		}
		fNode := wNodes.At(0)
		lNode := wNodes.At(wNodes.Len() - 1)
		if fNode == matchNode || lNode == matchNode {
			matchingWays = append(matchingWays, w)
		}
	}

	name, _ := state.Way.Way.Name()
	ref, _ := state.Way.Way.Ref()
	sortMatchingWays := func(a, b Way) int {
		aVal := float64(1000)
		bVal := float64(1000)
		if len(name) > 0 {
			an, _ := a.Name()
			bn, _ := b.Name()
			if an == name {
				aVal = -1000
			}
			if bn == name {
				bVal = -1000
			}
		} else if len(ref) > 0 {
			ar, _ := a.Ref()
			br, _ := b.Ref()
			if ar == ref {
				aVal = -1000
			}
			if br == name {
				bVal = -1000
			}
		} else {
			var aBearingNode Coordinates
			aNodes, err := a.Nodes()
			if err != nil {
				return cmp.Compare(aVal, bVal)
			}
			if matchNode == aNodes.At(0) {
				aBearingNode = aNodes.At(1)
			} else {
				aBearingNode = aNodes.At(aNodes.Len() - 2)
			}
			aBearing := Bearing(float64(matchNode.Latitude()), float64(matchNode.Longitude()), float64(aBearingNode.Latitude()), float64(aBearingNode.Longitude()))
			aVal = math.Abs((state.Position.Bearing * TO_RADIANS) - aBearing)

			var bBearingNode Coordinates
			bNodes, err := b.Nodes()
			if err != nil {
				return cmp.Compare(aVal, bVal)
			}
			if matchNode == bNodes.At(0) {
				bBearingNode = bNodes.At(1)
			} else {
				bBearingNode = bNodes.At(bNodes.Len() - 2)
			}
			bBearing := Bearing(float64(matchNode.Latitude()), float64(matchNode.Longitude()), float64(bBearingNode.Latitude()), float64(bBearingNode.Longitude()))
			bVal = math.Abs((state.Position.Bearing * TO_RADIANS) - bBearing)
		}

		return cmp.Compare(aVal, bVal)
	}
	slices.SortFunc(matchingWays, sortMatchingWays)
	return matchingWays, matchNode, nil
}
