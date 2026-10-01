// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"fmt"
	"os"

	"github.com/paulojmdias/go-imds/imds"
)

// The ECS agent injects one of these into every task. V4 supersedes V3 and
// carries strictly more, so it is preferred where both are present.
const (
	uriEnvV4 = "ECS_CONTAINER_METADATA_URI_V4"
	uriEnvV3 = "ECS_CONTAINER_METADATA_URI"
)

// taskPath is relative to the injected address, which already identifies this
// container.
const taskPath = "/task"

// Metadata describes the ECS task and container the process is running in.
type Metadata struct {
	// ContainerID is the container runtime's identifier.
	ContainerID string
	// ContainerName is the name the task definition gives this container.
	ContainerName string
	// ContainerARN identifies this container within the task.
	ContainerARN string
	// Image and ImageID describe the container image.
	Image   string
	ImageID string
	// LogDriver and LogOptions describe where the container's output is sent.
	// They are what a consumer needs to correlate this task with its logs.
	LogDriver  string
	LogOptions LogOptions

	// ClusterARN identifies the cluster, and is a name rather than an ARN on
	// clusters created before ECS started reporting the full ARN.
	ClusterARN string
	// TaskARN identifies the task.
	TaskARN string
	// Family and Revision identify the task definition.
	Family   string
	Revision string
	// LaunchType is "EC2" or "FARGATE", as reported.
	LaunchType string
	// Zone is the task's availability zone. It is reported for Fargate tasks
	// and absent on EC2 launches, where it belongs to the instance rather than
	// the task.
	Zone string

	// DockerName is the name the container runtime gave this container, which
	// differs from ContainerName.
	DockerName string
	// CreatedAt, StartedAt and FinishedAt are RFC 3339 timestamps, as reported.
	CreatedAt  string
	StartedAt  string
	FinishedAt string
	// KnownStatus is this container's reported state, for example "RUNNING".
	KnownStatus string
	// Type distinguishes an application container from an agent-injected one.
	Type string
	// Labels are the container's Docker labels.
	Labels map[string]string
	// Limits are this container's CPU and memory limits, zero when unset.
	Limits Limits
	// Networks are this container's attached networks.
	Networks []Network

	// TaskKnownStatus and TaskDesiredStatus are the task's reported and
	// intended states.
	TaskKnownStatus   string
	TaskDesiredStatus string
	// ServiceName is the ECS service that started the task, empty for a task
	// started directly.
	ServiceName string
	// VPCID is the VPC the task runs in, reported on awsvpc networking.
	VPCID string
	// PullStartedAt and PullStoppedAt bracket the image pull.
	PullStartedAt string
	PullStoppedAt string
	// TaskLimits are the task-level CPU and memory limits.
	TaskLimits Limits
	// EphemeralStorage reports Fargate ephemeral disk usage, in mebibytes.
	EphemeralStorage EphemeralStorage
	// TaskContainers are every container in the task, this one included.
	TaskContainers []Container
}

// Limits are CPU and memory limits. CPU is in vCPU units and Memory in
// mebibytes, as the service reports them.
type Limits struct {
	CPU    float64 `json:"CPU"`
	Memory uint64  `json:"Memory"`
}

// Network is one network attached to a container.
type Network struct {
	NetworkMode          string   `json:"NetworkMode"`
	IPv4Addresses        []string `json:"IPv4Addresses"`
	IPv6Addresses        []string `json:"IPv6Addresses"`
	MACAddress           string   `json:"MACAddress"`
	PrivateDNSName       string   `json:"PrivateDNSName"`
	SubnetCIDRBlock      string   `json:"SubnetCIDRBlock"`
	AttachmentIndex      int      `json:"AttachmentIndex"`
	DomainNameServers    []string `json:"DomainNameServers"`
	DomainNameSearchList []string `json:"DomainNameSearchList"`
}

// EphemeralStorage reports Fargate ephemeral disk usage, in mebibytes.
type EphemeralStorage struct {
	Utilized int64 `json:"Utilized"`
	Reserved int64 `json:"Reserved"`
}

