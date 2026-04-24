package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetDefaults_AllEmpty(t *testing.T) {
	r := &Redis{
		Spec: RedisSpec{},
	}
	r.SetDefaults()

	if r.Spec.Image != "bitnami/redis:8.0" {
		t.Errorf("expected default image bitnami/redis:8.0, got %s", r.Spec.Image)
	}
	if r.Spec.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %d", r.Spec.Replicas)
	}
	if r.Spec.Resources.Requests.CPU != "100m" {
		t.Errorf("expected default request CPU 100m, got %s", r.Spec.Resources.Requests.CPU)
	}
	if r.Spec.Resources.Requests.Memory != "128Mi" {
		t.Errorf("expected default request Memory 128Mi, got %s", r.Spec.Resources.Requests.Memory)
	}
	if r.Spec.Resources.Limits.CPU != "250m" {
		t.Errorf("expected default limit CPU 250m, got %s", r.Spec.Resources.Limits.CPU)
	}
	if r.Spec.Resources.Limits.Memory != "256Mi" {
		t.Errorf("expected default limit Memory 256Mi, got %s", r.Spec.Resources.Limits.Memory)
	}

	// Liveness probe defaults
	if len(r.Spec.Probes.Liveness.Command) == 0 {
		t.Error("expected default liveness command")
	}
	if r.Spec.Probes.Liveness.InitialDelaySeconds != 20 {
		t.Errorf("expected liveness InitialDelaySeconds 20, got %d", r.Spec.Probes.Liveness.InitialDelaySeconds)
	}
	if r.Spec.Probes.Liveness.PeriodSeconds != 10 {
		t.Errorf("expected liveness PeriodSeconds 10, got %d", r.Spec.Probes.Liveness.PeriodSeconds)
	}
	if r.Spec.Probes.Liveness.TimeoutSeconds != 3 {
		t.Errorf("expected liveness TimeoutSeconds 3, got %d", r.Spec.Probes.Liveness.TimeoutSeconds)
	}
	if r.Spec.Probes.Liveness.FailureThreshold != 3 {
		t.Errorf("expected liveness FailureThreshold 3, got %d", r.Spec.Probes.Liveness.FailureThreshold)
	}

	// Readiness probe defaults
	if len(r.Spec.Probes.Readiness.Command) == 0 {
		t.Error("expected default readiness command")
	}
	if r.Spec.Probes.Readiness.InitialDelaySeconds != 5 {
		t.Errorf("expected readiness InitialDelaySeconds 5, got %d", r.Spec.Probes.Readiness.InitialDelaySeconds)
	}
	if r.Spec.Probes.Readiness.PeriodSeconds != 15 {
		t.Errorf("expected readiness PeriodSeconds 15, got %d", r.Spec.Probes.Readiness.PeriodSeconds)
	}
	if r.Spec.Probes.Readiness.TimeoutSeconds != 4 {
		t.Errorf("expected readiness TimeoutSeconds 4, got %d", r.Spec.Probes.Readiness.TimeoutSeconds)
	}
	if r.Spec.Probes.Readiness.FailureThreshold != 3 {
		t.Errorf("expected readiness FailureThreshold 3, got %d", r.Spec.Probes.Readiness.FailureThreshold)
	}
}

