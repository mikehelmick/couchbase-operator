/*
Copyright 2024-Present Couchbase, Inc.

Use of this software is governed by the Business Source License included in
the file licenses/BSL-Couchbase.txt.  As of the Change Date specified in that
file, in accordance with the Business Source License, use of this software will
be governed by the Apache License, Version 2.0, included in the file
licenses/APL2.txt.
*/

package v2

import (
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LowVersionImage      = "couchbase/server:6.5.0"
	LowVersionHashImage  = "couchbase/server@sha256:b21765563ba510c0b1ca43bc9287567761d901b8d00fee704031e8f405bfa501"
	MedVersionImage      = "couchbase/server:7.0.1"
	MedVersionHashImage  = "couchbase/server@sha256:fa5d031059e005cd9d85983b1a120dab37fc60136cb699534b110f49d27388f7"
	HighVersionImage     = "couchbase/server:7.1.3"
	HighVersionHashImage = "couchbase/server@sha256:d0d1734a98fea7639793873d9a54c27d6be6e7838edad2a38e8d451d66be3497"
)

func TestIsNativeAuditCleanupEnabled(t *testing.T) {
	c := CouchbaseCluster{
		Spec: ClusterSpec{
			Logging: CouchbaseClusterLoggingSpec{
				Audit: &CouchbaseClusterAuditLoggingSpec{
					Enabled: true,
					Rotation: &CouchbaseClusterLogRotationSpec{
						PruneAge: &v1.Duration{Duration: 0 * time.Second},
					},
				},
			},
		},
	}

	if c.IsNativeAuditCleanupEnabled() {
		t.Error("expected IsNativeAuditCleanupEnabled to return false, but returned true")
	}

	c.Spec.Logging.Audit.Rotation.PruneAge = &v1.Duration{Duration: 10 * time.Second}

	if !c.IsNativeAuditCleanupEnabled() {
		t.Error("expected IsNativeAuditCleanupEnabled to return true, but returned false")
	}
}

func TestUnmarshalDurationWithNegativeOverride(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		crdEntry   string
		negValue   time.Duration
		defaultVal time.Duration
		expected   time.Duration
	}{
		{
			crdEntry:   "1s",
			negValue:   time.Second,
			defaultVal: time.Second,
			expected:   time.Second,
		},
		{
			crdEntry: "-1",
			negValue: time.Second * -1,
			expected: time.Second * -1,
		},
		{
			crdEntry: "-1",
			negValue: time.Millisecond * -1,
			expected: time.Millisecond * -1,
		},
		{
			crdEntry: "0",
			expected: time.Second * 0,
		},
		{
			crdEntry:   "",
			defaultVal: time.Second,
			expected:   time.Second,
		},
		{
			crdEntry:   "",
			defaultVal: time.Minute,
			expected:   time.Minute,
		},
	}

	for _, testcase := range testcases {
		duration, err := unmarshalDurationWithNegativeOverride(testcase.crdEntry, testcase.negValue, testcase.defaultVal)
		if err != nil {
			t.Fatal(err)
		}

		expected := v1.Duration{Duration: testcase.expected}
		if !reflect.DeepEqual(duration, &expected) {
			t.Errorf("expected duration to be %v, but got %v", expected, duration)
		}
	}
}

func TestMarshalDurationWithNegativeOverride(t *testing.T) {
	testcases := []struct {
		duration *v1.Duration
		negValue time.Duration
		expected string
	}{
		{
			duration: &v1.Duration{Duration: time.Second},
			expected: "1s",
		},
		{
			duration: &v1.Duration{Duration: time.Minute},
			expected: "1m0s",
		},
		{
			duration: &v1.Duration{Duration: time.Second * -1},
			negValue: time.Second * -1,
			expected: "-1",
		},
		{
			duration: &v1.Duration{Duration: time.Minute * -1},
			negValue: time.Minute * -1,
			expected: "-1",
		},
		{
			duration: &v1.Duration{Duration: time.Minute * -1},
			negValue: time.Second * -1,
			expected: "-1m0s",
		},
		{
			duration: &v1.Duration{Duration: time.Second * 0},
			negValue: time.Second * -1,
			expected: "0",
		},
		{
			duration: nil,
			expected: "",
		},
	}

	for _, testcase := range testcases {
		expected := testcase.expected

		actual := marshalDurationWithNegativeOverride(testcase.duration, testcase.negValue)
		if actual != expected {
			t.Errorf("expected %v, but got %v", expected, actual)
		}
	}
}

