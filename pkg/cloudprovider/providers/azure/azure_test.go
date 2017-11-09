/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	serviceapi "k8s.io/kubernetes/pkg/api/v1/service"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest/to"
)

var testClusterName = "testCluster"

// Test additional of a new service/port.
func TestAddPort(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80)

	svc.Spec.Ports = append(svc.Spec.Ports, v1.ServicePort{
		Name:     fmt.Sprintf("port-udp-%d", 1234),
		Protocol: v1.ProtocolUDP,
		Port:     1234,
		NodePort: getBackendPort(1234),
	})

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	// ensure we got a frontend ip configuration
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have a frontend ip configuration")
	}

	validateLoadBalancer(t, lb, svc)
}

// Test addition of a new service on an internal LB with a subnet.
func TestReconcileLoadBalancerAddServiceOnInternalSubnet(t *testing.T) {
	az := getTestCloud()
	svc := getInternalTestService("servicea", 80)
	addTestSubnet(t, az, &svc)

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	// ensure we got a frontend ip configuration
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have a frontend ip configuration")
	}

	validateLoadBalancer(t, lb, svc)
}

// Test addition of services on an internal LB using both default and explicit subnets.
func TestReconcileLoadBalancerAddServicesOnMultipleSubnets(t *testing.T) {
	az := getTestCloud()
	svc1 := getTestService("service1", v1.ProtocolTCP, 8081)
	svc2 := getInternalTestService("service2", 8081)

	// Internal and External service cannot reside on the same LB resource
	addTestSubnet(t, az, &svc2)

	// svc1 is using LB without "-internal" suffix
	lb, err := az.reconcileLoadBalancer(testClusterName, &svc1, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error reconciling svc1: %q", err)
	}

	// ensure we got a frontend ip configuration for each service
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have 1 frontend ip configurations")
	}

	validateLoadBalancer(t, lb, svc1)

	// svc2 is using LB with "-internal" suffix
	lb, err = az.reconcileLoadBalancer(testClusterName, &svc2, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error reconciling svc2: %q", err)
	}

	// ensure we got a frontend ip configuration for each service
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have 1 frontend ip configurations")
	}

	validateLoadBalancer(t, lb, svc2)
}

// Test moving a service exposure from one subnet to another.
func TestReconcileLoadBalancerEditServiceSubnet(t *testing.T) {
	az := getTestCloud()
	svc := getInternalTestService("service1", 8081)
	addTestSubnet(t, az, &svc)

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error reconciling initial svc: %q", err)
	}

	validateLoadBalancer(t, lb, svc)

	svc.Annotations[ServiceAnnotationLoadBalancerInternalSubnet] = "NewSubnet"
	addTestSubnet(t, az, &svc)

	lb, err = az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error reconciling edits to svc: %q", err)
	}

	// ensure we got a frontend ip configuration for the service
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have 1 frontend ip configuration")
	}

	validateLoadBalancer(t, lb, svc)
}

func TestReconcileLoadBalancerNodeHealth(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80)
	svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	svc.Spec.HealthCheckNodePort = int32(32456)

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	// ensure we got a frontend ip configuration
	if len(*lb.FrontendIPConfigurations) != 1 {
		t.Error("Expected the loadbalancer to have a frontend ip configuration")
	}

	validateLoadBalancer(t, lb, svc)
}

// Test removing all services results in removing the frontend ip configuration
func TestReconcileLoadBalancerRemoveService(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80, 443)

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}
	validateLoadBalancer(t, lb, svc)

	lb, err = az.reconcileLoadBalancer(testClusterName, &svc, nil, false /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	// ensure we abandoned the frontend ip configuration
	if len(*lb.FrontendIPConfigurations) != 0 {
		t.Error("Expected the loadbalancer to have no frontend ip configuration")
	}

	validateLoadBalancer(t, lb)
}

