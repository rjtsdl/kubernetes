package azure

import (
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
)

type fakeAzureLBClient struct {
	FakeStore map[string]map[string]network.LoadBalancer
}

func NewFakeAzureLBClient() fakeAzureLBClient {
	fLBC := fakeAzureLBClient{}
	fLBC.FakeStore = make(map[string]map[string]network.LoadBalancer)
	return fLBC
}

func (fLBC fakeAzureLBClient) CreateOrUpdate(resourceGroupName string, loadBalancerName string, parameters network.LoadBalancer, cancel <-chan struct{}) (<-chan network.LoadBalancer, <-chan error) {
	resultChan := make(chan network.LoadBalancer, 1)
	errChan := make(chan error, 1)
	var result network.LoadBalancer
	var err error
	defer func() {
		resultChan <- result
		errChan <- err
		close(resultChan)
		close(errChan)
	}()
	if _, ok := fLBC.FakeStore[resourceGroupName]; !ok {
		fLBC.FakeStore[resourceGroupName] = make(map[string]network.LoadBalancer)
	}
	fLBC.FakeStore[resourceGroupName][loadBalancerName] = parameters
	result = fLBC.FakeStore[resourceGroupName][loadBalancerName]
	err = nil
	return resultChan, errChan
}

func (fLBC fakeAzureLBClient) Delete(resourceGroupName string, loadBalancerName string, cancel <-chan struct{}) (<-chan autorest.Response, <-chan error) {
	respChan := make(chan autorest.Response, 1)
	errChan := make(chan error, 1)
	var resp autorest.Response
	var err error
	defer func() {
		respChan <- resp
		errChan <- err
		close(respChan)
		close(errChan)
	}()
	if _, ok := fLBC.FakeStore[resourceGroupName]; ok {
		if _, ok := fLBC.FakeStore[resourceGroupName][loadBalancerName]; ok {
			delete(fLBC.FakeStore[resourceGroupName], loadBalancerName)
			resp.Response = &http.Response{
				StatusCode: http.StatusAccepted,
			}
			err = nil
		}
	}
	resp.Response = &http.Response{
		StatusCode: http.StatusNotFound,
	}
	err = autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such LB",
	}
	return respChan, errChan
}

func (fLBC fakeAzureLBClient) Get(resourceGroupName string, loadBalancerName string, expand string) (result network.LoadBalancer, err error) {
	if _, ok := fLBC.FakeStore[resourceGroupName]; ok {
		if entity, ok := fLBC.FakeStore[resourceGroupName][loadBalancerName]; ok {
			return entity, nil
		}
	}
	return result, autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such LB",
	}
}

func (fLBC fakeAzureLBClient) List(resourceGroupName string) (result network.LoadBalancerListResult, err error) {
	var value []network.LoadBalancer
	if _, ok := fLBC.FakeStore[resourceGroupName]; ok {
		for _, v := range fLBC.FakeStore[resourceGroupName] {
			value = append(value, v)
		}
	}
	result.Response.Response = &http.Response{
		StatusCode: http.StatusOK,
	}
	result.NextLink = nil
	result.Value = &value
	return result, nil
}

type fakeAzurePIPClient struct {
	FakeStore map[string]map[string]network.PublicIPAddress
}

func NewFakeAzurePIPClient() fakeAzurePIPClient {
	fAPC := fakeAzurePIPClient{}
	fAPC.FakeStore = make(map[string]map[string]network.PublicIPAddress)
	return fAPC
}

func (fAPC fakeAzurePIPClient) CreateOrUpdate(resourceGroupName string, publicIPAddressName string, parameters network.PublicIPAddress, cancel <-chan struct{}) (<-chan network.PublicIPAddress, <-chan error) {
	resultChan := make(chan network.PublicIPAddress, 1)
	errChan := make(chan error, 1)
	var result network.PublicIPAddress
	var err error
	defer func() {
		resultChan <- result
		errChan <- err
		close(resultChan)
		close(errChan)
	}()
	if _, ok := fAPC.FakeStore[resourceGroupName]; !ok {
		fAPC.FakeStore[resourceGroupName] = make(map[string]network.PublicIPAddress)
	}
	fAPC.FakeStore[resourceGroupName][publicIPAddressName] = parameters
	result = fAPC.FakeStore[resourceGroupName][publicIPAddressName]
	err = nil
	return resultChan, errChan
}

func (fAPC fakeAzurePIPClient) Delete(resourceGroupName string, publicIPAddressName string, cancel <-chan struct{}) (<-chan autorest.Response, <-chan error) {
	respChan := make(chan autorest.Response, 1)
	errChan := make(chan error, 1)
	var resp autorest.Response
	var err error
	defer func() {
		respChan <- resp
		errChan <- err
		close(respChan)
		close(errChan)
	}()
	if _, ok := fAPC.FakeStore[resourceGroupName]; ok {
		if _, ok := fAPC.FakeStore[resourceGroupName][publicIPAddressName]; ok {
			delete(fAPC.FakeStore[resourceGroupName], publicIPAddressName)
			resp.Response = &http.Response{
				StatusCode: http.StatusAccepted,
			}
			err = nil
		}
	}
	resp.Response = &http.Response{
		StatusCode: http.StatusNotFound,
	}
	err = autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such PIP",
	}
	return respChan, errChan
}

