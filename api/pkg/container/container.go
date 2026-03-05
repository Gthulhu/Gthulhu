package container

import (
	"context"
	"errors"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	containerRunningState = "running"
	containerExitedState  = "exited"
	containerPausedState  = "paused"
	containerCreatedState = "created"
)

// NewContainerBuilder creates a new ContainerBuilder instance. The endpoint parameter specifies the Docker endpoint to connect to. can be empty to use the default.
func NewContainerBuilder(endpoint string) (*ContainerBuilder, error) {
	pool, err := dockertest.NewPool(endpoint)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = pool.Client.PingWithContext(ctx)
	if err != nil {
		return nil, errors.New("could not connect to docker endpoint: " + err.Error())
	}
	return &ContainerBuilder{
		Pool:         pool,
		containerIDs: make(map[string]ContainerInfo),
	}, nil
}

type ContainerType string

const (
	ContainerTypeMongoDB ContainerType = "mongodb"
)

type ContainerInfo struct {
	Name string
	Type ContainerType
}

type ContainerBuilder struct {
	*dockertest.Pool
	containerIDs map[string]ContainerInfo
}

func (builder *ContainerBuilder) RemoveByID(containerID string) error {
	return builder.Client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true, RemoveVolumes: true})
}

func (builder *ContainerBuilder) FindContainer(containerName string) (*docker.APIContainers, error) {
	containers, err := builder.Client.ListContainers(docker.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"name": {containerName},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, nil
	}
	if containers[0].State != containerRunningState {
		if err := builder.RemoveByID(containers[0].ID); err != nil {
			return nil, err
		}
		return nil, nil
	}

	return &containers[0], nil
}

func (builder *ContainerBuilder) PruneAll() error {
	var err error
	for id := range builder.containerIDs {
		purErr := builder.RemoveByID(id)
		if purErr != nil {
			if err == nil {
				err = purErr
			} else {
				err = errors.Join(err, purErr)
			}
		}
	}
	return err
}

func (builder *ContainerBuilder) AllContainerIDs() []string {
	ids := make([]string, 0, len(builder.containerIDs))
	for id := range builder.containerIDs {
		ids = append(ids, id)
	}
	return ids
}

func (builder *ContainerBuilder) AddContainer(id string, containerInfo ContainerInfo) {
	builder.containerIDs[id] = containerInfo
}

func (builder *ContainerBuilder) GetContainerInfo(id string) (ContainerInfo, bool) {
	info, ok := builder.containerIDs[id]
	return info, ok
}