// Test removing all service ports results in removing the frontend ip configuration
func TestReconcileLoadBalancerRemoveAllPortsRemovesFrontendConfig(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80)

	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}
	validateLoadBalancer(t, lb, svc)

	svcUpdated := getTestService("servicea", v1.ProtocolTCP)
	lb, err = az.reconcileLoadBalancer(testClusterName, &svcUpdated, nil, false /* wantLb*/)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	// ensure we abandoned the frontend ip configuration
	if len(*lb.FrontendIPConfigurations) != 0 {
		t.Error("Expected the loadbalancer to have no frontend ip configuration")
	}

	validateLoadBalancer(t, lb, svcUpdated)
}

// Test removal of a port from an existing service.
func TestReconcileLoadBalancerRemovesPort(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80, 443)
	lb, err := az.reconcileLoadBalancer(testClusterName, &svc, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	svcUpdated := getTestService("servicea", v1.ProtocolTCP, 80)
	lb, err = az.reconcileLoadBalancer(testClusterName, &svcUpdated, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateLoadBalancer(t, lb, svcUpdated)
}

// Test reconciliation of multiple services on same port
func TestReconcileLoadBalancerMultipleServices(t *testing.T) {
	az := getTestCloud()
	svc1 := getTestService("servicea", v1.ProtocolTCP, 80, 443)
	svc2 := getTestService("serviceb", v1.ProtocolTCP, 80)

	updatedLoadBalancer, err := az.reconcileLoadBalancer(testClusterName, &svc1, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	updatedLoadBalancer, err = az.reconcileLoadBalancer(testClusterName, &svc2, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateLoadBalancer(t, updatedLoadBalancer, svc1, svc2)
}

func TestReconcileSecurityGroupNewServiceAddsPort(t *testing.T) {
	az := getTestCloud()
	getTestSecurityGroup(az)
	svc1 := getTestService("serviceea", v1.ProtocolTCP, 80)

	// reconcileSecurityGroup need the lb there
	lb, err := az.reconcileLoadBalancer(testClusterName, &svc1, nil, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}
	validateLoadBalancer(t, lb, svc1)

	sg, err := az.reconcileSecurityGroup(testClusterName, &svc1, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateSecurityGroup(t, sg, svc1)
}

func TestReconcileSecurityGroupNewInternalServiceAddsPort(t *testing.T) {
	az := getTestCloud()
	getTestSecurityGroup(az)
	svc1 := getInternalTestService("serviceea", 80)

	sg, err := az.reconcileSecurityGroup(testClusterName, &svc1, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateSecurityGroup(t, sg, svc1)
}

func TestReconcileSecurityGroupRemoveService(t *testing.T) {
	az := getTestCloud()
	service1 := getTestService("servicea", v1.ProtocolTCP, 81)
	service2 := getTestService("serviceb", v1.ProtocolTCP, 82)

	sg := getTestSecurityGroup(az, service1, service2)
	validateSecurityGroup(t, sg, service1, service2)

	sg, err := az.reconcileSecurityGroup(testClusterName, &service1, false /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateSecurityGroup(t, sg, service2)
}

func TestReconcileSecurityGroupRemoveServiceRemovesPort(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80, 443)

	sg := getTestSecurityGroup(az, svc)

	svcUpdated := getTestService("servicea", v1.ProtocolTCP, 80)
	sg, err := az.reconcileSecurityGroup(testClusterName, &svcUpdated, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateSecurityGroup(t, sg, svcUpdated)
}

func TestReconcileSecurityWithSourceRanges(t *testing.T) {
	az := getTestCloud()
	svc := getTestService("servicea", v1.ProtocolTCP, 80, 443)
	svc.Spec.LoadBalancerSourceRanges = []string{
		"192.168.0.0/24",
		"10.0.0.0/32",
	}

	sg := getTestSecurityGroup(az, svc)
	sg, err := az.reconcileSecurityGroup(testClusterName, &svc, true /* wantLb */)
	if err != nil {
		t.Errorf("Unexpected error: %q", err)
	}

	validateSecurityGroup(t, sg, svc)
}

func getTestCloud() *Cloud {
	r := &Cloud{
		Config: Config{
			TenantID:          "tenant",
			SubscriptionID:    "subscription",
			ResourceGroup:     "rg",
			VnetResourceGroup: "rg",
			Location:          "westus",
			VnetName:          "vnet",
			SubnetName:        "subnet",
			SecurityGroupName: "nsg",
			RouteTableName:    "rt",
		},
	}
	r.operationPollRateLimiter = flowcontrol.NewTokenBucketRateLimiter(100, 100)
	r.LoadBalancerClient = NewFakeAzureLBClient()
	r.PublicIPAddressesClient = NewFakeAzurePIPClient()
	r.SubnetsClient = NewFakeAzureSubnetsClient()
	r.SecurityGroupsClient = NewFakeAzureNSGClient()
	return r
}

func getBackendPort(port int32) int32 {
	return port + 10000
}

func getTestPublicFipConfigurationProperties() network.FrontendIPConfigurationPropertiesFormat {
	return network.FrontendIPConfigurationPropertiesFormat{
		PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/this/is/a/public/ip/address/id")},
	}
}

func getTestInternalFipConfigurationProperties(expectedSubnetName *string) network.FrontendIPConfigurationPropertiesFormat {
	var expectedSubnet *network.Subnet
	if expectedSubnetName != nil {
		expectedSubnet = &network.Subnet{Name: expectedSubnetName}
	}
	return network.FrontendIPConfigurationPropertiesFormat{
		PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/this/is/a/public/ip/address/id")},
		Subnet:          expectedSubnet,
	}
}

func getTestService(identifier string, proto v1.Protocol, requestedPorts ...int32) v1.Service {
	ports := []v1.ServicePort{}
	for _, port := range requestedPorts {
		ports = append(ports, v1.ServicePort{
			Name:     fmt.Sprintf("port-tcp-%d", port),
			Protocol: proto,
			Port:     port,
			NodePort: getBackendPort(port),
		})
	}

	svc := v1.Service{
		Spec: v1.ServiceSpec{
			Type:  v1.ServiceTypeLoadBalancer,
			Ports: ports,
		},
	}
	svc.Name = identifier
	svc.Namespace = "default"
	svc.UID = types.UID(identifier)
	svc.Annotations = make(map[string]string)

	return svc
}

func getInternalTestService(identifier string, requestedPorts ...int32) v1.Service {
	svc := getTestService(identifier, v1.ProtocolTCP, requestedPorts...)
	svc.Annotations[ServiceAnnotationLoadBalancerInternal] = "true"

	return svc
}

func getTestLoadBalancer(services ...v1.Service) network.LoadBalancer {
	rules := []network.LoadBalancingRule{}
	probes := []network.Probe{}

	for _, service := range services {
		for _, port := range service.Spec.Ports {
			ruleName := getLoadBalancerRuleName(&service, port, nil)
			rules = append(rules, network.LoadBalancingRule{
				Name: to.StringPtr(ruleName),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					FrontendPort: to.Int32Ptr(port.Port),
					BackendPort:  to.Int32Ptr(port.Port),
				},
			})
			probes = append(probes, network.Probe{
				Name: to.StringPtr(ruleName),
				ProbePropertiesFormat: &network.ProbePropertiesFormat{
					Port: to.Int32Ptr(port.NodePort),
				},
			})
		}
	}

	lb := network.LoadBalancer{
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			LoadBalancingRules: &rules,
			Probes:             &probes,
		},
	}

	return lb
}

func getServiceSourceRanges(service *v1.Service) []string {
	if len(service.Spec.LoadBalancerSourceRanges) == 0 {
		if !requiresInternalLoadBalancer(service) {
			return []string{"Internet"}
		}
	}

	return service.Spec.LoadBalancerSourceRanges
}

func getTestSecurityGroup(az *Cloud, services ...v1.Service) *network.SecurityGroup {
	rules := []network.SecurityRule{}

	for _, service := range services {
		for _, port := range service.Spec.Ports {
			sources := getServiceSourceRanges(&service)
			for _, src := range sources {
				ruleName := getSecurityRuleName(&service, port, src)
				rules = append(rules, network.SecurityRule{
					Name: to.StringPtr(ruleName),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						SourceAddressPrefix:  to.StringPtr(src),
						DestinationPortRange: to.StringPtr(fmt.Sprintf("%d", port.Port)),
					},
				})
			}
		}
	}

	sg := network.SecurityGroup{
		Name: &az.SecurityGroupName,
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &rules,
		},
	}

	az.SecurityGroupsClient.CreateOrUpdate(
		az.ResourceGroup,
		az.SecurityGroupName,
		sg,
		nil)

	return &sg
}

func validateLoadBalancer(t *testing.T, loadBalancer *network.LoadBalancer, services ...v1.Service) {
	expectedRuleCount := 0
	expectedFrontendIPCount := 0
	expectedProbeCount := 0
	expectedFrontendIPs := []ExpectedFrontendIPInfo{}
	for _, svc := range services {
		if len(svc.Spec.Ports) > 0 {
			expectedFrontendIPCount++
			expectedFrontendIP := ExpectedFrontendIPInfo{
				Name:   getFrontendIPConfigName(&svc, subnet(&svc)),
				Subnet: subnet(&svc),
			}
			expectedFrontendIPs = append(expectedFrontendIPs, expectedFrontendIP)
		}
		for _, wantedRule := range svc.Spec.Ports {
			expectedRuleCount++
			wantedRuleName := getLoadBalancerRuleName(&svc, wantedRule, subnet(&svc))
			foundRule := false
			for _, actualRule := range *loadBalancer.LoadBalancingRules {
				if strings.EqualFold(*actualRule.Name, wantedRuleName) &&
					*actualRule.FrontendPort == wantedRule.Port &&
					*actualRule.BackendPort == wantedRule.Port {
					foundRule = true
					break
				}
			}
			if !foundRule {
				t.Errorf("Expected load balancer rule but didn't find it: %q", wantedRuleName)
			}

			// if UDP rule, there is no probe
			if wantedRule.Protocol == v1.ProtocolUDP {
				continue
			}

			expectedProbeCount++
			foundProbe := false
			if serviceapi.NeedsHealthCheck(&svc) {
				path, port := serviceapi.GetServiceHealthCheckPathPort(&svc)
				for _, actualProbe := range *loadBalancer.Probes {
					if strings.EqualFold(*actualProbe.Name, wantedRuleName) &&
						*actualProbe.Port == port &&
						*actualProbe.RequestPath == path &&
						actualProbe.Protocol == network.ProbeProtocolHTTP {
						foundProbe = true
						break
					}
				}
			} else {
				for _, actualProbe := range *loadBalancer.Probes {
					if strings.EqualFold(*actualProbe.Name, wantedRuleName) &&
						*actualProbe.Port == wantedRule.NodePort {
						foundProbe = true
						break
					}
				}
			}
			if !foundProbe {
				for _, actualProbe := range *loadBalancer.Probes {
					t.Logf("Probe: %s %d", *actualProbe.Name, *actualProbe.Port)
				}
				t.Errorf("Expected loadbalancer probe but didn't find it: %q", wantedRuleName)
			}
		}
	}

	frontendIPCount := len(*loadBalancer.FrontendIPConfigurations)
	if frontendIPCount != expectedFrontendIPCount {
		t.Errorf("Expected the loadbalancer to have %d frontend IPs. Found %d.\n%v", expectedFrontendIPCount, frontendIPCount, loadBalancer.FrontendIPConfigurations)
	}

	frontendIPs := *loadBalancer.FrontendIPConfigurations
	for _, expectedFrontendIP := range expectedFrontendIPs {
		if !expectedFrontendIP.existsIn(frontendIPs) {
			t.Errorf("Expected the loadbalancer to have frontend IP %s/%s. Found %s", expectedFrontendIP.Name, to.String(expectedFrontendIP.Subnet), describeFIPs(frontendIPs))
		}
	}

	lenRules := len(*loadBalancer.LoadBalancingRules)
	if lenRules != expectedRuleCount {
		t.Errorf("Expected the loadbalancer to have %d rules. Found %d.\n%v", expectedRuleCount, lenRules, loadBalancer.LoadBalancingRules)
	}

	lenProbes := len(*loadBalancer.Probes)
	if lenProbes != expectedProbeCount {
		t.Errorf("Expected the loadbalancer to have %d probes. Found %d.", expectedRuleCount, lenProbes)
	}
}

type ExpectedFrontendIPInfo struct {
	Name   string
	Subnet *string
}

func (expected ExpectedFrontendIPInfo) matches(frontendIP network.FrontendIPConfiguration) bool {
	return strings.EqualFold(expected.Name, to.String(frontendIP.Name)) && strings.EqualFold(to.String(expected.Subnet), to.String(subnetName(frontendIP)))
}

func (expected ExpectedFrontendIPInfo) existsIn(frontendIPs []network.FrontendIPConfiguration) bool {
	for _, fip := range frontendIPs {
		if expected.matches(fip) {
			return true
		}
	}
	return false
}

func subnetName(frontendIP network.FrontendIPConfiguration) *string {
	if frontendIP.Subnet != nil {
		return frontendIP.Subnet.Name
	}
	return nil
}

func describeFIPs(frontendIPs []network.FrontendIPConfiguration) string {
	description := ""
	for _, actualFIP := range frontendIPs {
		actualSubnetName := ""
		if actualFIP.Subnet != nil {
			actualSubnetName = to.String(actualFIP.Subnet.Name)
		}
		actualFIPText := fmt.Sprintf("%s/%s ", to.String(actualFIP.Name), actualSubnetName)
		description = description + actualFIPText
	}
	return description
}

func validateSecurityGroup(t *testing.T, securityGroup *network.SecurityGroup, services ...v1.Service) {
	expectedRuleCount := 0
	for _, svc := range services {
		for _, wantedRule := range svc.Spec.Ports {
			sources := getServiceSourceRanges(&svc)
			for _, source := range sources {
				wantedRuleName := getSecurityRuleName(&svc, wantedRule, source)
				expectedRuleCount++
				foundRule := false
				for _, actualRule := range *securityGroup.SecurityRules {
					if strings.EqualFold(*actualRule.Name, wantedRuleName) &&
						*actualRule.SourceAddressPrefix == source &&
						*actualRule.DestinationPortRange == fmt.Sprintf("%d", wantedRule.Port) {
						foundRule = true
						break
					}
				}
				if !foundRule {
					t.Errorf("Expected security group rule but didn't find it: %q", wantedRuleName)
				}
			}
		}
	}

	lenRules := len(*securityGroup.SecurityRules)
	if lenRules != expectedRuleCount {
		t.Errorf("Expected the loadbalancer to have %d rules. Found %d.\n", expectedRuleCount, lenRules)
	}
}

func TestSecurityRulePriorityPicksNextAvailablePriority(t *testing.T) {
	rules := []network.SecurityRule{}

	var expectedPriority int32 = loadBalancerMinimumPriority + 50

	var i int32
	for i = loadBalancerMinimumPriority; i < expectedPriority; i++ {
		rules = append(rules, network.SecurityRule{
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Priority: to.Int32Ptr(i),
			},
		})
	}

	priority, err := getNextAvailablePriority(rules)
	if err != nil {
		t.Errorf("Unexpectected error: %q", err)
	}

	if priority != expectedPriority {
		t.Errorf("Expected priority %d. Got priority %d.", expectedPriority, priority)
	}
}

func TestSecurityRulePriorityFailsIfExhausted(t *testing.T) {
	rules := []network.SecurityRule{}

	var i int32
	for i = loadBalancerMinimumPriority; i < loadBalancerMaximumPriority; i++ {
		rules = append(rules, network.SecurityRule{
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Priority: to.Int32Ptr(i),
			},
		})
	}

	_, err := getNextAvailablePriority(rules)
	if err == nil {
		t.Error("Expectected an error. There are no priority levels left.")
	}
}

