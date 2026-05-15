/*
Copyright 2026-Present Couchbase, Inc.

Use of this software is governed by the Business Source License included in
the file licenses/BSL-Couchbase.txt.  As of the Change Date specified in that
file, in accordance with the Business Source License, use of this software will
be governed by the Apache License, Version 2.0, included in the file
licenses/APL2.txt.
*/

package e2e

import (
	"context"
	"testing"
	"time"

	couchbasev2 "github.com/couchbase/couchbase-operator/pkg/apis/couchbase/v2"
	cbconstants "github.com/couchbase/couchbase-operator/pkg/util/constants"
	"github.com/couchbase/couchbase-operator/pkg/util/jsonpatch"
	"github.com/couchbase/couchbase-operator/test/e2e/constants"
	"github.com/couchbase/couchbase-operator/test/e2e/e2eutil"
	"github.com/couchbase/couchbase-operator/test/e2e/framework"
	"github.com/couchbase/couchbase-operator/test/e2e/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRollingRestartViaAnnotation verifies that setting
// kubectl.kubernetes.io/restartedAt on a CouchbaseCluster causes every server
// pod to be recreated through the rolling-upgrade pipeline.
func TestRollingRestartViaAnnotation(t *testing.T) {
	f := framework.Global

	kubernetes, cleanup := f.SetupTest(t)
	defer cleanup()

	clusterSize := constants.Size3
	cluster := clusterOptionsUpgrade().WithEphemeralTopology(clusterSize).MustCreate(t, kubernetes)
	e2eutil.MustWaitClusterStatusHealthy(t, kubernetes, cluster, 2*time.Minute)

	preRestart := mustListServerPodCreationTimestamps(t, kubernetes, cluster)
	if len(preRestart) != clusterSize {
		t.Fatalf("expected %d server pods, found %d", clusterSize, len(preRestart))
	}

	// JSON Pointer escapes: '~' -> '~0', '/' -> '~1'. Use the constant only
	// for compile-time confirmation that the key is what we expect.
	const escapedKey = "kubectl.kubernetes.io~1restartedAt"
	if cbconstants.RestartedAtAnnotation != "kubectl.kubernetes.io/restartedAt" {
		t.Fatalf("unexpected RestartedAtAnnotation value %q", cbconstants.RestartedAtAnnotation)
	}

	restartedAt := time.Now().UTC().Format(time.RFC3339)
	patch := jsonpatch.NewPatchSet().Add("/metadata/annotations/"+escapedKey, restartedAt)
	cluster = e2eutil.MustPatchCluster(t, kubernetes, cluster, patch, time.Minute)

	// Cluster must remain healthy after rolling restart.
	e2eutil.MustWaitClusterStatusHealthy(t, kubernetes, cluster, 20*time.Minute)

	parsedRestart, err := time.Parse(time.RFC3339, restartedAt)
	if err != nil {
		t.Fatalf("parse restartedAt: %v", err)
	}

	postRestart := mustListServerPodCreationTimestamps(t, kubernetes, cluster)
	for name, ts := range postRestart {
		if !ts.After(parsedRestart) {
			t.Errorf("pod %q creationTimestamp %v not after restartedAt %v (was not recreated)", name, ts, parsedRestart)
		}
	}
}

func mustListServerPodCreationTimestamps(t *testing.T, k8s *types.Cluster, cluster *couchbasev2.CouchbaseCluster) map[string]time.Time {
	t.Helper()

	pods, err := k8s.KubeClient.CoreV1().Pods(cluster.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: constants.CouchbaseServerClusterKey + "=" + cluster.Name,
	})
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}

	out := make(map[string]time.Time, len(pods.Items))
	for _, p := range pods.Items {
		out[p.Name] = p.CreationTimestamp.Time
	}
	return out
}