func TestSetDefaults_NoOverrideExisting(t *testing.T) {
	r := &Redis{
		Spec: RedisSpec{
			Image:    "custom/redis:9.0",
			Replicas: 5,
			Resources: ResourceSpec{
				Requests: ResourceList{CPU: "200m", Memory: "256Mi"},
				Limits:   ResourceList{CPU: "500m", Memory: "512Mi"},
			},
			Probes: ProbeSpec{
				Liveness: ProbeConfig{
					Command:             []string{"custom-check"},
					InitialDelaySeconds: 30,
					PeriodSeconds:       20,
					TimeoutSeconds:      5,
					FailureThreshold:    5,
				},
				Readiness: ProbeConfig{
					Command:             []string{"custom-readiness"},
					InitialDelaySeconds: 10,
					PeriodSeconds:       25,
					TimeoutSeconds:      6,
					FailureThreshold:    4,
				},
			},
		},
	}
	r.SetDefaults()

	if r.Spec.Image != "custom/redis:9.0" {
		t.Errorf("expected custom image, got %s", r.Spec.Image)
	}
	if r.Spec.Replicas != 5 {
		t.Errorf("expected 5 replicas, got %d", r.Spec.Replicas)
	}
	if r.Spec.Resources.Requests.CPU != "200m" {
		t.Errorf("expected custom request CPU, got %s", r.Spec.Resources.Requests.CPU)
	}
	if r.Spec.Resources.Requests.Memory != "256Mi" {
		t.Errorf("expected custom request Memory, got %s", r.Spec.Resources.Requests.Memory)
	}
	if r.Spec.Resources.Limits.CPU != "500m" {
		t.Errorf("expected custom limit CPU, got %s", r.Spec.Resources.Limits.CPU)
	}
	if r.Spec.Resources.Limits.Memory != "512Mi" {
		t.Errorf("expected custom limit Memory, got %s", r.Spec.Resources.Limits.Memory)
	}
	if r.Spec.Probes.Liveness.Command[0] != "custom-check" {
		t.Errorf("expected custom liveness command, got %v", r.Spec.Probes.Liveness.Command)
	}
	if r.Spec.Probes.Liveness.InitialDelaySeconds != 30 {
		t.Errorf("expected custom liveness InitialDelaySeconds, got %d", r.Spec.Probes.Liveness.InitialDelaySeconds)
	}
	if r.Spec.Probes.Liveness.PeriodSeconds != 20 {
		t.Errorf("expected custom liveness PeriodSeconds, got %d", r.Spec.Probes.Liveness.PeriodSeconds)
	}
	if r.Spec.Probes.Liveness.TimeoutSeconds != 5 {
		t.Errorf("expected custom liveness TimeoutSeconds, got %d", r.Spec.Probes.Liveness.TimeoutSeconds)
	}
	if r.Spec.Probes.Liveness.FailureThreshold != 5 {
		t.Errorf("expected custom liveness FailureThreshold, got %d", r.Spec.Probes.Liveness.FailureThreshold)
	}
	if r.Spec.Probes.Readiness.Command[0] != "custom-readiness" {
		t.Errorf("expected custom readiness command, got %v", r.Spec.Probes.Readiness.Command)
	}
	if r.Spec.Probes.Readiness.InitialDelaySeconds != 10 {
		t.Errorf("expected custom readiness InitialDelaySeconds, got %d", r.Spec.Probes.Readiness.InitialDelaySeconds)
	}
	if r.Spec.Probes.Readiness.PeriodSeconds != 25 {
		t.Errorf("expected custom readiness PeriodSeconds, got %d", r.Spec.Probes.Readiness.PeriodSeconds)
	}
	if r.Spec.Probes.Readiness.TimeoutSeconds != 6 {
		t.Errorf("expected custom readiness TimeoutSeconds, got %d", r.Spec.Probes.Readiness.TimeoutSeconds)
	}
	if r.Spec.Probes.Readiness.FailureThreshold != 4 {
		t.Errorf("expected custom readiness FailureThreshold, got %d", r.Spec.Probes.Readiness.FailureThreshold)
	}
}

func TestSetDefaults_PartialSpec(t *testing.T) {
	r := &Redis{
		Spec: RedisSpec{
			Image:    "custom/redis:7.0",
			Replicas: 3,
			// Resources left empty - should get defaults
			// Probes left empty - should get defaults
		},
	}
	r.SetDefaults()

	if r.Spec.Image != "custom/redis:7.0" {
		t.Errorf("expected custom image, got %s", r.Spec.Image)
	}
	if r.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", r.Spec.Replicas)
	}
	// Resources should get defaults
	if r.Spec.Resources.Requests.CPU != "100m" {
		t.Errorf("expected default request CPU, got %s", r.Spec.Resources.Requests.CPU)
	}
	if r.Spec.Resources.Limits.CPU != "250m" {
		t.Errorf("expected default limit CPU, got %s", r.Spec.Resources.Limits.CPU)
	}
}