func TestProtocolTranslationTCP(t *testing.T) {
	proto := v1.ProtocolTCP
	transportProto, securityGroupProto, probeProto, err := getProtocolsFromKubernetesProtocol(proto)
	if err != nil {
		t.Error(err)
	}

	if *transportProto != network.TransportProtocolTCP {
		t.Errorf("Expected TCP LoadBalancer Rule Protocol. Got %v", transportProto)
	}
	if *securityGroupProto != network.SecurityRuleProtocolTCP {
		t.Errorf("Expected TCP SecurityGroup Protocol. Got %v", transportProto)
	}
	if *probeProto != network.ProbeProtocolTCP {
		t.Errorf("Expected TCP LoadBalancer Probe Protocol. Got %v", transportProto)
	}
}

func TestProtocolTranslationUDP(t *testing.T) {
	proto := v1.ProtocolUDP
	transportProto, securityGroupProto, probeProto, _ := getProtocolsFromKubernetesProtocol(proto)
	if *transportProto != network.TransportProtocolUDP {
		t.Errorf("Expected UDP LoadBalancer Rule Protocol. Got %v", transportProto)
	}
	if *securityGroupProto != network.SecurityRuleProtocolUDP {
		t.Errorf("Expected UDP SecurityGroup Protocol. Got %v", transportProto)
	}
	if probeProto != nil {
		t.Errorf("Expected UDP LoadBalancer Probe Protocol. Got %v", transportProto)
	}
}

