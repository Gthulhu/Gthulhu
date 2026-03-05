package container

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	mongo "go.mongodb.org/mongo-driver/v2/mongo"
	mongooption "go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoContainerOptions struct {
	Username string
	Password string
	Database string
	Port     string
}

type MongoContainerConnection struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
}

const (
	mongoDBPort = 27017
)

// RunMongoContainer runs a MongoDB container with the specified options and returns the connection details.
func RunMongoContainer(builder *ContainerBuilder, name string, options MongoContainerConnection) (MongoContainerConnection, error) {
	runOptions := dockertest.RunOptions{
		Name:       name,
		Repository: "mongo",
		Tag:        "8.2.2",
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=" + options.Username,
			"MONGO_INITDB_ROOT_PASSWORD=" + options.Password,
		},
	}
	if options.Database != "" {
		runOptions.Env = append(runOptions.Env, "MONGO_INITDB_DATABASE="+options.Database)
	}
	if options.Port != "" {
		runOptions.PortBindings = map[docker.Port][]docker.PortBinding{
			docker.Port(strconv.Itoa(mongoDBPort) + "/tcp"): {{HostIP: "127.0.0.1", HostPort: options.Port}},
		}
	}

	container, err := builder.FindContainer(name)
	if err != nil {
		return MongoContainerConnection{}, err
	}
	if container != nil && container.State == "running" {
		publicPort := int64(0)
		host := ""
		for _, bind := range container.Ports {
			if bind.PrivatePort == mongoDBPort {
				host = bind.IP
				publicPort = bind.PublicPort
				break
			}
		}
		if publicPort == 0 {
			return MongoContainerConnection{}, fmt.Errorf("failed to find public port for mongo container (%s)", name)
		}

		builder.AddContainer(container.ID, ContainerInfo{
			Name: name,
			Type: ContainerTypeMongoDB,
		})
		return MongoContainerConnection{
			Host:     host,
			Port:     strconv.FormatInt(publicPort, 10),
			Username: options.Username,
			Password: options.Password,
			Database: options.Database,
		}, nil
	}

	resource, err := builder.RunWithOptions(&runOptions)
	if err != nil {
		return MongoContainerConnection{}, err
	}

	builder.AddContainer(resource.Container.ID, ContainerInfo{
		Name: name,
		Type: ContainerTypeMongoDB,
	})
	host := resource.GetBoundIP(strconv.Itoa(mongoDBPort) + "/tcp")
	mongoPort := resource.GetPort(strconv.Itoa(mongoDBPort) + "/tcp")

	builder.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", options.Username, options.Password, host, mongoPort)

		mongoOpts := mongooption.Client().ApplyURI(uri)
		client, err := mongo.Connect(mongoOpts)
		if err != nil {
			return err
		}
		return client.Ping(ctx, nil)
	})

	return MongoContainerConnection{
		Host:     host,
		Port:     mongoPort,
		Username: options.Username,
		Password: options.Password,
		Database: options.Database,
	}, nil
}
