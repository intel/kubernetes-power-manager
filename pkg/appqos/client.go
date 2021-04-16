package appqos

// AppQoS API Calles + Marshalling

import (
	"time"
	//	"crypto/ecdsa"
	//	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	//	"encoding/json"
	//	"fmt"
	"io/ioutil"
	//	"k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	//	"reflect"
	//	"strconv"
	//	"strings"
)

const (
	tlsServerName   = "appqos-nameserver"
)

var certPath = "/etc/certs/public/appqos.crt"
var keyPath = "/etc/certs/private/appqos.key"
var caPath = "/etc/certs/public/ca.crt"

// AppQoSClient is used by the operator to become a client to AppQoS
type AppQoSClient struct {
	client *http.Client
}

func NewOperatorAppQoSClient() (*AppQoSClient, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return &AppQoSClient{}, err
	}
/*
	err = verifyKeyLength(cert)
	if err != nil {
		return &AppQoSClient{}, err
	}
*/
	caCert, err := ioutil.ReadFile(caPath)
	if err != nil {
		return &AppQoSClient{}, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		//ServerName:   tlsServerName,
		InsecureSkipVerify: true,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			//tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			//tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			TLSHandshakeTimeout:   700 * time.Millisecond,
		},
	}

	appQoSClient := &AppQoSClient{
                client: client,
        }

        return appQoSClient, nil
}

// NewDefaultAppQoSClient returns a default client for testing and debugging
func NewDefaultAppQoSClient() *AppQoSClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defaultClient := &http.Client{Transport: tr}
	appQoSClient := &AppQoSClient{
		client: defaultClient,
	}

	return appQoSClient
}