func (fAPC fakeAzurePIPClient) Get(resourceGroupName string, publicIPAddressName string, expand string) (result network.PublicIPAddress, err error) {
	if _, ok := fAPC.FakeStore[resourceGroupName]; ok {
		if entity, ok := fAPC.FakeStore[resourceGroupName][publicIPAddressName]; ok {
			return entity, nil
		}
	}
	return result, autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such PIP",
	}
}

func (fAPC fakeAzurePIPClient) List(resourceGroupName string) (result network.PublicIPAddressListResult, err error) {
	var value []network.PublicIPAddress
	if _, ok := fAPC.FakeStore[resourceGroupName]; ok {
		for _, v := range fAPC.FakeStore[resourceGroupName] {
			value = append(value, v)
		}
	}
	result.Response.Response = &http.Response{
		StatusCode: http.StatusOK,
	}
	result.NextLink = nil
	result.Value = &value
	return result, nil
}

type fakeAzureSubnetsClient struct {
	FakeStore map[string]map[string]network.Subnet
}

func NewFakeAzureSubnetsClient() fakeAzureSubnetsClient {
	fASC := fakeAzureSubnetsClient{}
	fASC.FakeStore = make(map[string]map[string]network.Subnet)
	return fASC
}

func (fASC fakeAzureSubnetsClient) CreateOrUpdate(resourceGroupName string, virtualNetworkName string, subnetName string, subnetParameters network.Subnet, cancel <-chan struct{}) (<-chan network.Subnet, <-chan error) {
	resultChan := make(chan network.Subnet, 1)
	errChan := make(chan error, 1)
	var result network.Subnet
	var err error
	defer func() {
		resultChan <- result
		errChan <- err
		close(resultChan)
		close(errChan)
	}()
	rgVnet := strings.Join([]string{resourceGroupName, virtualNetworkName}, "AND")
	if _, ok := fASC.FakeStore[rgVnet]; !ok {
		fASC.FakeStore[rgVnet] = make(map[string]network.Subnet)
	}
	fASC.FakeStore[rgVnet][subnetName] = subnetParameters
	result = fASC.FakeStore[rgVnet][subnetName]
	err = nil
	return resultChan, errChan
}

func (fASC fakeAzureSubnetsClient) Delete(resourceGroupName string, virtualNetworkName string, subnetName string, cancel <-chan struct{}) (<-chan autorest.Response, <-chan error) {
	respChan := make(chan autorest.Response, 1)
	errChan := make(chan error, 1)
	var resp autorest.Response
	var err error
	defer func() {
		respChan <- resp
		errChan <- err
		close(respChan)
		close(errChan)
	}()

	rgVnet := strings.Join([]string{resourceGroupName, virtualNetworkName}, "AND")
	if _, ok := fASC.FakeStore[rgVnet]; ok {
		if _, ok := fASC.FakeStore[rgVnet][subnetName]; ok {
			delete(fASC.FakeStore[rgVnet], subnetName)
			resp.Response = &http.Response{
				StatusCode: http.StatusAccepted,
			}
			err = nil
		}
	}
	resp.Response = &http.Response{
		StatusCode: http.StatusNotFound,
	}
	err = autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such Subnet",
	}
	return respChan, errChan
}
func (fASC fakeAzureSubnetsClient) Get(resourceGroupName string, virtualNetworkName string, subnetName string, expand string) (result network.Subnet, err error) {
	rgVnet := strings.Join([]string{resourceGroupName, virtualNetworkName}, "AND")
	if _, ok := fASC.FakeStore[rgVnet]; ok {
		if entity, ok := fASC.FakeStore[rgVnet][subnetName]; ok {
			return entity, nil
		}
	}
	return result, autorest.DetailedError{
		StatusCode: http.StatusNotFound,
		Message:    "Not such Subnet",
	}
}
func (fASC fakeAzureSubnetsClient) List(resourceGroupName string, virtualNetworkName string) (result network.SubnetListResult, err error) {
	rgVnet := strings.Join([]string{resourceGroupName, virtualNetworkName}, "AND")
	var value []network.Subnet
	if _, ok := fASC.FakeStore[rgVnet]; ok {
		for _, v := range fASC.FakeStore[rgVnet] {
			value = append(value, v)
		}
	}
	result.Response.Response = &http.Response{
		StatusCode: http.StatusOK,
	}
	result.NextLink = nil
	result.Value = &value
	return result, nil
}
