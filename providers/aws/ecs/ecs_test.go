// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ecs_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/aws/ecs"
)

// The field set follows the ECS task metadata endpoint, version 4. Values are
// illustrative rather than captured from a running task.
const containerJSON = `{
  "DockerId": "ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66",
  "Name": "app-server-1",
  "DockerName": "ecs-telemetry-3-app-server-1",
  "Image": "public.ecr.aws/example/app-server:latest",
  "ImageID": "sha256:6c2b4a3c9b1f8f0d1e2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f80",
  "ContainerARN": "arn:aws:ecs:us-west-2:111122223333:container/0206b271-b33f-47ab-86c6-a0ba208a70a9",
  "LogDriver": "awslogs",
  "LogOptions": {
    "awslogs-group": "/ecs/telemetry",
    "awslogs-stream": "ecs/app-server-1/158d1c8083dd49d6b527399fd6414f5c",
    "awslogs-region": "us-west-2"
  },
  "KnownStatus": "RUNNING",
  "Type": "NORMAL",
  "CreatedAt": "2024-01-02T03:04:05.123456789Z",
  "StartedAt": "2024-01-02T03:04:06.123456789Z",
  "Labels": { "com.amazonaws.ecs.cluster": "default" },
  "Limits": { "CPU": 0.5, "Memory": 512 },
  "Networks": [
    {
      "NetworkMode": "awsvpc",
      "IPv4Addresses": ["10.0.1.15"],
      "IPv6Addresses": ["2600:1f14::1"],
      "MACAddress": "0a:58:0a:00:01:0f",
      "PrivateDNSName": "ip-10-0-1-15.us-west-2.compute.internal",
      "SubnetCIDRBlock": "10.0.1.0/24",
      "AttachmentIndex": 0,
      "DomainNameServers": ["10.0.0.2"],
      "DomainNameSearchList": ["us-west-2.compute.internal"]
    }
  ]
}`

const taskJSON = `{
  "Cluster": "arn:aws:ecs:us-west-2:111122223333:cluster/default",
  "TaskARN": "arn:aws:ecs:us-west-2:111122223333:task/default/158d1c8083dd49d6b527399fd6414f5c",
  "Family": "telemetry",
  "Revision": "3",
  "DesiredStatus": "RUNNING",
  "KnownStatus": "RUNNING",
  "AvailabilityZone": "us-west-2a",
  "LaunchType": "FARGATE",
  "ServiceName": "telemetry-svc",
  "VPCID": "vpc-0a1b2c3d",
  "PullStartedAt": "2024-01-02T03:04:00.000000000Z",
  "PullStoppedAt": "2024-01-02T03:04:04.000000000Z",
  "Limits": { "CPU": 1, "Memory": 2048 },
  "EphemeralStorageMetrics": { "Utilized": 261, "Reserved": 20496 },
  "Containers": [
    {
      "DockerId": "ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66",
      "Name": "app-server-1",
      "KnownStatus": "RUNNING",
      "Type": "NORMAL"
    }
  ]
}`

// The agent serves the container document at the injected address itself and
// the task document one level below it.
func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/task", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(taskJSON))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(containerJSON))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "aws/ecs",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := ecs.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, ecs.Metadata{
		ContainerID:   "ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66",
		ContainerName: "app-server-1",
		ContainerARN:  "arn:aws:ecs:us-west-2:111122223333:container/0206b271-b33f-47ab-86c6-a0ba208a70a9",
		Image:         "public.ecr.aws/example/app-server:latest",
		ImageID:       "sha256:6c2b4a3c9b1f8f0d1e2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f80",
		LogDriver:     "awslogs",
		LogOptions: ecs.LogOptions{
			Group:  "/ecs/telemetry",
			Stream: "ecs/app-server-1/158d1c8083dd49d6b527399fd6414f5c",
			Region: "us-west-2",
		},
		ClusterARN: "arn:aws:ecs:us-west-2:111122223333:cluster/default",
		TaskARN:    "arn:aws:ecs:us-west-2:111122223333:task/default/158d1c8083dd49d6b527399fd6414f5c",
		Family:     "telemetry",
		Revision:   "3",
		LaunchType: "FARGATE",
		Zone:       "us-west-2a",

		DockerName:  "ecs-telemetry-3-app-server-1",
		CreatedAt:   "2024-01-02T03:04:05.123456789Z",
		StartedAt:   "2024-01-02T03:04:06.123456789Z",
		KnownStatus: "RUNNING",
		Type:        "NORMAL",
		Labels:      map[string]string{"com.amazonaws.ecs.cluster": "default"},
		Limits:      ecs.Limits{CPU: 0.5, Memory: 512},
		Networks: []ecs.Network{{
			NetworkMode:          "awsvpc",
			IPv4Addresses:        []string{"10.0.1.15"},
			IPv6Addresses:        []string{"2600:1f14::1"},
			MACAddress:           "0a:58:0a:00:01:0f",
			PrivateDNSName:       "ip-10-0-1-15.us-west-2.compute.internal",
			SubnetCIDRBlock:      "10.0.1.0/24",
			DomainNameServers:    []string{"10.0.0.2"},
			DomainNameSearchList: []string{"us-west-2.compute.internal"},
		}},

		TaskKnownStatus:   "RUNNING",
		TaskDesiredStatus: "RUNNING",
		ServiceName:       "telemetry-svc",
		VPCID:             "vpc-0a1b2c3d",
		PullStartedAt:     "2024-01-02T03:04:00.000000000Z",
		PullStoppedAt:     "2024-01-02T03:04:04.000000000Z",
		TaskLimits:        ecs.Limits{CPU: 1, Memory: 2048},
		EphemeralStorage:  ecs.EphemeralStorage{Utilized: 261, Reserved: 20496},
		TaskContainers: []ecs.Container{{
			DockerID:    "ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66",
			Name:        "app-server-1",
			KnownStatus: "RUNNING",
			Type:        "NORMAL",
		}},
	}, md)
}

// Outside a task the agent injects nothing, and that is the whole signal. No
// request is worth making, and it must not read as a failure.
func TestNoEnvironmentVariableIsAbsence(t *testing.T) {
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", "")
	t.Setenv("ECS_CONTAINER_METADATA_URI", "")

	_, err := ecs.Fetch(t.Context())
	require.ErrorIs(t, err, imds.ErrAbsent)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestAddressIsTakenFromTheEnvironment(t *testing.T) {
	ep := imdstest.Server(t, handler())
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", ep)

	md, err := ecs.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "telemetry", md.Family)
}

// V4 supersedes V3 and carries strictly more, so it wins where both are set.
func TestV4IsPreferredOverV3(t *testing.T) {
	v4 := imdstest.Server(t, handler())
	v3 := imdstest.Server(t, imdstest.Status(http.StatusInternalServerError))
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", v4)
	t.Setenv("ECS_CONTAINER_METADATA_URI", v3)

	md, err := ecs.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "telemetry", md.Family)
}

func TestV3IsUsedWhenV4IsUnset(t *testing.T) {
	ep := imdstest.Server(t, handler())
	t.Setenv("ECS_CONTAINER_METADATA_URI_V4", "")
	t.Setenv("ECS_CONTAINER_METADATA_URI", ep)

	md, err := ecs.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "telemetry", md.Family)
}

// The container document already identified the service, so a failure reading
// the task document afterwards is that service failing.
func TestTaskDocumentFailureIsNotAbsence(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/task", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(containerJSON))
	})
	ep := imdstest.Server(t, mux)

	_, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}