// Test Configuration deserialization (json)
func TestNewCloudFromJSON(t *testing.T) {
	config := `{
		"tenantId": "--tenant-id--",
		"subscriptionId": "--subscription-id--",
		"aadClientId": "--aad-client-id--",
		"aadClientSecret": "--aad-client-secret--",
		"aadClientCertPath": "--aad-client-cert-path--",
		"aadClientCertPassword": "--aad-client-cert-password--",
		"resourceGroup": "--resource-group--",
		"location": "--location--",
		"subnetName": "--subnet-name--",
		"securityGroupName": "--security-group-name--",
		"vnetName": "--vnet-name--",
		"routeTableName": "--route-table-name--",
		"primaryAvailabilitySetName": "--primary-availability-set-name--",
		"cloudProviderBackoff": true,
		"cloudProviderBackoffRetries": 6,
		"cloudProviderBackoffExponent": 1.5,
		"cloudProviderBackoffDuration": 5,
		"cloudProviderBackoffJitter": 1.0,
		"cloudProviderRatelimit": true,
		"cloudProviderRateLimitQPS": 0.5,
		"cloudProviderRateLimitBucket": 5
	}`
	validateConfig(t, config)
}

// Test Backoff and Rate Limit defaults (json)
func TestCloudDefaultConfigFromJSON(t *testing.T) {
	config := `{
                "aadClientId": "--aad-client-id--",
                "aadClientSecret": "--aad-client-secret--"
        }`

	validateEmptyConfig(t, config)
}

