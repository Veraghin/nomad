package e2eutil

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/uuid"
)

// NodeRestart is a test helper function that restarts a client node
// running under systemd using a raw_exec job. Returns the job ID of
// the restart job so that callers can clean it up.
func NodeRestart(client *api.Client, nodeID string) (string, error) {
	ok, err := isUbuntu(client, nodeID)
	if !ok {
		// TODO(tgross): we're checking this because we want to use
		// systemctl to restart the node, but we should also figure
		// out a way to detect dev mode targets.
		return "", fmt.Errorf("NodeRestart only works against ubuntu targets")
	}
	if err != nil {
		return "", err
	}

	job := newRestartJob(nodeID)
	jobID := *job.ID
	_, _, err = client.Jobs().Register(job, nil)
	if err != nil {
		return jobID, err
	}

	retries := 30
	for retries > 0 {
		time.Sleep(1 * time.Second)
		retries--
		node, _, err := client.Nodes().Info(nodeID, nil)
		if node != nil && node.Status == "ready" {
			return jobID, nil
		}
		if retries == 0 {
			return jobID, err
		}
	}
	return jobID, nil
}

func isUbuntu(client *api.Client, nodeID string) (bool, error) {
	node, _, err := client.Nodes().Info(nodeID, nil)
	if err != nil || node == nil {
		return false, err
	}
	if name, ok := node.Attributes["os.name"]; ok {
		return name == "ubuntu", nil
	}
	return false, nil
}

func newRestartJob(nodeID string) *api.Job {
	jobType := "batch"
	name := "restart"
	jobID := "restart-" + uuid.Generate()[0:8]
	job := &api.Job{
		Name:        &name,
		ID:          &jobID,
		Datacenters: []string{"dc1"},
		Type:        &jobType,
		TaskGroups: []*api.TaskGroup{
			{
				Name: &name,
				Constraints: []*api.Constraint{
					{
						LTarget: "${node.unique.id}",
						RTarget: nodeID,
						Operand: "=",
					},
				},
				Tasks: []*api.Task{
					{
						Name:   name,
						Driver: "raw_exec",
						Config: map[string]interface{}{
							"command": "systemctl",
							"args": []string{
								"restart", "nomad", "--no-block"},
						},
					},
				},
			},
		},
	}
	job.Canonicalize()
	return job
}
