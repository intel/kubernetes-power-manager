package appqos

// AppQoS API Calls + Marshalling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"strconv"
)

const (
	PoolsEndpoint         = "/pools"
	AppsEndpoint          = "/apps"
	PowerProfilesEndpoint = "/power_profiles"
	Username              = "admin"
	Passwd                = "password"
)

// GetPools /pools
func (ac *AppQoSClient) GetPools(address string) ([]Pool, error) {
	httpString := fmt.Sprintf("%s%s", address, PoolsEndpoint)

	req, err := http.NewRequest("GET", httpString, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	receivedJSON, err := ioutil.ReadAll(resp.Body) // This reads raw request body
	if err != nil {
		return nil, err
	}

	allPools := make([]Pool, 0)
	err = json.Unmarshal([]byte(receivedJSON), &allPools)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	return allPools, nil
}

// GetPool /pools/{id}
func (ac *AppQoSClient) GetPool(address string, id int) (*Pool, error) {
	httpString := fmt.Sprintf("%s%s%s%s", address, PoolsEndpoint, "/", strconv.Itoa(id))

	pool := &Pool{}
	req, err := http.NewRequest("GET", httpString, nil)
	if err != nil {
		return pool, err
	}

	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return pool, err
	}
	receivedJSON, err := ioutil.ReadAll(resp.Body) // This reads raw request body
	if err != nil {
		return pool, err
	}

	err = json.Unmarshal([]byte(receivedJSON), pool)
	if err != nil {
		return pool, err
	}

	resp.Body.Close()

	return pool, nil
}

