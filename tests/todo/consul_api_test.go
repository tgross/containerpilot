package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/joyent/containerpilot/discovery"
	cpconsul "github.com/joyent/containerpilot/discovery/consul"
)

/*
The TestWithConsul suite of tests uses Hashicorp's own testutil for managing
a Consul server for testing. The 'consul' binary must be in the $PATH
ref https://github.com/hashicorp/consul/tree/master/testutil
*/

var testServer *testutil.TestServer

func TestWithConsul(t *testing.T) {
	testServer = testutil.NewTestServerConfig(t, func(c *testutil.TestServerConfig) {
		c.LogLevel = "err"
	})
	defer testServer.Stop()
	t.Run("TestConsulTTLPass", testConsulTTLPass)
	t.Run("TestConsulReregister", testConsulReregister)
	t.Run("TestConsulCheckForChanges", testConsulCheckForChanges)
	t.Run("TestConsulEnableTagOverride", testConsulEnableTagOverride)
}

func testConsulTTLPass(t *testing.T) {
	consul, _ := cpconsul.NewConsulConfig(testServer.HTTPAddr)
	service := generateServiceDefinition(fmt.Sprintf("service-TestConsulTTLPass"))
	id := service.ID

	consul.SendHeartbeat(service) // force registration and 1st heartbeat
	checks, _ := consul.Agent().Checks()
	check := checks[id]
	if check.Status != "passing" {
		t.Fatalf("status of check %s should be 'passing' but is %s", id, check.Status)
	}
}

func testConsulReregister(t *testing.T) {
	consul, _ := cpconsul.NewConsulConfig(testServer.HTTPAddr)
	service := generateServiceDefinition(fmt.Sprintf("service-TestConsulReregister"))
	id := service.ID
	consul.SendHeartbeat(service) // force registration and 1st heartbeat
	services, _ := consul.Agent().Services()
	svc := services[id]
	if svc.Address != "192.168.1.1" {
		t.Fatalf("service address should be '192.168.1.1' but is %s", svc.Address)
	}

	// new Consul client (as though we've restarted)
	consul, _ = cpconsul.NewConsulConfig(testServer.HTTPAddr)
	service.IPAddress = "192.168.1.2"
	consul.SendHeartbeat(service) // force re-registration and 1st heartbeat

	services, _ = consul.Agent().Services()
	svc = services[id]
	if svc.Address != "192.168.1.2" {
		t.Fatalf("service address should be '192.168.1.2' but is %s", svc.Address)
	}
}

func testConsulCheckForChanges(t *testing.T) {
	backend := fmt.Sprintf("service-TestConsulCheckForChanges")
	consul, _ := cpconsul.NewConsulConfig(testServer.HTTPAddr)
	service := generateServiceDefinition(backend)
	id := service.ID
	if consul.CheckForUpstreamChanges(backend, "") {
		t.Fatalf("First read of %s should show `false` for change", id)
	}
	consul.SendHeartbeat(service) // force registration and 1st heartbeat

	if !consul.CheckForUpstreamChanges(backend, "") {
		t.Errorf("%v should have changed after first health check TTL", id)
	}
	if consul.CheckForUpstreamChanges(backend, "") {
		t.Errorf("%v should not have changed without TTL expiring", id)
	}
	consul.Agent().UpdateTTL(id, "expired", "critical")
	if !consul.CheckForUpstreamChanges(backend, "") {
		t.Errorf("%v should have changed after TTL expired.", id)
	}
}

func testConsulEnableTagOverride(t *testing.T) {
	backend := fmt.Sprintf("service-TestConsulEnableTagOverride")
	consul, _ := cpconsul.NewConsulConfig(testServer.HTTPAddr)
	service := &discovery.ServiceDefinition{
		ID:        backend,
		Name:      backend,
		IPAddress: "192.168.1.1",
		TTL:       1,
		Port:      9000,
		ConsulExtras: &discovery.ConsulExtras{
			EnableTagOverride: true,
		},
	}
	id := service.ID
	if consul.CheckForUpstreamChanges(backend, "") {
		t.Fatalf("First read of %s should show `false` for change", id)
	}
	consul.SendHeartbeat(service) // force registration
	catalogService, _, err := consul.Catalog().Service(id, "", nil)
	if err != nil {
		t.Fatalf("Error finding service: %v", err)
	}

	for _, service := range catalogService {
		if service.ServiceEnableTagOverride != true {
			t.Errorf("%v should have had EnableTagOverride set to true", id)
		}
	}
}

func generateServiceDefinition(serviceName string) *discovery.ServiceDefinition {
	return &discovery.ServiceDefinition{
		ID:        serviceName,
		Name:      serviceName,
		IPAddress: "192.168.1.1",
		TTL:       5,
		Port:      9000,
	}
}