// Test Backoff and Rate Limit defaults (yaml)
func TestCloudDefaultConfigFromYAML(t *testing.T) {
	config := `
aadClientId: --aad-client-id--
aadClientSecret: --aad-client-secret--
`
	validateEmptyConfig(t, config)
}

// Test Configuration deserialization (yaml)
func TestNewCloudFromYAML(t *testing.T) {
	config := `
tenantId: --tenant-id--
subscriptionId: --subscription-id--
aadClientId: --aad-client-id--
aadClientSecret: --aad-client-secret--
aadClientCertPath: --aad-client-cert-path--
aadClientCertPassword: --aad-client-cert-password--
resourceGroup: --resource-group--
location: --location--
subnetName: --subnet-name--
securityGroupName: --security-group-name--
vnetName: --vnet-name--
routeTableName: --route-table-name--
primaryAvailabilitySetName: --primary-availability-set-name--
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 5
cloudProviderBackoffJitter: 1.0
cloudProviderRatelimit: true
cloudProviderRateLimitQPS: 0.5
cloudProviderRateLimitBucket: 5
`
	validateConfig(t, config)
}

func validateConfig(t *testing.T, config string) {
	azureCloud := getCloudFromConfig(t, config)

	if azureCloud.TenantID != "--tenant-id--" {
		t.Errorf("got incorrect value for TenantID")
	}
	if azureCloud.SubscriptionID != "--subscription-id--" {
		t.Errorf("got incorrect value for SubscriptionID")
	}
	if azureCloud.AADClientID != "--aad-client-id--" {
		t.Errorf("got incorrect value for AADClientID")
	}
	if azureCloud.AADClientSecret != "--aad-client-secret--" {
		t.Errorf("got incorrect value for AADClientSecret")
	}
	if azureCloud.AADClientCertPath != "--aad-client-cert-path--" {
		t.Errorf("got incorrect value for AADClientCertPath")
	}
	if azureCloud.AADClientCertPassword != "--aad-client-cert-password--" {
		t.Errorf("got incorrect value for AADClientCertPassword")
	}
	if azureCloud.ResourceGroup != "--resource-group--" {
		t.Errorf("got incorrect value for ResourceGroup")
	}
	if azureCloud.Location != "--location--" {
		t.Errorf("got incorrect value for Location")
	}
	if azureCloud.SubnetName != "--subnet-name--" {
		t.Errorf("got incorrect value for SubnetName")
	}
	if azureCloud.SecurityGroupName != "--security-group-name--" {
		t.Errorf("got incorrect value for SecurityGroupName")
	}
	if azureCloud.VnetName != "--vnet-name--" {
		t.Errorf("got incorrect value for VnetName")
	}
	if azureCloud.RouteTableName != "--route-table-name--" {
		t.Errorf("got incorrect value for RouteTableName")
	}
	if azureCloud.PrimaryAvailabilitySetName != "--primary-availability-set-name--" {
		t.Errorf("got incorrect value for PrimaryAvailabilitySetName")
	}
	if azureCloud.CloudProviderBackoff != true {
		t.Errorf("got incorrect value for CloudProviderBackoff")
	}
	if azureCloud.CloudProviderBackoffRetries != 6 {
		t.Errorf("got incorrect value for CloudProviderBackoffRetries")
	}
	if azureCloud.CloudProviderBackoffExponent != 1.5 {
		t.Errorf("got incorrect value for CloudProviderBackoffExponent")
	}
	if azureCloud.CloudProviderBackoffDuration != 5 {
		t.Errorf("got incorrect value for CloudProviderBackoffDuration")
	}
	if azureCloud.CloudProviderBackoffJitter != 1.0 {
		t.Errorf("got incorrect value for CloudProviderBackoffJitter")
	}
	if azureCloud.CloudProviderRateLimit != true {
		t.Errorf("got incorrect value for CloudProviderRateLimit")
	}
	if azureCloud.CloudProviderRateLimitQPS != 0.5 {
		t.Errorf("got incorrect value for CloudProviderRateLimitQPS")
	}
	if azureCloud.CloudProviderRateLimitBucket != 5 {
		t.Errorf("got incorrect value for CloudProviderRateLimitBucket")
	}
}

