package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
)

// contains reports whether ids includes id.
func contains(ids []uint64, id uint64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestSpatialIndex_RadiusQueryFindsNeighbors(t *testing.T) {
	idx := NewSpatialIndex(100, 80, 10, false) // w,h,cell,torus
	idx.Insert(1, common.Point{X: 10, Y: 10})
	idx.Insert(2, common.Point{X: 13, Y: 12})
	idx.Insert(3, common.Point{X: 90, Y: 70})
	got := idx.Within(common.Point{X: 10, Y: 10}, 5) // ids within radius 5
	if !contains(got, 2) || contains(got, 3) {
		t.Fatalf("Within=%v want {2} (and not 3)", got)
	}
}

func TestSpatialIndex_TorusWrapsDistance(t *testing.T) {
	idx := NewSpatialIndex(100, 80, 10, true)
	idx.Insert(1, common.Point{X: 1, Y: 1})
	idx.Insert(2, common.Point{X: 99, Y: 1})
	got := idx.Within(common.Point{X: 1, Y: 1}, 5) // wraps: distance to x=99 is 2
	if !contains(got, 2) {
		t.Fatalf("torus Within=%v want to include 2", got)
	}
}

func TestSpatialIndex_NearestReturnsClosestFirst(t *testing.T) {
	idx := NewSpatialIndex(100, 80, 10, false)
	idx.Insert(1, common.Point{X: 10, Y: 10})
	idx.Insert(2, common.Point{X: 13, Y: 12})
	idx.Insert(3, common.Point{X: 90, Y: 70})
	got := idx.Nearest(common.Point{X: 11, Y: 11}, 2)
	if len(got) != 2 {
		t.Fatalf("Nearest len=%d want 2", len(got))
	}
	if got[0] != 1 && got[0] != 2 {
		t.Fatalf("Nearest first=%d want a near neighbor (1 or 2)", got[0])
	}
	if contains(got, 3) {
		t.Fatalf("Nearest=%v should not include far agent 3", got)
	}
}

func TestIntegrateMovement_ClampsAndWraps(t *testing.T) {
	// non-torus: clamp to bounds
	p := IntegrateMovement(common.Point{X: 98, Y: 40}, Vector{DX: 5, DY: 0}, 100, 80, false)
	if p.X != 100 {
		t.Fatalf("clamp X=%v want 100", p.X)
	}
	// torus: wrap
	p = IntegrateMovement(common.Point{X: 98, Y: 40}, Vector{DX: 5, DY: 0}, 100, 80, true)
	if p.X != 3 {
		t.Fatalf("wrap X=%v want 3", p.X)
	}
}

func TestPointInShape_Polygon(t *testing.T) {
	poly := []common.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}
	if !PointInPolygon(common.Point{X: 5, Y: 5}, poly) {
		t.Fatal("center should be inside")
	}
	if PointInPolygon(common.Point{X: 15, Y: 5}, poly) {
		t.Fatal("outside should be outside")
	}
}

func TestDistance_TorusAware(t *testing.T) {
	// non-torus straight line distance
	d := Distance(common.Point{X: 0, Y: 0}, common.Point{X: 3, Y: 4}, 100, 80, false)
	if d != 5 {
		t.Fatalf("Distance=%v want 5", d)
	}
	// torus wraps: 1 -> 99 on width 100 is distance 2, not 98
	d = Distance(common.Point{X: 1, Y: 1}, common.Point{X: 99, Y: 1}, 100, 80, true)
	if d != 2 {
		t.Fatalf("torus Distance=%v want 2", d)
	}
}

func TestVectorToward_PointsAtTargetTorusAware(t *testing.T) {
	// non-torus: toward a target to the right => positive DX
	v := VectorToward(common.Point{X: 0, Y: 0}, common.Point{X: 10, Y: 0}, 100, 80, false, 1)
	if v.DX <= 0 {
		t.Fatalf("VectorToward DX=%v want positive", v.DX)
	}
	// torus: from x=1, target x=99 is shorter going negative (wrap) => negative DX
	v = VectorToward(common.Point{X: 1, Y: 1}, common.Point{X: 99, Y: 1}, 100, 80, true, 1)
	if v.DX >= 0 {
		t.Fatalf("torus VectorToward DX=%v want negative (wrap)", v.DX)
	}
}

func TestPointInCircleAndRect(t *testing.T) {
	if !PointInCircle(common.Point{X: 5, Y: 5}, common.Point{X: 5, Y: 5}, 3) {
		t.Fatal("center should be in circle")
	}
	if PointInCircle(common.Point{X: 20, Y: 20}, common.Point{X: 5, Y: 5}, 3) {
		t.Fatal("far point should be outside circle")
	}
	if !PointInRect(common.Point{X: 5, Y: 5}, common.Point{X: 0, Y: 0}, 10, 10) {
		t.Fatal("inside point should be in rect")
	}
	if PointInRect(common.Point{X: 15, Y: 5}, common.Point{X: 0, Y: 0}, 10, 10) {
		t.Fatal("outside point should be outside rect")
	}
}
