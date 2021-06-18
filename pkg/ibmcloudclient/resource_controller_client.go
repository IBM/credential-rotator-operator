/*
Copyright 2021.

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

package ibmcloudclient

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
)

type ibmCloudClient struct {
	resourceControllerService *resourcecontrollerv2.ResourceControllerV2
}

// NewClient creates our client wrapper object for interacting with IBM Cloud
func NewClient(userAPIKey string) (*ibmCloudClient, error) {
	resourceControllerService, err := initResourceController(userAPIKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to IBM Cloud resource controller service")
	}

	return &ibmCloudClient{
		resourceControllerService: resourceControllerService,
	}, nil
}

// CreateResourceKeyForServiceInstance Create resource key for a service on the IBM Cloud
func (c *ibmCloudClient) CreateResourceKeyForServiceInstance(serviceGUID string) (*resourcecontrollerv2.ResourceKey, error) {
	t := time.Now()
	var keyName = "creds_for_" + strings.Split(serviceGUID, "-")[0] + "_" + t.Format("20060102150405")

	createResourceKeyOptions := c.resourceControllerService.NewCreateResourceKeyOptions(
		keyName,
		serviceGUID,
	)
	createResourceKeyOptions.SetRole("Manager")

	resourceKey, response, err := c.resourceControllerService.CreateResourceKey(createResourceKeyOptions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create resource key")
	}
	if response.StatusCode != 201 {
		return nil, errors.New(fmt.Sprint("Failed creating resource key with error code: %i",
			response.StatusCode))
	}
	if resourceKey == nil {
		return nil, errors.New("Resource key is null")
	}

	return resourceKey, nil
}

// DeleteResourceKey Delete a resource key
func (c *ibmCloudClient) DeleteResourceKey(resourceKeyID string) error {
	deleteResourceKeyOptions := c.resourceControllerService.NewDeleteResourceKeyOptions(
		resourceKeyID,
	)

	response, err := c.resourceControllerService.DeleteResourceKey(deleteResourceKeyOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to delete resource key with ID (%s)", resourceKeyID)
	}
	if response.StatusCode != 204 {
		return fmt.Errorf("Failed to delete resource key (%s) with error code: %d",
			resourceKeyID, response.StatusCode)
	}
	return nil
}

// initResourceController Get handle to the resource controller service
// for a particular user as specified by user API key.
func initResourceController(userAPIKey string) (*resourcecontrollerv2.ResourceControllerV2, error) {
	// Create an IAM authenticator
	authenticator := &core.IamAuthenticator{
		ApiKey: userAPIKey,
	}

	// Create a service options struct
	options := &resourcecontrollerv2.ResourceControllerV2Options{
		Authenticator: authenticator,
		URL:           "https://resource-controller.cloud.ibm.com",
	}

	// Construct the service client
	resourceControllerService, err := resourcecontrollerv2.NewResourceControllerV2(options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init resource contoller")
	}

	return resourceControllerService, nil
}