func getCloudFromConfig(t *testing.T, config string) *Cloud {
	configReader := strings.NewReader(config)
	cloud, err := NewCloud(configReader)
	if err != nil {
		t.Error(err)
	}
	azureCloud, ok := cloud.(*Cloud)
	if !ok {
		t.Error("NewCloud returned incorrect type")
	}
	return azureCloud
}

// TODO include checks for other appropriate default config parameters
func validateEmptyConfig(t *testing.T, config string) {
	azureCloud := getCloudFromConfig(t, config)

	// backoff should be disabled by default if not explicitly enabled in config
	if azureCloud.CloudProviderBackoff != false {
		t.Errorf("got incorrect value for CloudProviderBackoff")
	}

	// rate limits should be disabled by default if not explicitly enabled in config
	if azureCloud.CloudProviderRateLimit != false {
		t.Errorf("got incorrect value for CloudProviderRateLimit")
	}
}

func TestDecodeInstanceInfo(t *testing.T) {
	response := `{"ID":"_azdev","UD":"0","FD":"99"}`

	faultDomain, err := readFaultDomain(strings.NewReader(response))
	if err != nil {
		t.Error("Unexpected error in ReadFaultDomain")
	}

	if faultDomain == nil {
		t.Error("Fault domain was unexpectedly nil")
	}

	if *faultDomain != "99" {
		t.Error("got incorrect fault domain")
	}
}

