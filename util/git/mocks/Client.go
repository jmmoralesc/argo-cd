// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	git "github.com/argoproj/argo-cd/util/git"
	mock "github.com/stretchr/testify/mock"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// Checkout provides a mock function with given fields: revision
func (_m *Client) Checkout(revision string) error {
	ret := _m.Called(revision)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(revision)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CommitSHA provides a mock function with given fields:
func (_m *Client) CommitSHA() (string, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Fetch provides a mock function with given fields: revision
func (_m *Client) Fetch(revision string) error {
	ret := _m.Called(revision)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(revision)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Init provides a mock function with given fields:
func (_m *Client) Init() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LsFiles provides a mock function with given fields: path
func (_m *Client) LsFiles(path string) ([]string, error) {
	ret := _m.Called(path)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LsLargeFiles provides a mock function with given fields:
func (_m *Client) LsLargeFiles() ([]string, error) {
	ret := _m.Called()

	var r0 []string
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LsRefs provides a mock function with given fields:
func (_m *Client) LsRefs() (*git.Refs, error) {
	ret := _m.Called()

	var r0 *git.Refs
	if rf, ok := ret.Get(0).(func() *git.Refs); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*git.Refs)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LsRemote provides a mock function with given fields: revision
func (_m *Client) LsRemote(revision string) (string, error) {
	ret := _m.Called(revision)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(revision)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(revision)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RevisionMetadata provides a mock function with given fields: revision
func (_m *Client) RevisionMetadata(revision string) (*git.RevisionMetadata, error) {
	ret := _m.Called(revision)

	var r0 *git.RevisionMetadata
	if rf, ok := ret.Get(0).(func(string) *git.RevisionMetadata); ok {
		r0 = rf(revision)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*git.RevisionMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(revision)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Root provides a mock function with given fields:
func (_m *Client) Root() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// VerifyCommitSignature provides a mock function with given fields: _a0
func (_m *Client) VerifyCommitSignature(_a0 string) (string, error) {
	ret := _m.Called(_a0)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
