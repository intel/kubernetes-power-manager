/*
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
package util

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type addressAndDialTests struct {
	endpoint      string
	address       string
	hasDial       bool  // as we would only need to check if it's nil or not
	expectedError error //  this is I think mandatory
}

type dialTest struct {
	ctx    context.Context
	addr   string
	hasErr bool
}

/*parseEndpointWithFallbackProtocol Test struct*/
type pewfpTest struct {
	endpoint         string
	fallbackProtocol string
	protocol         string
	addr             string
	hasErr           bool
}

type parseEndpointTest struct {
	endpoint    string
	protocol    string
	addr        string
	expectedErr error
}

type cpuTest struct {
	name     uint
	isInList bool
}

type nodeTest struct {
	nodeName string
	isInList bool
}

type stringTest struct {
	str      string
	isInList bool
}

func TestGetAddressAndDialer(t *testing.T) {

	testCases := []addressAndDialTests{
		{
			endpoint:      "https://google.com/",
			address:       "",
			hasDial:       false,
			expectedError: fmt.Errorf("protocol %q not supported", "https"),
		},
		{
			endpoint:      "unix://google.com",
			address:       "",
			hasDial:       true,
			expectedError: nil,
		},
		{
			endpoint:      "",
			address:       "",
			hasDial:       false,
			expectedError: nil,
		},
		{
			endpoint:      "tcp://google.com",
			address:       "",
			hasDial:       false,
			expectedError: fmt.Errorf("only support unix socket endpoint"),
		},
	}

	for _, want := range testCases {

		addrGot, dialGot, errGot := GetAddressAndDialer(want.endpoint)

		if want.hasDial {
			assert.NotNil(t, dialGot)
		}

		assert.Equal(t, want.address, addrGot, "the expected address and the actual address are not equal")
		assert.Equal(t, want.expectedError, errGot, "expected error and actual error are not equal")

	}

}

func TestDial(t *testing.T) {

	testCases := []dialTest{
		{
			ctx: func() context.Context {
				nil, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return nil
			}(),
			addr:   "https://google.com",
			hasErr: true,
		},
		{
			ctx: func() context.Context {
				nil, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return nil
			}(),
			addr:   "",
			hasErr: true,
		},
		{
			ctx: func() context.Context {
				nil, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				return nil
			}(),
			addr:   "not a valid URL:",
			hasErr: true,
		},
	}

	for _, want := range testCases {
		dialGot, errGot := dial(want.ctx, want.addr)

		if want.hasErr {
			assert.NotNil(t, errGot)

		}
		assert.Nil(t, dialGot)
	}
}

func TestParseEndpointWithFallbackProtocol(t *testing.T) {

	testCases := []addressAndDialTests{
		{
			endpoint:      "https://google.com/",
			address:       "",
			hasDial:       false,
			expectedError: fmt.Errorf("protocol %q not supported", "https"),
		},
		{
			endpoint:      "unix://google.com/hi/there",
			address:       "/hi/there",
			hasDial:       true,
			expectedError: nil,
		},
		{
			endpoint:      "",
			address:       "",
			hasDial:       false,
			expectedError: nil,
		},
		{
			endpoint:      "tcp://google.com",
			address:       "",
			hasDial:       false,
			expectedError: fmt.Errorf("only support unix socket endpoint"),
		},
	}
	for _, want := range testCases {

		addrGot, dialGot, errGot := GetAddressAndDialer(want.endpoint)

		if want.hasDial {
			assert.NotNil(t, dialGot)
		}

		assert.Equal(t, want.address, addrGot, "the expected address and the actual address are not equal")
		assert.Equal(t, want.expectedError, errGot, "expected error and actual error are not equal")

	}
}
func TestParseEndpoint(t *testing.T) {

	testCases := []parseEndpointTest{
		{
			endpoint:    "tcp://www.google.com/",
			protocol:    "tcp",
			addr:        "www.google.com",
			expectedErr: nil,
		},
		{
			endpoint:    "unix://www.bing.com/hi",
			protocol:    "unix",
			addr:        "/hi",
			expectedErr: nil,
		},
		{
			endpoint:    "https://www.github.com/",
			protocol:    "https",
			addr:        "",
			expectedErr: fmt.Errorf("protocol %q not supported", "https"),
		},
		{
			endpoint:    "error",
			protocol:    "",
			addr:        "",
			expectedErr: fmt.Errorf("using %q as endpoint is deprecated, please consider using full url format", "error"),
		},
		{
			endpoint:    "",
			protocol:    "",
			addr:        "",
			expectedErr: fmt.Errorf("using %q as endpoint is deprecated, please consider using full url format", ""),
		},
	}
	for _, want := range testCases {
		protoGot, addrGot, errGot := parseEndpoint(want.endpoint)

		assert.Equal(t, want.protocol, protoGot, "the expected protocol and the actual protocol are not equal")
		assert.Equal(t, want.addr, addrGot, "the expected address and the actual address are not equal")
		assert.Equal(t, want.expectedErr, errGot, "the expected error and the actual error are not equal")
	}
}