// DeepCopy tests for all types
func TestRedis_DeepCopy(t *testing.T) {
	r := &Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: RedisSpec{
			Replicas: 3,
			Image:    "redis:7",
			Resources: ResourceSpec{
				Requests: ResourceList{CPU: "100m", Memory: "128Mi"},
				Limits:   ResourceList{CPU: "250m", Memory: "256Mi"},
			},
			Probes: ProbeSpec{
				Liveness: ProbeConfig{
					Command:             []string{"sh", "-c", "redis-cli PING"},
					InitialDelaySeconds: 20,
					PeriodSeconds:       10,
					TimeoutSeconds:      3,
					FailureThreshold:    3,
				},
				Readiness: ProbeConfig{
					Command:             []string{"sh", "-c", "redis-cli SET probe OK"},
					InitialDelaySeconds: 5,
					PeriodSeconds:       15,
					TimeoutSeconds:      4,
					FailureThreshold:    3,
				},
			},
		},
		Status: RedisStatus{
			Status:  "Ready",
			Message: "All good",
			Nodes:   []string{"node-1", "node-2"},
			Conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "AllGood",
				},
			},
		},
	}

	copy := r.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if copy.Name != r.Name {
		t.Errorf("DeepCopy name mismatch: got %s, want %s", copy.Name, r.Name)
	}
	if copy.Spec.Replicas != r.Spec.Replicas {
		t.Errorf("DeepCopy replicas mismatch")
	}
	if copy.Status.Status != r.Status.Status {
		t.Errorf("DeepCopy status mismatch")
	}
	if len(copy.Status.Nodes) != len(r.Status.Nodes) {
		t.Errorf("DeepCopy nodes length mismatch")
	}
	if len(copy.Status.Conditions) != len(r.Status.Conditions) {
		t.Errorf("DeepCopy conditions length mismatch")
	}

	// Verify deep copy (mutation should not affect original)
	copy.Spec.Replicas = 99
	if r.Spec.Replicas == 99 {
		t.Error("DeepCopy did not create independent copy")
	}
	copy.Status.Nodes[0] = "modified"
	if r.Status.Nodes[0] == "modified" {
		t.Error("DeepCopy did not deep copy Nodes slice")
	}
	copy.Spec.Probes.Liveness.Command[0] = "modified"
	if r.Spec.Probes.Liveness.Command[0] == "modified" {
		t.Error("DeepCopy did not deep copy Liveness Command slice")
	}

	// DeepCopyObject
	obj := r.DeepCopyObject()
	if obj == nil {
		t.Fatal("DeepCopyObject returned nil")
	}
	if _, ok := obj.(*Redis); !ok {
		t.Error("DeepCopyObject should return *Redis")
	}
}

func TestRedis_DeepCopy_Nil(t *testing.T) {
	var r *Redis
	copy := r.DeepCopy()
	if copy != nil {
		t.Error("DeepCopy on nil should return nil")
	}
}

func TestRedisList_DeepCopy(t *testing.T) {
	list := &RedisList{
		Items: []Redis{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "r1"},
				Spec:       RedisSpec{Replicas: 1, Image: "redis:7"},
				Status: RedisStatus{
					Nodes:      []string{"n1"},
					Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "r2"},
				Spec:       RedisSpec{Replicas: 2, Image: "redis:8"},
			},
		},
	}

	copy := list.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if len(copy.Items) != 2 {
		t.Errorf("DeepCopy items length: got %d, want 2", len(copy.Items))
	}

	// Verify independence
	copy.Items[0].Spec.Replicas = 99
	if list.Items[0].Spec.Replicas == 99 {
		t.Error("DeepCopy did not create independent copy of list items")
	}

	// DeepCopyObject
	obj := list.DeepCopyObject()
	if obj == nil {
		t.Fatal("DeepCopyObject returned nil")
	}
	if _, ok := obj.(*RedisList); !ok {
		t.Error("DeepCopyObject should return *RedisList")
	}
}

func TestRedisList_DeepCopy_Nil(t *testing.T) {
	var list *RedisList
	copy := list.DeepCopy()
	if copy != nil {
		t.Error("DeepCopy on nil should return nil")
	}
}

