/*
Copyright 2026-Present Couchbase, Inc.

Use of this software is governed by the Business Source License included in
the file licenses/BSL-Couchbase.txt.  As of the Change Date specified in that
file, in accordance with the Business Source License, use of this software will
be governed by the Apache License, Version 2.0, included in the file
licenses/APL2.txt.
*/

package cluster

import (
	"testing"
	"time"

	v2 "github.com/couchbase/couchbase-operator/pkg/apis/couchbase/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCheckNeedsRestart exercises the rolling-restart trigger logic without
// spinning up a full reconcile. It validates the three behaviours:
//   - no annotation -> no restart
//   - annotation older than the pod -> no restart
//   - annotation newer than the pod -> restart
//   - unparseable annotation -> no restart (logged, tolerated)
func TestCheckNeedsRestart(t *testing.T) {
	t.Parallel()

	podCreated := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cb-0",
			CreationTimestamp: metav1.NewTime(podCreated),
		},
	}

	tests := []struct {
		name       string
		clusterAnn map[string]string
		classAnn   map[string]string
		want       bool
	}{
		{name: "no annotation"},
		{
			name:       "cluster annotation before pod -> no restart",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": "2024-05-01T00:00:00Z"},
		},
		{
			name:       "cluster annotation after pod -> restart",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": "2024-07-01T00:00:00Z"},
			want:       true,
		},
		{
			name:       "per-class annotation overrides cluster",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": "2099-01-01T00:00:00Z"},
			classAnn:   map[string]string{"kubectl.kubernetes.io/restartedAt": "2024-05-01T00:00:00Z"},
			want:       false,
		},
		{
			name:     "per-class annotation triggers restart",
			classAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": "2024-07-01T00:00:00Z"},
			want:     true,
		},
		{
			name:       "unparseable value tolerated",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": "not-a-time"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverClass := &v2.ServerConfig{Name: "data"}
			if tt.classAnn != nil {
				serverClass.Pod = &v2.PodTemplate{
					ObjectMeta: v2.ObjectMeta{Annotations: tt.classAnn},
				}
			}
			c := &Cluster{
				cluster: &v2.CouchbaseCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "cb",
						Namespace:   "default",
						Annotations: tt.clusterAnn,
					},
				},
			}
			got, err := c.checkNeedsRestart(pod, serverClass)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("checkNeedsRestart = %v, want %v", got, tt.want)
			}
		})
	}
}