func TestCPUInCPUList(t *testing.T) {

	cpuList := []uint{1222267, 811198, 0}

	testCases := []cpuTest{

		{
			name:     1222267,
			isInList: true,
		},
		{
			name:     44,
			isInList: false,
		},
	}

	var nilList []uint = nil

	for _, want := range testCases {

		resultTrue := CPUInCPUList(want.name, cpuList)
		assert.Equal(t, want.isInList, resultTrue, "One variable that is expected to be in the List or not be in the list was handeled incorrectly")

		resultFalse := CPUInCPUList(want.name, nilList)
		assert.Equal(t, false, resultFalse, "nilList should be null")
	}

}

func TestNodeNameInNodeList(t *testing.T) {
	nodeList := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Node1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Node2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "!",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "",
			},
		},
	}
	var nilList []corev1.Node = nil
	testCases := []nodeTest{
		{
			nodeName: "Node1",
			isInList: true,
		},
		{
			nodeName: "",
			isInList: true,
		},
		{
			nodeName: "?",
			isInList: false,
		},
		{
			nodeName: "node10",
			isInList: false,
		},
	}

	for _, want := range testCases {

		resultTrue := NodeNameInNodeList(want.nodeName, nodeList)
		assert.Equal(t, want.isInList, resultTrue, "One variable that is expected to be in the list or not be in the list was handeled incorrectly")

		resultFalse := NodeNameInNodeList(want.nodeName, nilList)
		assert.Equal(t, false, resultFalse, "nilList should be empty")
	}

}

func TestStringInStringList(t *testing.T) {
	itemList := []string{"aa", "b", "3", "5", "", "!"}
	var nilList []string = nil

	testCases := []stringTest{
		{
			str:      "aa",
			isInList: true,
		},
		{
			str:      "",
			isInList: true,
		},
		{
			str:      "!",
			isInList: true,
		},
		{
			str:      "hi",
			isInList: false,
		},
		{
			str:      "-",
			isInList: false,
		},
	}
	for _, want := range testCases {

		resultTrue := StringInStringList(want.str, itemList)
		assert.Equal(t, want.isInList, resultTrue, "One variable that is expected to be in the list or not be in the list was handeled incorrectly")

		resultFalse := StringInStringList(want.str, nilList)
		assert.Equal(t, false, resultFalse, "nilList should be empty")
	}

}

func TestUnpackErrsToStrings(t *testing.T) {
	assert.Equal(t, &[]string{}, UnpackErrsToStrings(nil))

	// single error
	const errString1 = "err1"
	assert.Equal(t, UnpackErrsToStrings(fmt.Errorf(errString1)), &[]string{errString1})

	// wrapped err
	const errString2 = "err2"
	assert.Equal(
		t,
		UnpackErrsToStrings(errors.Join(fmt.Errorf(errString1), fmt.Errorf(errString2))),
		&[]string{errString1, errString2},
	)

}
