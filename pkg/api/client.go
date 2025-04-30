package api

import (
    "context"
    "crypto/tls"
    "fmt"
    "net/http"

    "github.com/Telmate/proxmox-api-go/proxmox"
)

// Client is a Proxmox API client
type Client struct {
    client *proxmox.Client
}

// NewClient initializes a new Proxmox API client
func NewClient(addr, user, password string, insecure bool) (*Client, error) {
    tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
    httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
    proxClient, err := proxmox.NewClient(addr, httpClient, "", tlsConfig, "", 300)
    if err != nil {
        return nil, err
    }
    if err := proxClient.Login(context.Background(), user, password, ""); err != nil {
        return nil, err
    }
    return &Client{client: proxClient}, nil
}

// Node represents a Proxmox cluster node.
type Node struct {
    ID   string
    Name string
}

// ListNodes retrieves all nodes from the cluster.
func (c *Client) ListNodes() ([]Node, error) {
    raw, err := c.client.GetNodeList(context.Background())
    if err != nil {
        return nil, err
    }
    data, ok := raw["data"].([]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected format for node list")
    }
    nodes := make([]Node, len(data))
    for i, item := range data {
        m := item.(map[string]interface{})
        name := m["node"].(string)
        nodes[i] = Node{ID: name, Name: name}
    }
    return nodes, nil
}

// VM represents a Proxmox VM or container.
type VM struct {
    ID   int
    Name string
    Node string
    Type string
}

// ListVMs retrieves all virtual machines on the given node.
func (c *Client) ListVMs(nodeName string) ([]VM, error) {
    raw, err := c.client.GetVmList(context.Background())
    if err != nil {
        return nil, err
    }
    data, ok := raw["data"].([]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected format for VM list")
    }
    var vms []VM
    for _, item := range data {
        m := item.(map[string]interface{})
        if m["node"].(string) != nodeName {
            continue
        }
        id := int(m["vmid"].(float64))
        name := m["name"].(string)
        tp, _ := m["type"].(string)
        vms = append(vms, VM{ID: id, Name: name, Node: nodeName, Type: tp})
    }
    return vms, nil
}

// GetNodeStatus retrieves metrics for a given node from Proxmox API.
func (c *Client) GetNodeStatus(nodeName string) (map[string]interface{}, error) {
    var res map[string]interface{}
    if err := c.client.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/status", nodeName), &res, 3); err != nil {
        return nil, err
    }
    data, ok := res["data"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected format for node status")
    }
    return data, nil
}

// GetNodeConfig retrieves configuration for a given node.
func (c *Client) GetNodeConfig(nodeName string) (map[string]interface{}, error) {
    var res map[string]interface{}
    if err := c.client.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/config", nodeName), &res, 3); err != nil {
        return nil, err
    }
    data, ok := res["data"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected format for node config")
    }
    return data, nil
}

// GetClusterStatus retrieves cluster status items from Proxmox API.
// Parses the data array into a map keyed by node name.
func (c *Client) GetClusterStatus() (map[string]map[string]interface{}, error) {
    var res map[string]interface{}
    if err := c.client.GetJsonRetryable(context.Background(), "/cluster/status", &res, 3); err != nil {
        return nil, err
    }
    // Interpret data as a slice of node status objects
    dataSlice, ok := res["data"].([]interface{})
    if !ok {
        return nil, fmt.Errorf("unexpected format for cluster status data")
    }
    items := make(map[string]map[string]interface{}, len(dataSlice))
    for _, v := range dataSlice {
        m, ok := v.(map[string]interface{})
        if !ok {
            continue
        }
        if name, ok := m["name"].(string); ok {
            items[name] = m
        }
    }
    return items, nil
}

// TODO: add methods: StartVM, StopVM, etc.
