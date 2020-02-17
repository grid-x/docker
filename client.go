package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func statusCode(statusCode, want int) error {
	if statusCode != want {
		return fmt.Errorf("invalid response code want=%d, got=%d",
			want, statusCode)
	}
	return nil
}

// Client offers the possibility to communicate with dockerd.
// A local http connection is established via unix socket. This allows to
// create and delete containers and networks.
// Note: this is not a complete docker client implementation.
// Only the requirements for the simulator are covered. And it tries not to
// include docker as an external dependency in the project.
type Client struct {
	http *http.Client
}

const baseAddr = "http://localhost/"

// NewClient returns a new docker client. The arguments are the path to the
// docker sock which is necessary to control dockerd.
// e.g.: c := NewClient(&logger, "/var/run/docker.sock")
func NewClient(sock string) *Client {
	return &Client{
		http: &http.Client{
			Transport: &http.Transport{
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					return net.Dial("unix", sock)
				},
			},
			Timeout: time.Second * 5,
		},
	}
}

// Ping pings the server and returns true if the daemon responds with
// http.StatusOK and false if an error occures.
// docs.: https://docs.docker.com/engine/api/v1.36/#operation/SystemPing
func (c *Client) Ping() bool {
	endpoint := fmt.Sprintf("%s/_ping", baseAddr)
	r, err := c.http.Get(endpoint)
	if err != nil {
		return false
	}
	return statusCode(r.StatusCode, http.StatusOK) == nil
}

// ContainerIDByName returns the containerID for the given name. If this fails,
// an error is returned.
func (c *Client) ContainerIDByName(name string) (string, error) {
	endpoint := fmt.Sprintf("%scontainers/json", baseAddr)
	r, err := c.http.Get(endpoint)
	if err != nil {
		return "", err
	}

	containers := []struct {
		ID     string   `json:"ID"`
		Status string   `json:"Status"`
		Image  string   `json:"Image"`
		Names  []string `json:"Names"`
	}{}

	if err = statusCode(r.StatusCode, http.StatusOK); err != nil {
		return "", err
	}

	if err := json.NewDecoder(r.Body).Decode(&containers); err != nil {
		return "", err
	}

	for _, container := range containers {
		for _, cn := range container.Names {
			if ok := strings.Contains(cn, name); ok {
				return container.ID, nil
			}
		}
	}

	return "", fmt.Errorf("can not extract containerID for %s", name)
}

// CreateContainer tries to create a container with the given name based on the
// image. If this is successful the containerID is returned. If it fails,
// an error is returend.
// cmd and exposedPorts can be used to overwrite the command to execute and
// expose ports when creating a container. Createcontainer gets only minimal set
// of options tailored to the needs of the simulator.
// Cmd should look like this: ["sleep", "3600"]
// ExposedPorts shall be so specified: ["<port>/<tcp|udp>", "<port>/<tcp|udp>"]
// Mounts e.g.: ["/var/run/docker.sock:/var/run/docker.sock"]
// All options can also be left empty. Then the defaults of the image are used.
func (c *Client) CreateContainer(name, image string, cmd, exposedPorts, mounts []string) (string, error) {
	endpoint := fmt.Sprintf("%scontainers/create?name=%s", baseAddr, name)

	type Mount struct {
		Target      string `json:"Target"`
		Source      string `json:"Source"`
		ReadOnly    bool   `json:"ReadOnly"`
		Type        string `json:"Type"`
		Consistency string `json:"Consistency"`
	}

	min := struct {
		Name         string              `json:"Name"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
		Image        string              `json:"Image"`
		Cmd          []string            `json:"Cmd,omitempty"`
		HostConfig   struct {
			Mounts       []Mount `json:"Mounts,omitempty"`
			PortBindings map[string]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings,omitempty"`
		} `json:"HostConfig"`
	}{
		Name:         name,
		Image:        image,
		Cmd:          cmd,
		ExposedPorts: make(map[string]struct{}),
	}
	min.HostConfig.Mounts = make([]Mount, len(mounts))

	for _, port := range exposedPorts {
		min.ExposedPorts[port] = struct{}{}
	}

	for i, m := range mounts {
		if ss := strings.Split(m, ":"); len(ss) == 2 {
			min.HostConfig.Mounts[i] = Mount{
				Source:      ss[0],
				Target:      ss[1],
				Type:        "bind",
				Consistency: "default",
			}
		}
	}

	b, err := json.Marshal(&min)
	if err != nil {
		return "", err
	}

	r, err := c.http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}

	if err := statusCode(r.StatusCode, http.StatusCreated); err != nil {
		return "", err
	}

	res := struct {
		ID       string        `json:"Id"`
		Warnings []interface{} `json:"Warnings"`
	}{}

	return res.ID, json.NewDecoder(r.Body).Decode(&res)
}

