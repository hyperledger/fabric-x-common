/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mocks

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

type MockACLProvider struct {
	// create a mock object that can be reset after
	// registering a MockACLProvider with aclmgmt
	mock *mock.Mock
}

// clear the mock so we can start afresh
func (m *MockACLProvider) Reset() {
	m.mock = &mock.Mock{}
}

func (m *MockACLProvider) CheckACL(resName string, channelID string, idinfo interface{}) error {
	args := m.mock.Called(resName, channelID, idinfo)
	return args.Error(0)
}

func (m *MockACLProvider) CheckACLNoChannel(resName string, idinfo interface{}) error {
	args := m.mock.Called(resName, idinfo)
	return args.Error(0)
}

// On overrider the mock method for convenience
func (m *MockACLProvider) On(methodName string, arguments ...interface{}) *mock.Call {
	return m.mock.On(methodName, arguments...)
}

// AssertExpectations overrider the mock method for convenience
func (m *MockACLProvider) AssertExpectations(t *testing.T) {
	m.mock.AssertExpectations(t)
}
