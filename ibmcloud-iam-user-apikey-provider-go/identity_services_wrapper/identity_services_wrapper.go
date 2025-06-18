package identity_services_wrapper

import (
	"errors"
	"fmt"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/iamidentityv1"
	"github.com/mitchellh/mapstructure"
)

/*

A thin wrapper around IAM Identity Service API (https://cloud.ibm.com/apidocs/iam-identity-token-api?code=go).

It exposes 2 functions:
- CreateApiKey - creates a new locked API key
- DeleteApiKey - unlocks and deletes an API key (if it exists)


*/

type Wrapper interface {
	CreateApiKey(options *CreateOptions) (*ApiKey, error)
	DeleteApiKey(apikeyId string) error
}

type wrapper struct {
	client *iamidentityv1.IamIdentityV1
}

type ApiKey struct {
	ID        string
	CRN       string
	IamID     string
	AccountID string
	ApiKey    string
}

type CreateOptions struct {
	Name             string
	Description      string
	IamID            string
	AccountID        string
	SupportSessions  bool
	ActionWhenLeaked string
}

// New initializes the identity services wrapper
func New(url string, apikey string) (Wrapper, error) {

	serviceClientOptions := &iamidentityv1.IamIdentityV1Options{
		URL: url,
		Authenticator: &core.IamAuthenticator{
			ApiKey: apikey,
			URL:    url,
		},
	}
	serviceClient, err := iamidentityv1.NewIamIdentityV1UsingExternalConfig(serviceClientOptions)

	if err != nil {
		return nil, err
	}

	return &wrapper{
		client: serviceClient,
	}, nil
}

// CreateApiKey creates a new API with the given options. The API key will be created enabled and locked.
func (w *wrapper) CreateApiKey(options *CreateOptions) (*ApiKey, error) {
	resultApiKey, _, err := w.client.CreateAPIKey(buildOptions(options))
	if err != nil {
		return nil, err
	}
	return &ApiKey{
		ID:        *resultApiKey.ID,
		CRN:       *resultApiKey.CRN,
		IamID:     *resultApiKey.IamID,
		AccountID: *resultApiKey.AccountID,
		ApiKey:    *resultApiKey.Apikey,
	}, nil
}

// DeleteApiKey deletes the given API key associated with the given ID if it exists.
// if the API key isn't found returns nil without making any changes.
func (w *wrapper) DeleteApiKey(apikeyId string) error {
	found, err := w.unlockApiKey(apikeyId)
	if err != nil {
		return err
	}
	if !found {
		// trying to delete an API key that does not exist
		return nil
	}
	deleteAPIKeyOptions := w.client.NewDeleteAPIKeyOptions(apikeyId)
	_, err = w.client.DeleteAPIKey(deleteAPIKeyOptions)
	return err
}

func buildOptions(options *CreateOptions) *iamidentityv1.CreateAPIKeyOptions {
	createOpts := &iamidentityv1.CreateAPIKeyOptions{
		Name:            core.StringPtr(options.Name),
		IamID:           core.StringPtr(options.IamID),
		Description:     core.StringPtr(options.Description),
		AccountID:       core.StringPtr(options.AccountID),
		SupportSessions: core.BoolPtr(options.SupportSessions),
		EntityLock:      core.StringPtr("true"),
		EntityDisable:   core.StringPtr("false"),
	}
	if options.ActionWhenLeaked != "" {
		createOpts.ActionWhenLeaked = core.StringPtr(options.ActionWhenLeaked)
	}

	return createOpts
}

// unlocks and API key if it exists. Returns a boolean value indicating the existence of the API key
func (w *wrapper) unlockApiKey(apikeyID string) (bool, error) {
	unlockAPIKeyOptions := w.client.NewUnlockAPIKeyOptions(apikeyID)
	resp, err := w.client.UnlockAPIKey(unlockAPIKeyOptions)

	if err == nil && resp != nil && resp.StatusCode == 204 {
		// the API key was found and was successfully unlocked
		return true, nil
	}

	if isApiKeyNotFound(resp) {
		// an API key with the given ID does not exist
		return false, nil
	}
	if err != nil {
		// an error has occurred
		return false, err
	}

	// no error, but unexpected response
	return false, errors.New(fmt.Sprintf("unexpected %d response from IAM when attempting to unlock API key.", resp.StatusCode))

}

type errorResponse struct {
	Errors []struct {
		Code        string `mapstructure:"code"`
		Details     string `mapstructure:"details"`
		Message     string `mapstructure:"message"`
		MessageCode string `mapstructure:"message_code"`
	} `mapstructure:"errors"`
}

// checks if the response indicates that the given API key does not exist.
func isApiKeyNotFound(resp *core.DetailedResponse) bool {
	if resp != nil && resp.GetStatusCode() == 404 {
		if resMap, ok := resp.GetResultAsMap(); ok {
			errResp := errorResponse{}
			err := mapstructure.Decode(resMap, &errResp)
			if err != nil {
				return false
			}
			return len(errResp.Errors) == 1 && errResp.Errors[0].Code == "not_found"
		}
	}
	return false
}