// DeleteContainer remove a container by the given ContainerID. If it fails,
// an error is returend.
func (c *Client) DeleteContainer(id string) error {
	endpoint := fmt.Sprintf("%scontainers/%s", baseAddr, id)
	r, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(r)
	if err != nil {
		return err
	}
	return statusCode(resp.StatusCode, http.StatusNoContent)
}

// StartContainer by given containerID. If it fails, an error is returend.
func (c *Client) StartContainer(id string) error {
	endpoint := fmt.Sprintf("%scontainers/%s/start", baseAddr, id)
	r, err := c.http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	return statusCode(r.StatusCode, http.StatusNoContent)
}

// StopContainer by given containerID. If it fails, an error is returend.
func (c *Client) StopContainer(id string) error {
	endpoint := fmt.Sprintf("%scontainers/%s/stop", baseAddr, id)
	r, err := c.http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	return statusCode(r.StatusCode, http.StatusNoContent)
}

// NetworkIDByName returns the networkID for the given Network name.
// if this fails, an error is returned.
func (c *Client) NetworkIDByName(name string) (string, error) {
	endpoint := fmt.Sprintf("%snetworks", baseAddr)
	r, err := c.http.Get(endpoint)
	if err != nil {
		return "", err
	}

	if err = statusCode(r.StatusCode, http.StatusOK); err != nil {
		return "", err
	}

	networks := []struct {
		Driver string `json:"Driver"`
		ID     string `json:"ID"`
		Name   string `json:"Name"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&networks); err != nil {
		return "", err
	}

	for _, n := range networks {
		if ok := strings.Contains(n.Name, name); ok {
			return n.ID, nil
		}
	}
	return "", fmt.Errorf("can not extract containerID for %s", name)
}

// CreateNetwork creates a default network with the given name.
// This network uses the bridge driver and is attachable.
// After success the NetworkID is returned. If it fails, an error is returned.
func (c *Client) CreateNetwork(name string) (string, error) {
	endpoint := fmt.Sprintf("%snetworks/create", baseAddr)

	min := struct {
		Name       string `json:"Name"`
		Driver     string `json:"Driver"`
		Attachable bool   `json:"Attachable"`
	}{
		Name:       name,
		Driver:     "bridge",
		Attachable: true,
	}

	b, err := json.Marshal(&min)
	if err != nil {
		return "", err
	}

	r, err := c.http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}

	if err = statusCode(r.StatusCode, http.StatusCreated); err != nil {
		return "", err
	}

	res := struct {
		ID       string        `json:"Id"`
		Warnings []interface{} `json:"Warnings"`
	}{}

	return res.ID, json.NewDecoder(r.Body).Decode(&res)
}

// DeleteNetwork by the given NetworkID. If it fails an error is returned.
func (c *Client) DeleteNetwork(id string) error {
	endpoint := fmt.Sprintf("%snetworks/%s", baseAddr, id)
	r, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(r)
	if err != nil {
		return err
	}
	return statusCode(resp.StatusCode, http.StatusNoContent)
}

// ConnectNetwork connects a container to a network. for doin this container
// and network are identified by their ID. If it fails an error is returned.
func (c *Client) ConnectNetwork(nwid string, cid string, aliases []string) error {
	endpoint := fmt.Sprintf("%snetworks/%s/connect", baseAddr, nwid)

	type endpointConfig struct {
		Aliases []string `json:"Aliases"`
	}

	min := struct {
		Container      string          `json:"Container"`
		EndpointConfig *endpointConfig `json:"EndpointConfig"`
	}{
		Container: cid,
		EndpointConfig: &endpointConfig{
			Aliases: aliases,
		},
	}

	b, err := json.Marshal(&min)
	if err != nil {
		return err
	}
	r, err := c.http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return statusCode(r.StatusCode, http.StatusOK)
}

// DisconnectNetwork removes a container from a network. container and network
// are identified by theier ID. If it fails, an error is returned.
func (c *Client) DisconnectNetwork(nwid string, cid string) error {
	endpoint := fmt.Sprintf("%snetworks/%s/disconnect", baseAddr, nwid)

	min := struct {
		Container string `json:"Container"`
	}{
		Container: cid,
	}
	b, err := json.Marshal(&min)
	if err != nil {
		return err
	}
	r, err := c.http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return statusCode(r.StatusCode, http.StatusOK)
}

// Labels returns a map of all labels belonging to the given containerID
func (c *Client) Labels(containerID string) (map[string]string, error) {
	r, err := c.http.Get(fmt.Sprintf("%scontainers/%s/json", baseAddr, containerID))
	if err != nil {
		return nil, err
	}

	if err = statusCode(r.StatusCode, http.StatusOK); err != nil {
		return nil, err
	}

	inspect := struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}{}

	return inspect.Config.Labels, json.NewDecoder(r.Body).Decode(&inspect)
}