func TestSplitProviderID(t *testing.T) {
	providers := []struct {
		providerID string
		name       types.NodeName

		fail bool
	}{
		{
			providerID: CloudProviderName + ":///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachines/k8s-agent-AAAAAAAA-0",
			name:       "k8s-agent-AAAAAAAA-0",
			fail:       false,
		},
		{
			providerID: CloudProviderName + ":/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachines/k8s-agent-AAAAAAAA-0",
			name:       "",
			fail:       true,
		},
		{
			providerID: CloudProviderName + "://",
			name:       "",
			fail:       true,
		},
		{
			providerID: ":///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachines/k8s-agent-AAAAAAAA-0",
			name:       "",
			fail:       true,
		},
		{
			providerID: "aws:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachines/k8s-agent-AAAAAAAA-0",
			name:       "",
			fail:       true,
		},
	}

	for _, test := range providers {
		name, err := splitProviderID(test.providerID)
		if (err != nil) != test.fail {
			t.Errorf("Expected to failt=%t, with pattern %v", test.fail, test)
		}

		if test.fail {
			continue
		}

		if name != test.name {
			t.Errorf("Expected %v, but got %v", test.name, name)
		}

	}
}

func TestMetadataURLGeneration(t *testing.T) {
	metadata := NewInstanceMetadata()
	fullPath := metadata.makeMetadataURL("some/path")
	if fullPath != "http://169.254.169.254/metadata/some/path" {
		t.Errorf("Expected http://169.254.169.254/metadata/some/path saw %s", fullPath)
	}
}

