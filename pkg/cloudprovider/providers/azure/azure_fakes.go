package azure

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
)

type fakeAzureLBClient struct {
	fakeStore map[string]map[string]network.LoadBalancer
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
	if _, ok := fLBC.fakeStore[resourceGroupName]; !ok {
		fLBC.fakeStore[resourceGroupName] = map[string]network.LoadBalancer{}
	}
	fLBC.fakeStore[resourceGroupName][loadBalancerName] = parameters
	result = fLBC.fakeStore[resourceGroupName][loadBalancerName]
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
	if _, ok := fLBC.fakeStore[resourceGroupName]; ok {
		if _, ok := fLBC.fakeStore[resourceGroupName][loadBalancerName]; ok {
			delete(fLBC.fakeStore[resourceGroupName], loadBalancerName)
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
	if _, ok := fLBC.fakeStore[resourceGroupName]; ok {
		if entity, ok := fLBC.fakeStore[resourceGroupName][loadBalancerName]; ok {
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
	if _, ok := fLBC.fakeStore[resourceGroupName]; ok {
		for _, v := range fLBC.fakeStore[resourceGroupName] {
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
