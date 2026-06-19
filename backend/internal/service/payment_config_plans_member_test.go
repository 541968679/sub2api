package service

import (
	"reflect"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestPlanMemberGroupIDs(t *testing.T) {
	tests := []struct {
		name string
		plan *dbent.SubscriptionPlan
		want []int64
	}{
		{"nil plan", nil, nil},
		{"single group, empty members", &dbent.SubscriptionPlan{GroupID: 5}, []int64{5}},
		{"bundle dedup + primary first", &dbent.SubscriptionPlan{GroupID: 5, MemberGroupIds: []int64{7, 5, 9, 7}}, []int64{5, 7, 9}},
		{"drop non-positive members", &dbent.SubscriptionPlan{GroupID: 5, MemberGroupIds: []int64{0, -3, 8}}, []int64{5, 8}},
		{"primary non-positive dropped", &dbent.SubscriptionPlan{GroupID: 0, MemberGroupIds: []int64{8}}, []int64{8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanMemberGroupIDs(tt.plan)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PlanMemberGroupIDs(%+v) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}
