//go:build freebsd || linux || darwin
// +build freebsd linux darwin

/*
Copyright 2017 The Kubernetes Authors.
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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"net"
	"net/url"
)

const (
	// unixProtocol is the network protocol of unix socket.
	unixProtocol = "unix"
)

// GetAddressAndDialer returns the address parsed from the given endpoint and a context dialer.
func GetAddressAndDialer(endpoint string) (string, func(ctx context.Context, addr string) (net.Conn, error), error) {
	protocol, addr, err := parseEndpointWithFallbackProtocol(endpoint, unixProtocol)
	if err != nil {
		return "", nil, err
	}
	if protocol != unixProtocol {
		return "", nil, fmt.Errorf("only support unix socket endpoint")
	}

	return addr, dial, nil
}

func dial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, unixProtocol, addr)
}

func parseEndpointWithFallbackProtocol(endpoint string, fallbackProtocol string) (protocol string, addr string, err error) {
	if protocol, addr, err = parseEndpoint(endpoint); err != nil && protocol == "" {
		fallbackEndpoint := fallbackProtocol + "://" + endpoint
		protocol, addr, err = parseEndpoint(fallbackEndpoint)
		if err == nil {
			klog.Warningf("Using %q as endpoint is deprecated, please consider using full url format %q.", endpoint, fallbackEndpoint)
		}
	}
	return
}

func parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", err
	}

	switch u.Scheme {
	case "tcp":
		return "tcp", u.Host, nil

	case "unix":
		return "unix", u.Path, nil

	case "":
		return "", "", fmt.Errorf("using %q as endpoint is deprecated, please consider using full url format", endpoint)

	default:
		return u.Scheme, "", fmt.Errorf("protocol %q not supported", u.Scheme)
	}
}

func CPUInCPUList(cpu uint, cpuList []uint) bool {
	for _, cpuListID := range cpuList {
		if cpuListID == cpu {
			return true
		}
	}

	return false
}

func NodeNameInNodeList(name string, nodeList []corev1.Node) bool {
	for _, node := range nodeList {
		if node.Name == name {
			return true
		}
	}

	return false
}

func StringInStringList(item string, itemList []string) bool {
	for _, i := range itemList {
		if i == item {
			return true
		}
	}

	return false
}

// UnpackErrsToStrings will try to unpack a multi-error to a list of strings, if possible, if not it will return string
// representation as a first element
func UnpackErrsToStrings(err error) *[]string {
	if err == nil {
		return &[]string{}
	}

	switch joinedErr := err.(type) {
	case interface{ Unwrap() []error }:
		stringErrs := make([]string, len(joinedErr.Unwrap()))
		for i, individualErr := range joinedErr.Unwrap() {
			stringErrs[i] = individualErr.Error()
		}
		return &stringErrs
	default:
		return &[]string{err.Error()}
	}
}
