// Code generated by counterfeiter. DO NOT EDIT.
package fake

import (
	"sync"

	"github.com/hyperledger/fabric-x-common/common/deliverclient/blocksprovider"
	"github.com/hyperledger/fabric-x-common/common/deliverclient/orderers"
)

type OrdererConnectionSource struct {
	RandomEndpointStub        func() (*orderers.Endpoint, error)
	randomEndpointMutex       sync.RWMutex
	randomEndpointArgsForCall []struct {
	}
	randomEndpointReturns struct {
		result1 *orderers.Endpoint
		result2 error
	}
	randomEndpointReturnsOnCall map[int]struct {
		result1 *orderers.Endpoint
		result2 error
	}
	ShuffledEndpointsStub        func() []*orderers.Endpoint
	shuffledEndpointsMutex       sync.RWMutex
	shuffledEndpointsArgsForCall []struct {
	}
	shuffledEndpointsReturns struct {
		result1 []*orderers.Endpoint
	}
	shuffledEndpointsReturnsOnCall map[int]struct {
		result1 []*orderers.Endpoint
	}
	UpdateStub        func([]string, map[string]orderers.OrdererOrg)
	updateMutex       sync.RWMutex
	updateArgsForCall []struct {
		arg1 []string
		arg2 map[string]orderers.OrdererOrg
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *OrdererConnectionSource) RandomEndpoint() (*orderers.Endpoint, error) {
	fake.randomEndpointMutex.Lock()
	ret, specificReturn := fake.randomEndpointReturnsOnCall[len(fake.randomEndpointArgsForCall)]
	fake.randomEndpointArgsForCall = append(fake.randomEndpointArgsForCall, struct {
	}{})
	stub := fake.RandomEndpointStub
	fakeReturns := fake.randomEndpointReturns
	fake.recordInvocation("RandomEndpoint", []interface{}{})
	fake.randomEndpointMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *OrdererConnectionSource) RandomEndpointCallCount() int {
	fake.randomEndpointMutex.RLock()
	defer fake.randomEndpointMutex.RUnlock()
	return len(fake.randomEndpointArgsForCall)
}

func (fake *OrdererConnectionSource) RandomEndpointCalls(stub func() (*orderers.Endpoint, error)) {
	fake.randomEndpointMutex.Lock()
	defer fake.randomEndpointMutex.Unlock()
	fake.RandomEndpointStub = stub
}

func (fake *OrdererConnectionSource) RandomEndpointReturns(result1 *orderers.Endpoint, result2 error) {
	fake.randomEndpointMutex.Lock()
	defer fake.randomEndpointMutex.Unlock()
	fake.RandomEndpointStub = nil
	fake.randomEndpointReturns = struct {
		result1 *orderers.Endpoint
		result2 error
	}{result1, result2}
}

func (fake *OrdererConnectionSource) RandomEndpointReturnsOnCall(i int, result1 *orderers.Endpoint, result2 error) {
	fake.randomEndpointMutex.Lock()
	defer fake.randomEndpointMutex.Unlock()
	fake.RandomEndpointStub = nil
	if fake.randomEndpointReturnsOnCall == nil {
		fake.randomEndpointReturnsOnCall = make(map[int]struct {
			result1 *orderers.Endpoint
			result2 error
		})
	}
	fake.randomEndpointReturnsOnCall[i] = struct {
		result1 *orderers.Endpoint
		result2 error
	}{result1, result2}
}

func (fake *OrdererConnectionSource) ShuffledEndpoints() []*orderers.Endpoint {
	fake.shuffledEndpointsMutex.Lock()
	ret, specificReturn := fake.shuffledEndpointsReturnsOnCall[len(fake.shuffledEndpointsArgsForCall)]
	fake.shuffledEndpointsArgsForCall = append(fake.shuffledEndpointsArgsForCall, struct {
	}{})
	stub := fake.ShuffledEndpointsStub
	fakeReturns := fake.shuffledEndpointsReturns
	fake.recordInvocation("ShuffledEndpoints", []interface{}{})
	fake.shuffledEndpointsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *OrdererConnectionSource) ShuffledEndpointsCallCount() int {
	fake.shuffledEndpointsMutex.RLock()
	defer fake.shuffledEndpointsMutex.RUnlock()
	return len(fake.shuffledEndpointsArgsForCall)
}

func (fake *OrdererConnectionSource) ShuffledEndpointsCalls(stub func() []*orderers.Endpoint) {
	fake.shuffledEndpointsMutex.Lock()
	defer fake.shuffledEndpointsMutex.Unlock()
	fake.ShuffledEndpointsStub = stub
}

func (fake *OrdererConnectionSource) ShuffledEndpointsReturns(result1 []*orderers.Endpoint) {
	fake.shuffledEndpointsMutex.Lock()
	defer fake.shuffledEndpointsMutex.Unlock()
	fake.ShuffledEndpointsStub = nil
	fake.shuffledEndpointsReturns = struct {
		result1 []*orderers.Endpoint
	}{result1}
}

func (fake *OrdererConnectionSource) ShuffledEndpointsReturnsOnCall(i int, result1 []*orderers.Endpoint) {
	fake.shuffledEndpointsMutex.Lock()
	defer fake.shuffledEndpointsMutex.Unlock()
	fake.ShuffledEndpointsStub = nil
	if fake.shuffledEndpointsReturnsOnCall == nil {
		fake.shuffledEndpointsReturnsOnCall = make(map[int]struct {
			result1 []*orderers.Endpoint
		})
	}
	fake.shuffledEndpointsReturnsOnCall[i] = struct {
		result1 []*orderers.Endpoint
	}{result1}
}

func (fake *OrdererConnectionSource) Update(arg1 []string, arg2 map[string]orderers.OrdererOrg) {
	var arg1Copy []string
	if arg1 != nil {
		arg1Copy = make([]string, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.updateMutex.Lock()
	fake.updateArgsForCall = append(fake.updateArgsForCall, struct {
		arg1 []string
		arg2 map[string]orderers.OrdererOrg
	}{arg1Copy, arg2})
	stub := fake.UpdateStub
	fake.recordInvocation("Update", []interface{}{arg1Copy, arg2})
	fake.updateMutex.Unlock()
	if stub != nil {
		fake.UpdateStub(arg1, arg2)
	}
}

func (fake *OrdererConnectionSource) UpdateCallCount() int {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	return len(fake.updateArgsForCall)
}

func (fake *OrdererConnectionSource) UpdateCalls(stub func([]string, map[string]orderers.OrdererOrg)) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = stub
}

func (fake *OrdererConnectionSource) UpdateArgsForCall(i int) ([]string, map[string]orderers.OrdererOrg) {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	argsForCall := fake.updateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *OrdererConnectionSource) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.randomEndpointMutex.RLock()
	defer fake.randomEndpointMutex.RUnlock()
	fake.shuffledEndpointsMutex.RLock()
	defer fake.shuffledEndpointsMutex.RUnlock()
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *OrdererConnectionSource) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ blocksprovider.OrdererConnectionSource = new(OrdererConnectionSource)
