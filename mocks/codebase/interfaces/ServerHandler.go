// Code generated by mockery v2.49.1. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// ServerHandler is an autogenerated mock type for the ServerHandler type
type ServerHandler struct {
	mock.Mock
}

// MountHandlers provides a mock function with given fields: group
func (_m *ServerHandler) MountHandlers(group any) {
	_m.Called(group)
}

// NewServerHandler creates a new instance of ServerHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewServerHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *ServerHandler {
	mock := &ServerHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