// Container describes one container in the task.
type Container struct {
	DockerID     string            `json:"DockerId"`
	Name         string            `json:"Name"`
	DockerName   string            `json:"DockerName"`
	ContainerARN string            `json:"ContainerARN"`
	Image        string            `json:"Image"`
	ImageID      string            `json:"ImageID"`
	KnownStatus  string            `json:"KnownStatus"`
	Type         string            `json:"Type"`
	CreatedAt    string            `json:"CreatedAt"`
	StartedAt    string            `json:"StartedAt"`
	FinishedAt   string            `json:"FinishedAt"`
	Labels       map[string]string `json:"Labels"`
	Limits       Limits            `json:"Limits"`
	LogDriver    string            `json:"LogDriver"`
	LogOptions   LogOptions        `json:"LogOptions"`
	Networks     []Network         `json:"Networks"`
}

// LogOptions carries the awslogs driver's configuration, which is empty for a
// container logging through any other driver.
type LogOptions struct {
	Group  string `json:"awslogs-group"`
	Stream string `json:"awslogs-stream"`
	Region string `json:"awslogs-region"`
}

type taskDocument struct {
	Cluster          string           `json:"Cluster"`
	TaskARN          string           `json:"TaskARN"`
	Family           string           `json:"Family"`
	Revision         string           `json:"Revision"`
	AvailabilityZone string           `json:"AvailabilityZone"`
	LaunchType       string           `json:"LaunchType"`
	KnownStatus      string           `json:"KnownStatus"`
	DesiredStatus    string           `json:"DesiredStatus"`
	ServiceName      string           `json:"ServiceName"`
	VPCID            string           `json:"VPCID"`
	PullStartedAt    string           `json:"PullStartedAt"`
	PullStoppedAt    string           `json:"PullStoppedAt"`
	Limits           Limits           `json:"Limits"`
	EphemeralStorage EphemeralStorage `json:"EphemeralStorageMetrics"`
	Containers       []Container      `json:"Containers"`
}

// Fetch reads the metadata of the ECS task the process is running in.
//
// The error is [imds.ErrNotDetected] when the process is not in an ECS task,
// which includes the ordinary case of neither environment variable being set.
// Any other error means the metadata service was reachable and the lookup
// failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	endpoints := c.Endpoints(fromEnvironment()...)
	if len(endpoints) == 0 {
		// Not an error worth a request: outside a task there is no address to
		// try, and the agent injecting nothing is exactly how a process is
		// meant to find that out.
		return Metadata{}, fmt.Errorf("%w: neither %s nor %s is set", imds.ErrAbsent, uriEnvV4, uriEnvV3)
	}

	var (
		container Container
		task      taskDocument
	)
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint, err := c.GetJSONFirst(ctx, endpoints, "", &container)
		if err != nil {
			return err
		}
		// The container document identified the service, so a 4xx on the task
		// document is that service failing rather than a sign of not being in a
		// task at all.
		_, err = c.GetJSONFirst(ctx, []string{endpoint}, taskPath, &task,
			imds.NeverForeign())
		return err
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ContainerID:   container.DockerID,
		ContainerName: container.Name,
		ContainerARN:  container.ContainerARN,
		Image:         container.Image,
		ImageID:       container.ImageID,
		LogDriver:     container.LogDriver,
		LogOptions:    container.LogOptions,
		DockerName:    container.DockerName,
		CreatedAt:     container.CreatedAt,
		StartedAt:     container.StartedAt,
		FinishedAt:    container.FinishedAt,
		KnownStatus:   container.KnownStatus,
		Type:          container.Type,
		Labels:        container.Labels,
		Limits:        container.Limits,
		Networks:      container.Networks,

		ClusterARN: task.Cluster,
		TaskARN:    task.TaskARN,
		Family:     task.Family,
		Revision:   task.Revision,
		LaunchType: task.LaunchType,
		Zone:       task.AvailabilityZone,

		TaskKnownStatus:   task.KnownStatus,
		TaskDesiredStatus: task.DesiredStatus,
		ServiceName:       task.ServiceName,
		VPCID:             task.VPCID,
		PullStartedAt:     task.PullStartedAt,
		PullStoppedAt:     task.PullStoppedAt,
		TaskLimits:        task.Limits,
		EphemeralStorage:  task.EphemeralStorage,
		TaskContainers:    task.Containers,
	}, nil
}

// fromEnvironment returns the injected address, preferring V4.
func fromEnvironment() []string {
	for _, env := range []string{uriEnvV4, uriEnvV3} {
		if uri := os.Getenv(env); uri != "" {
			return []string{uri}
		}
	}
	return nil
}