func TestRedisList_DeepCopy_EmptyItems(t *testing.T) {
	list := &RedisList{}
	copy := list.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if copy.Items != nil {
		t.Error("DeepCopy of empty list should have nil Items")
	}
}

func TestRedisSpec_DeepCopy(t *testing.T) {
	spec := &RedisSpec{
		Replicas: 3,
		Image:    "redis:7",
		Resources: ResourceSpec{
			Requests: ResourceList{CPU: "100m", Memory: "128Mi"},
			Limits:   ResourceList{CPU: "250m", Memory: "256Mi"},
		},
		Probes: ProbeSpec{
			Liveness: ProbeConfig{
				Command: []string{"a", "b"},
			},
		},
	}

	copy := spec.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	copy.Probes.Liveness.Command[0] = "modified"
	if spec.Probes.Liveness.Command[0] == "modified" {
		t.Error("DeepCopy did not deep copy command slice")
	}

	var nilSpec *RedisSpec
	if nilSpec.DeepCopy() != nil {
		t.Error("DeepCopy on nil should return nil")
	}
}

func TestRedisStatus_DeepCopy(t *testing.T) {
	status := &RedisStatus{
		Status:  "Ready",
		Message: "ok",
		Nodes:   []string{"a", "b"},
		Conditions: []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "OK"},
		},
	}

	copy := status.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	copy.Nodes[0] = "modified"
	if status.Nodes[0] == "modified" {
		t.Error("DeepCopy did not deep copy Nodes")
	}
	copy.Conditions[0].Reason = "modified"
	if status.Conditions[0].Reason == "modified" {
		t.Error("DeepCopy did not deep copy Conditions")
	}

	var nilStatus *RedisStatus
	if nilStatus.DeepCopy() != nil {
		t.Error("DeepCopy on nil should return nil")
	}
}

func TestProbeSpec_DeepCopy(t *testing.T) {
	ps := &ProbeSpec{
		Readiness: ProbeConfig{Command: []string{"a"}},
		Liveness:  ProbeConfig{Command: []string{"b"}},
	}
	copy := ps.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	copy.Readiness.Command[0] = "x"
	if ps.Readiness.Command[0] == "x" {
		t.Error("not a deep copy")
	}

	var nilPS *ProbeSpec
	if nilPS.DeepCopy() != nil {
		t.Error("nil DeepCopy should return nil")
	}
}

func TestProbeConfig_DeepCopy(t *testing.T) {
	pc := &ProbeConfig{
		Command:             []string{"check"},
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      2,
		FailureThreshold:    3,
	}
	copy := pc.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	copy.Command[0] = "modified"
	if pc.Command[0] == "modified" {
		t.Error("not a deep copy")
	}

	// nil Command
	pc2 := &ProbeConfig{InitialDelaySeconds: 5}
	copy2 := pc2.DeepCopy()
	if copy2.Command != nil {
		t.Error("nil command should stay nil")
	}

	var nilPC *ProbeConfig
	if nilPC.DeepCopy() != nil {
		t.Error("nil DeepCopy should return nil")
	}
}

func TestResourceSpec_DeepCopy(t *testing.T) {
	rs := &ResourceSpec{
		Requests: ResourceList{CPU: "100m", Memory: "128Mi"},
		Limits:   ResourceList{CPU: "250m", Memory: "256Mi"},
	}
	copy := rs.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if copy.Requests.CPU != "100m" {
		t.Error("DeepCopy mismatch")
	}

	var nilRS *ResourceSpec
	if nilRS.DeepCopy() != nil {
		t.Error("nil DeepCopy should return nil")
	}
}

func TestResourceList_DeepCopy(t *testing.T) {
	rl := &ResourceList{CPU: "100m", Memory: "128Mi"}
	copy := rl.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if copy.CPU != "100m" || copy.Memory != "128Mi" {
		t.Error("DeepCopy mismatch")
	}

	var nilRL *ResourceList
	if nilRL.DeepCopy() != nil {
		t.Error("nil DeepCopy should return nil")
	}
}