func TestMigrateDeprecatedRoles(t *testing.T) {
	tests := []struct {
		name     string
		input    []Role
		expected []Role
	}{
		{
			name: "migrate security_admin_local",
			input: []Role{
				{Name: RoleSecurityAdminLocal},
			},
			expected: []Role{
				{Name: RoleSecurityAdmin},
				{Name: RoleUserAdminLocal},
			},
		},
		{
			name: "migrate security_admin_external",
			input: []Role{
				{Name: RoleSecurityAdminExternal},
			},
			expected: []Role{
				{Name: RoleSecurityAdmin},
				{Name: RoleUserAdminExternal},
			},
		},
		{
			name: "no migration for new roles",
			input: []Role{
				{Name: RoleUserAdminLocal},
				{Name: RoleSecurityAdmin},
			},
			expected: []Role{
				{Name: RoleUserAdminLocal},
				{Name: RoleSecurityAdmin},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MigrateDeprecatedRoles(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

func TestResolvedRestartedAt(t *testing.T) {
	const (
		clusterTS = "2024-01-01T00:00:00Z"
		classTS   = "2024-06-01T00:00:00Z"
	)
	classWithAnno := func(name, value string) ServerConfig {
		sc := ServerConfig{Name: name}
		if value != "" {
			sc.Pod = &PodTemplate{
				ObjectMeta: ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/restartedAt": value,
					},
				},
			}
		}
		return sc
	}

	tests := []struct {
		name       string
		clusterAnn map[string]string
		server     *ServerConfig
		want       string
	}{
		{name: "no annotations", want: ""},
		{
			name:       "cluster only",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": clusterTS},
			server:     &ServerConfig{Name: "data"},
			want:       clusterTS,
		},
		{
			name: "per-class only",
			server: func() *ServerConfig {
				sc := classWithAnno("data", classTS)
				return &sc
			}(),
			want: classTS,
		},
		{
			name:       "per-class wins over cluster",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": clusterTS},
			server: func() *ServerConfig {
				sc := classWithAnno("data", classTS)
				return &sc
			}(),
			want: classTS,
		},
		{
			name:       "empty per-class falls back to cluster",
			clusterAnn: map[string]string{"kubectl.kubernetes.io/restartedAt": clusterTS},
			server: func() *ServerConfig {
				sc := classWithAnno("data", "")
				sc.Pod = &PodTemplate{ObjectMeta: ObjectMeta{Annotations: map[string]string{
					"kubectl.kubernetes.io/restartedAt": "",
				}}}
				return &sc
			}(),
			want: clusterTS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CouchbaseCluster{}
			c.Annotations = tt.clusterAnn
			if got := c.ResolvedRestartedAt(tt.server); got != tt.want {
				t.Errorf("ResolvedRestartedAt = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsedRestartedAt(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		zero    bool
	}{
		{name: "empty is zero time", value: "", zero: true},
		{name: "valid RFC3339", value: "2024-06-01T12:34:56Z"},
		{name: "valid with offset", value: "2024-06-01T12:34:56-07:00"},
		{name: "garbage rejected", value: "not-a-timestamp", wantErr: true},
		{name: "missing TZ rejected", value: "2024-06-01T12:34:56", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsedRestartedAt(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.zero && !got.IsZero() {
				t.Errorf("expected zero time, got %v", got)
			}
		})
	}
}