// PostPool /pools
func (ac *AppQoSClient) PostPool(pool *Pool, address string) (string, error) {
	postFailedErr := errors.NewServiceUnavailable("Response status code error")

	payloadBytes, err := json.Marshal(pool)
	if err != nil {
		return "Failed to marshal payload data", err
	}
	body := bytes.NewReader(payloadBytes)

	httpString := fmt.Sprintf("%s%s", address, PoolsEndpoint)
	req, err := http.NewRequest("POST", httpString, body)
	if err != nil {
		return "Failed to create new HTTP POST request", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return "Failed to set header for  HTTP POST request", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	if resp.StatusCode != 201 {
		errStr := fmt.Sprintf("%s%v", "Fail: ", respStr)
		return errStr, postFailedErr
	}

	defer resp.Body.Close()
	successStr := fmt.Sprintf("%s%v", "Success: ", resp.StatusCode)

	return successStr, nil
}

// PutPool /pools/{id}
func (ac *AppQoSClient) PutPool(pool *Pool, address string, id int) (string, error) {
	patchFailedErr := errors.NewServiceUnavailable("Response status code error")

	payloadBytes, err := json.Marshal(pool)
	if err != nil {
		return "Failed to marshal payload data", err
	}
	body := bytes.NewReader(payloadBytes)

	httpString := fmt.Sprintf("%s%s%s%s", address, PoolsEndpoint, "/", strconv.Itoa(id))
	req, err := http.NewRequest("PUT", httpString, body)
	if err != nil {
		return "Failed to create new HTTP PATCH request", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return "Failed to set header for  HTTP PATCH request", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	if resp.StatusCode != 200 {
		errStr := fmt.Sprintf("%s%v", "Fail: ", respStr)
		return errStr, patchFailedErr
	}

	defer resp.Body.Close()
	successStr := fmt.Sprintf("%s%v", "Success: ", resp.StatusCode)

	return successStr, nil
}

// DeletePool /pools/{id}
func (ac *AppQoSClient) DeletePool(address string, id int) error {
	httpString := fmt.Sprintf("%s%s%s%s", address, PoolsEndpoint, "/", strconv.Itoa(id))

	req, err := http.NewRequest("DELETE", httpString, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	if resp.StatusCode != 200 {
		deleteFailedErr := errors.NewServiceUnavailable(buf.String())
		return deleteFailedErr
	}

	defer resp.Body.Close()

	return nil
}

// GetPowerProfiles /power_profiles
func (ac *AppQoSClient) GetPowerProfiles(address string) ([]PowerProfile, error) {
	httpString := fmt.Sprintf("%s%s", address, PowerProfilesEndpoint)

	req, err := http.NewRequest("GET", httpString, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	receivedJSON, err := ioutil.ReadAll(resp.Body) // This reads raw request body
	if err != nil {
		return nil, err
	}

	allPowerProfiles := make([]PowerProfile, 0)
	err = json.Unmarshal([]byte(receivedJSON), &allPowerProfiles)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	return allPowerProfiles, nil
}

// GetPowerProfile /power_profiles/{id}
func (ac *AppQoSClient) GetPowerProfile(address string, id int) (*PowerProfile, error) {
	httpString := fmt.Sprintf("%s%s%s%s", address, PowerProfilesEndpoint, "/", strconv.Itoa(id))

	powerProfile := &PowerProfile{}
	req, err := http.NewRequest("GET", httpString, nil)
	if err != nil {
		return powerProfile, err
	}

	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return powerProfile, err
	}
	receivedJSON, err := ioutil.ReadAll(resp.Body) // This reads raw request body
	if err != nil {
		return powerProfile, err
	}

	err = json.Unmarshal([]byte(receivedJSON), powerProfile)
	if err != nil {
		return powerProfile, err
	}

	resp.Body.Close()

	return powerProfile, nil
}

// PutPowerProfile /power_profiles/{id}
func (ac *AppQoSClient) PutPowerProfile(powerProfile *PowerProfile, address string, id int) (string, error) {
	patchFailedErr := errors.NewServiceUnavailable("Response status code error")

	payloadBytes, err := json.Marshal(powerProfile)
	if err != nil {
		return "Failed to marshal payload data", err
	}
	body := bytes.NewReader(payloadBytes)

	httpString := fmt.Sprintf("%s%s%s%s", address, PowerProfilesEndpoint, "/", strconv.Itoa(id))
	req, err := http.NewRequest("PUT", httpString, body)
	if err != nil {
		return "Failed to create new HTTP PATCH request", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return "Failed to set header for  HTTP PATCH request", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	if resp.StatusCode != 200 {
		errStr := fmt.Sprintf("%s%v", "Fail: ", respStr)
		return errStr, patchFailedErr
	}

	defer resp.Body.Close()
	successStr := fmt.Sprintf("%s%v", "Success: ", resp.StatusCode)

	return successStr, nil
}

// DeletePowerProfile /power_profiles/{id}
func (ac *AppQoSClient) DeletePowerProfile(address string, id int) error {
	httpString := fmt.Sprintf("%s%s%s%s", address, PowerProfilesEndpoint, "/", strconv.Itoa(id))

	req, err := http.NewRequest("DELETE", httpString, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	if resp.StatusCode != 200 {
		deleteFailedErr := errors.NewServiceUnavailable(buf.String())
		return deleteFailedErr
	}

	defer resp.Body.Close()

	return nil
}

// This will be deleted after we get the hardware working with sst
func (ac *AppQoSClient) PostApp(app *App, address string) (string, error) {
	postFailedErr := errors.NewServiceUnavailable("Response status code error")

	payloadBytes, err := json.Marshal(app)
	if err != nil {
		return "Failed to marshal payload", err
	}
	body := bytes.NewReader(payloadBytes)

	httpString := fmt.Sprintf("%s%s", address, AppsEndpoint)
	req, err := http.NewRequest("POST", httpString, body)
	if err != nil {
		return "Failed to create new HTTP POST request", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return "Failed to set header for http post request", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	if resp.StatusCode != 201 {
		errStr := fmt.Sprintf("%s%v", "Fail: ", respStr)
		return errStr, postFailedErr
	}

	defer resp.Body.Close()
	successStr := fmt.Sprintf("%s%v", "Success: ", resp.StatusCode)

	return successStr, nil
}

// This will be deleted after we get the hardware working with sst
func (ac *AppQoSClient) GetApps(address string) ([]App, error) {
	httpString := fmt.Sprintf("%s%s", address, AppsEndpoint)

	req, err := http.NewRequest("GET", httpString, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	receivedJSON, err := ioutil.ReadAll(resp.Body) //This reads raw request body
	if err != nil {
		return nil, err
	}

	allApps := make([]App, 0)
	err = json.Unmarshal([]byte(receivedJSON), &allApps)
	if err != nil {
		emptyMessage := &EmptyMessage{}
		er := json.Unmarshal([]byte(receivedJSON), emptyMessage)
		if er != nil {
			return nil, er
		}

		if *emptyMessage.Message == "No apps in config file" {
			return allApps, nil
		}
		return nil, err
	}

	resp.Body.Close()

	return allApps, nil
}

// This will be deleted after we get the hardware working with sst
func (ac *AppQoSClient) PutApp(app *App, address string, id int) (string, error) {
	patchFailedErr := errors.NewServiceUnavailable("Response status code error")

	payloadBytes, err := json.Marshal(app)
	if err != nil {
		return "Failed to marshal payload data", err
	}
	body := bytes.NewReader(payloadBytes)

	httpString := fmt.Sprintf("%s%s%s%s", address, AppsEndpoint, "/", strconv.Itoa(id))
	req, err := http.NewRequest("PUT", httpString, body)
	if err != nil {
		return "Failed to create new HTTP PATCH request", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return "Failed to set header for  HTTP PATCH request", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	if resp.StatusCode != 200 {
		errStr := fmt.Sprintf("%s%v", "Fail: ", respStr)
		return errStr, patchFailedErr
	}

	defer resp.Body.Close()
	successStr := fmt.Sprintf("%s%v", "Success: ", resp.StatusCode)

	return successStr, nil
}

// This will be deleted after we get the hardware working with sst
func (ac *AppQoSClient) DeleteApp(address string, id int) error {
	httpString := fmt.Sprintf("%s%s%s%s", address, AppsEndpoint, "/", strconv.Itoa(id))

	req, err := http.NewRequest("DELETE", httpString, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(Username, Passwd)
	resp, err := ac.client.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	if resp.StatusCode != 200 {
		deleteFailedErr := errors.NewServiceUnavailable(buf.String())
		return deleteFailedErr
	}

	defer resp.Body.Close()

	return nil
}

func FindProfileByName(profiles []*PowerProfile, profileName string) *PowerProfile {
	for _, profile := range profiles {
		if profile.Name == profileName) {
			return profile
		}
	}

	return &PowerProfile{}
}
