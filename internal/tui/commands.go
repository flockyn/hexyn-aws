package tui

import (
	"context"
	"fmt"

	"hexyn-aws/internal/awsx"
	"hexyn-aws/internal/secrets"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// commandRunner turns service calls into Bubble Tea commands that emit messages.
type commandRunner struct {
	svc *secrets.Service
}

// outputLabel formats the user-facing destination path, collapsing an empty
// subdirectory to the base output/ directory instead of "output//". The
// receiver is unused; it keeps the helper grouped on commandRunner.
func (*commandRunner) outputLabel(subDir string) string {
	if subDir == "" {
		return "output/"
	}
	return "output/" + subDir + "/"
}

func (c *commandRunner) checkSession() tea.Cmd {
	return func() tea.Msg {
		session, err := c.svc.CheckSession(context.Background())
		return sessionMsg{session: session, err: err}
	}
}

func (c *commandRunner) listRegions() tea.Cmd {
	return func() tea.Msg {
		regions, err := c.svc.ListRegions(context.Background())
		if err != nil {
			return listMsg{err: err}
		}
		items := make([]list.Item, 0, len(regions))
		for _, r := range regions {
			items = append(items, item{title: r, desc: "AWS Region"})
		}
		return listMsg{items: items, next: stateSelectRegion}
	}
}

func (c *commandRunner) listClusters(region string) tea.Cmd {
	return func() tea.Msg {
		clusters, err := c.svc.ListClusters(context.Background(), region)
		if err != nil {
			return listMsg{err: err}
		}
		items := make([]list.Item, 0, len(clusters))
		for _, cl := range clusters {
			items = append(items, item{title: cl.Name, desc: "ECS Cluster"})
		}
		if len(items) == 0 {
			return listMsg{err: fmt.Errorf("no ECS clusters found")}
		}
		return listMsg{items: items, next: stateSelectCluster}
	}
}

func (c *commandRunner) listServices(region, cluster string) tea.Cmd {
	return func() tea.Msg {
		services, err := c.svc.ListServices(context.Background(), region, cluster)
		if err != nil {
			return listMsg{err: err}
		}
		items := make([]list.Item, 0, len(services))
		for _, s := range services {
			items = append(items, item{title: s.Name, desc: "ECS Service"})
		}
		if len(items) == 0 {
			return listMsg{err: fmt.Errorf("no services found in cluster %s", cluster)}
		}
		return listMsg{items: items, next: stateSelectService}
	}
}

func (c *commandRunner) saveCredentials(creds awsx.Credentials, arn string) tea.Cmd {
	return func() tea.Msg {
		if err := c.svc.SaveCredentials(creds, arn); err != nil {
			return resultMsg{err: err}
		}
		return c.checkSession()()
	}
}

func (c *commandRunner) updateRegion(region string) tea.Cmd {
	return func() tea.Msg {
		if err := c.svc.UpdateRegion(region); err != nil {
			return resultMsg{err: err}
		}
		return resultMsg{message: "Region updated to " + region}
	}
}

func (c *commandRunner) getByPath(t secrets.ParamTarget, outDir string) tea.Cmd {
	return func() tea.Msg {
		if err := c.svc.PullByPath(context.Background(), t, outDir); err != nil {
			return resultMsg{err: err}
		}
		return resultMsg{message: "Successfully exported parameters to " + c.outputLabel(outDir)}
	}
}

func (c *commandRunner) getByTaskDef(t secrets.TaskTarget) tea.Cmd {
	return func() tea.Msg {
		if err := c.svc.PullByTaskDef(context.Background(), t); err != nil {
			return resultMsg{err: err}
		}
		return resultMsg{message: "Successfully exported task definition secrets to " + c.outputLabel(t.OutputDir)}
	}
}

func (c *commandRunner) putParameters(t secrets.ParamTarget, fileName string) tea.Cmd {
	return func() tea.Msg {
		success, errs := c.svc.Push(context.Background(), t, fileName)
		if len(errs) > 0 {
			return resultMsg{message: fmt.Sprintf("Uploaded %d parameters (with errors)", success), err: errs[0]}
		}
		return resultMsg{message: fmt.Sprintf("Successfully uploaded %d parameters", success)}
	}
}
