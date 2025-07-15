package api

import (
	"fmt"
	"net/url"
	"strings"
)

// VNCProxyResponse represents the response from a VNC proxy request
type VNCProxyResponse struct {
	Ticket   string `json:"ticket"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Cert     string `json:"cert"`
	Password string `json:"password,omitempty"` // One-time password for WebSocket connections
}

// GetVNCProxy creates a VNC proxy for a VM and returns connection details
func (c *Client) GetVNCProxy(vm *VM) (*VNCProxyResponse, error) {
	c.logger.Info("Creating VNC proxy for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	if vm.Type != VMTypeQemu && vm.Type != VMTypeLXC {
		c.logger.Error("VNC proxy not supported for VM type: %s", vm.Type)
		return nil, fmt.Errorf("VNC proxy only available for QEMU VMs and LXC containers")
	}

	var res map[string]interface{}
	path := fmt.Sprintf("/nodes/%s/%s/%d/vncproxy", vm.Node, vm.Type, vm.ID)

	c.logger.Debug("VNC proxy API path for VM %s: %s", vm.Name, path)

	// POST request with websocket=1 parameter for noVNC compatibility
	data := map[string]interface{}{
		"websocket": 1,
	}

	c.logger.Debug("VNC proxy request data for VM %s: %+v", vm.Name, data)

	if err := c.PostWithResponse(path, data, &res); err != nil {
		c.logger.Error("Failed to create VNC proxy for VM %s: %v", vm.Name, err)
		return nil, fmt.Errorf("failed to create VNC proxy: %w", err)
	}

	c.logger.Debug("VNC proxy API response for VM %s: %+v", vm.Name, res)

	responseData, ok := res["data"].(map[string]interface{})
	if !ok {
		c.logger.Error("Unexpected VNC proxy response format for VM %s", vm.Name)
		return nil, fmt.Errorf("unexpected VNC proxy response format")
	}

	response := &VNCProxyResponse{}

	if ticket, ok := responseData["ticket"].(string); ok {
		response.Ticket = ticket
		c.logger.Debug("VNC proxy ticket obtained for VM %s (length: %d)", vm.Name, len(ticket))
	}

	if port, ok := responseData["port"].(string); ok {
		response.Port = port
		c.logger.Debug("VNC proxy port for VM %s: %s", vm.Name, port)
	} else if portFloat, ok := responseData["port"].(float64); ok {
		response.Port = fmt.Sprintf("%.0f", portFloat)
		c.logger.Debug("VNC proxy port for VM %s (converted from float): %s", vm.Name, response.Port)
	}

	if user, ok := responseData["user"].(string); ok {
		response.User = user
		c.logger.Debug("VNC proxy user for VM %s: %s", vm.Name, user)
	}

	if cert, ok := responseData["cert"].(string); ok {
		response.Cert = cert
		c.logger.Debug("VNC proxy certificate obtained for VM %s (length: %d)", vm.Name, len(cert))
	}

	c.logger.Info("VNC proxy created successfully for VM %s - Port: %s", vm.Name, response.Port)
	return response, nil
}

// GetVNCProxyWithWebSocket creates a VNC proxy for a VM with WebSocket support and one-time password
func (c *Client) GetVNCProxyWithWebSocket(vm *VM) (*VNCProxyResponse, error) {
	c.logger.Info("Creating VNC proxy with WebSocket for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	if vm.Type != VMTypeQemu && vm.Type != VMTypeLXC {
		c.logger.Error("VNC proxy with WebSocket not supported for VM type: %s", vm.Type)
		return nil, fmt.Errorf("VNC proxy only available for QEMU VMs and LXC containers")
	}

	var res map[string]interface{}
	path := fmt.Sprintf("/nodes/%s/%s/%d/vncproxy", vm.Node, vm.Type, vm.ID)

	c.logger.Debug("VNC proxy WebSocket API path for VM %s: %s", vm.Name, path)

	// Different parameters based on VM type
	// LXC containers don't support generate-password parameter
	var data map[string]interface{}
	if vm.Type == VMTypeLXC {
		// LXC containers only support websocket parameter
		data = map[string]interface{}{
			"websocket": 1,
		}
		c.logger.Debug("Using LXC-compatible parameters for VM %s (no generate-password)", vm.Name)
	} else {
		// QEMU VMs support both websocket and generate-password
		data = map[string]interface{}{
			"websocket":         1,
			"generate-password": 1,
		}
		c.logger.Debug("Using QEMU parameters for VM %s (with generate-password)", vm.Name)
	}

	c.logger.Debug("VNC proxy WebSocket request data for VM %s: %+v", vm.Name, data)

	if err := c.PostWithResponse(path, data, &res); err != nil {
		c.logger.Error("Failed to create VNC proxy with WebSocket for VM %s: %v", vm.Name, err)
		return nil, fmt.Errorf("failed to create VNC proxy with WebSocket: %w", err)
	}

	c.logger.Debug("VNC proxy WebSocket API response for VM %s: %+v", vm.Name, res)

	responseData, ok := res["data"].(map[string]interface{})
	if !ok {
		c.logger.Error("Unexpected VNC proxy WebSocket response format for VM %s", vm.Name)
		return nil, fmt.Errorf("unexpected VNC proxy response format")
	}

	response := &VNCProxyResponse{}

	if ticket, ok := responseData["ticket"].(string); ok {
		response.Ticket = ticket
		c.logger.Debug("VNC proxy WebSocket ticket obtained for VM %s (length: %d)", vm.Name, len(ticket))
	}

	if port, ok := responseData["port"].(string); ok {
		response.Port = port
		c.logger.Debug("VNC proxy WebSocket port for VM %s: %s", vm.Name, port)
	} else if portFloat, ok := responseData["port"].(float64); ok {
		response.Port = fmt.Sprintf("%.0f", portFloat)
		c.logger.Debug("VNC proxy WebSocket port for VM %s (converted from float): %s", vm.Name, response.Port)
	}

	if user, ok := responseData["user"].(string); ok {
		response.User = user
		c.logger.Debug("VNC proxy WebSocket user for VM %s: %s", vm.Name, user)
	}

	if cert, ok := responseData["cert"].(string); ok {
		response.Cert = cert
		c.logger.Debug("VNC proxy WebSocket certificate obtained for VM %s (length: %d)", vm.Name, len(cert))
	}

	// Password is only available for QEMU VMs with generate-password=1
	if password, ok := responseData["password"].(string); ok {
		response.Password = password
		c.logger.Debug("VNC proxy one-time password obtained for VM %s (length: %d)", vm.Name, len(password))
	} else if vm.Type == VMTypeLXC {
		c.logger.Debug("No one-time password for LXC container %s (expected behavior)", vm.Name)
	} else {
		c.logger.Debug("No one-time password in response for QEMU VM %s (unexpected)", vm.Name)
	}

	c.logger.Info("VNC proxy with WebSocket created successfully for VM %s - Port: %s, Has Password: %t",
		vm.Name, response.Port, response.Password != "")
	return response, nil
}

// GenerateVNCURL creates a noVNC console URL for the given VM
func (c *Client) GenerateVNCURL(vm *VM) (string, error) {
	c.logger.Info("Generating VNC URL for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Get VNC proxy details
	c.logger.Debug("Requesting VNC proxy for URL generation for VM %s", vm.Name)
	proxy, err := c.GetVNCProxy(vm)
	if err != nil {
		c.logger.Error("Failed to get VNC proxy for URL generation for VM %s: %v", vm.Name, err)
		return "", err
	}

	// Extract server details from base URL
	serverURL := strings.TrimSuffix(c.baseURL, "/api2/json")
	c.logger.Debug("Base server URL for VM %s: %s", vm.Name, serverURL)

	// URL encode the VNC ticket (critical for avoiding 401 errors)
	encodedTicket := url.QueryEscape(proxy.Ticket)
	c.logger.Debug("VNC ticket encoded for VM %s (original length: %d, encoded length: %d)",
		vm.Name, len(proxy.Ticket), len(encodedTicket))

	// Determine console type based on VM type
	consoleType := "kvm"
	if vm.Type == VMTypeLXC {
		consoleType = "lxc"
	}
	c.logger.Debug("Console type for VM %s: %s", vm.Name, consoleType)

	// Build the noVNC console URL using the working format from the forum post
	// Format: https://server:8006/?console=kvm&novnc=1&vmid=100&vmname=vmname&node=nodename&resize=off&cmd=&vncticket=encoded_ticket
	vncURL := fmt.Sprintf("%s/?console=%s&novnc=1&vmid=%d&vmname=%s&node=%s&resize=off&cmd=&vncticket=%s",
		serverURL, consoleType, vm.ID, url.QueryEscape(vm.Name), vm.Node, encodedTicket)

	c.logger.Info("VNC URL generated successfully for VM %s", vm.Name)
	c.logger.Debug("VNC URL for VM %s: %s", vm.Name, vncURL)

	return vncURL, nil
}