func TestMetadataParsing(t *testing.T) {
	data := `
{
    "interface": [
      {
        "ipv4": {
          "ipAddress": [
            {
              "privateIpAddress": "10.0.1.4",
              "publicIpAddress": "X.X.X.X"
            }
          ],
          "subnet": [
            {
              "address": "10.0.1.0",
              "prefix": "24"
            }
          ]
        },
        "ipv6": {
          "ipAddress": [

          ]
        },
        "macAddress": "002248020E1E"
      }
    ]
}	
`

	network := NetworkMetadata{}
	if err := json.Unmarshal([]byte(data), &network); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	ip := network.Interface[0].IPV4.IPAddress[0].PrivateIP
	if ip != "10.0.1.4" {
		t.Errorf("Unexpected value: %s, expected 10.0.1.4", ip)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, data)
	}))
	defer server.Close()

	metadata := &InstanceMetadata{
		baseURL: server.URL,
	}

	networkJSON := NetworkMetadata{}
	if err := metadata.Object("/some/path", &networkJSON); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(network, networkJSON) {
		t.Errorf("Unexpected inequality:\n%#v\nvs\n%#v", network, networkJSON)
	}
}

func addTestSubnet(t *testing.T, az *Cloud, svc *v1.Service) {
	if svc.Annotations[ServiceAnnotationLoadBalancerInternal] != "true" {
		t.Error("Subnet added to non-internal service")
	}
	subName := svc.Annotations[ServiceAnnotationLoadBalancerInternalSubnet]
	if subName == "" {
		subName = az.SubnetName
	}

	subnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
		az.SubscriptionID,
		az.VnetResourceGroup,
		az.VnetName,
		subName)

	_, errChan := az.SubnetsClient.CreateOrUpdate(az.VnetResourceGroup, az.VnetName, subName,
		network.Subnet{
			ID:   &subnetID,
			Name: &subName,
		}, nil)

	if err := <-errChan; err != nil {
		t.Errorf("Subnet cannot be created or update, %v", err)
	}
	svc.Annotations[ServiceAnnotationLoadBalancerInternalSubnet] = subName
}
