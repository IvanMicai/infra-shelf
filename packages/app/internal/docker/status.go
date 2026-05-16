package docker

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

type Status struct {
	Service   string
	Container string
	State     string
	Exists    bool
	Running   bool
}

var containers = []Status{
	{Service: "PostgreSQL", Container: "infra-postgres"},
	{Service: "Redis", Container: "infra-redis"},
	{Service: "RabbitMQ", Container: "infra-rabbitmq"},
	{Service: "AIStor", Container: "infra-aistor"},
}

func ListStatus(ctx context.Context) []Status {
	statuses := make([]Status, 0, len(containers))
	for _, item := range containers {
		statuses = append(statuses, inspect(ctx, item))
	}
	return statuses
}

func inspect(ctx context.Context, item Status) Status {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{.State.Status}}", item.Container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		item.State = "not created"
		item.Exists = false
		item.Running = false
		return item
	}

	item.State = strings.TrimSpace(string(output))
	item.Exists = true
	item.Running = item.State == "running"
	return item
}
