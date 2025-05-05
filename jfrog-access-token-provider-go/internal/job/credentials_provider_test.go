package job

import (
	"fmt"
	"github.com/IBM/go-sdk-core/v5/core"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"jfrog-access-token-provider-go/internal/utils"
	"net/http"
	"testing"
)

const (
	JFrogValidAccessToken = "jfrog-valid-access-token"
	JFrogValidTokenId     = "jfrog-valid-token-id"
)

// MockSecretsManagerClient is a mock implementation of SecretsManagerClient
type MockSecretsManagerClient struct {
	mock.Mock
}

func (m *MockSecretsManagerClient) UpdateTaskAboutError(config Config, code, description string) (*sm.SecretTask, error) {
	args := m.Called(config, code, description)
	return args.Get(0).(*sm.SecretTask), args.Error(1)
}

func (m *MockSecretsManagerClient) UpdateTaskAboutCredentialsCreated(config Config, payload CredentialsPayload) (*sm.SecretTask, error) {
	args := m.Called(config, payload)
	return args.Get(0).(*sm.SecretTask), args.Error(1)
}

func (m *MockSecretsManagerClient) UpdateTaskAboutCredentialsDeleted(config Config) (*sm.SecretTask, error) {
	args := m.Called(config)
	return args.Get(0).(*sm.SecretTask), args.Error(1)
}

// Mock implementation of GetSecret
func (m *MockSecretsManagerClient) GetSecret(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
	args := m.Called(options)

	var secret sm.SecretIntf
	if args.Get(0) != nil {
		secret = args.Get(0).(sm.SecretIntf)
	}
	var response *core.DetailedResponse
	if args.Get(1) != nil {
		response = args.Get(1).(*core.DetailedResponse)
	}
	return secret, response, args.Error(2)
}

// Mock implementation of ReplaceSecretTask
func (m *MockSecretsManagerClient) ReplaceSecretTask(
	options *sm.ReplaceSecretTaskOptions,
) (*sm.SecretTask, *core.DetailedResponse, error) {
	args := m.Called(options)

	var task *sm.SecretTask
	if args.Get(0) != nil {
		task = args.Get(0).(*sm.SecretTask)
	}

	var response *core.DetailedResponse
	if args.Get(1) != nil {
		response = args.Get(1).(*core.DetailedResponse)
	}

	var err error
	if args.Get(2) != nil {
		err = args.Error(2)
	}

	return task, response, err
}

// Mock implementation of NewSecretTaskError
func (m *MockSecretsManagerClient) NewSecretTaskError(code, description string) (*sm.SecretTaskError, error) {
	args := m.Called(code, description)

	var taskError *sm.SecretTaskError
	if args.Get(0) != nil {
		taskError = args.Get(0).(*sm.SecretTaskError)
	}
	return taskError, args.Error(1)
}

// Mock implementation of NewCustomCredentialsNewCredentials
func (m *MockSecretsManagerClient) NewCustomCredentialsNewCredentials(id string, credentials map[string]interface{}) (*sm.CustomCredentialsNewCredentials, error) {
	args := m.Called(id, credentials)

	var customCredentials *sm.CustomCredentialsNewCredentials
	if args.Get(0) != nil {
		customCredentials = args.Get(0).(*sm.CustomCredentialsNewCredentials)
	}
	return customCredentials, args.Error(1)
}

// MockRestyClient is a mock implementation of RestyClient
type MockRestyClient struct {
	mock.Mock
}

func (m *MockRestyClient) Post(authToken string, body interface{}, url string) (*resty.Response, error) {
	args := m.Called(authToken, body, url)
	return args.Get(0).(*resty.Response), args.Error(1)
}

func (m *MockRestyClient) Delete(authToken string, url string) (*resty.Response, error) {
	args := m.Called(authToken, url)
	return args.Get(0).(*resty.Response), args.Error(1)
}

// TestSetDefaultValues tests the setDefaultValues function
func TestSetDefaultValues(t *testing.T) {
	testCases := []struct {
		name           string
		inputConfig    Config
		expectedConfig Config
	}{
		{
			name: "All values set",
			inputConfig: Config{
				SM_USERNAME:                "username",
				SM_SCOPE:                   "scope",
				SM_EXPIRES_IN_SECONDS:      3600,
				SM_REFRESHABLE:             true,
				SM_DESCRIPTION:             "description",
				SM_AUDIENCE:                "audience",
				SM_INCLUDE_REFERENCE_TOKEN: false,
			},
			expectedConfig: Config{
				SM_USERNAME:                "username",
				SM_SCOPE:                   "scope",
				SM_EXPIRES_IN_SECONDS:      3600,
				SM_REFRESHABLE:             true,
				SM_DESCRIPTION:             "description",
				SM_AUDIENCE:                "audience",
				SM_INCLUDE_REFERENCE_TOKEN: false,
			},
		},
		{
			name:        "No values set",
			inputConfig: Config{},
			expectedConfig: Config{
				SM_SCOPE:                   "applied-permissions/user",
				SM_EXPIRES_IN_SECONDS:      7776000,
				SM_REFRESHABLE:             false,
				SM_AUDIENCE:                "*@*",
				SM_INCLUDE_REFERENCE_TOKEN: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setDefaultValues(&tc.inputConfig)
			assert.Equal(t, tc.expectedConfig, tc.inputConfig)
		})
	}
}

// TestCreateJFrogAccessToken tests the createJFrogAccessToken function
func TestCreateJFrogAccessToken(t *testing.T) {
	JFrogServiceCredentialsSecretBearerToken := "jfrog-bearer-token"
	loginSecretId := "login-secret-id"

	// Create a mock logger
	mockLogger := utils.NewLogger("secret-task-id", "create-jfrog-access-token")

	// Store the original logger and restore it after the test
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Set the global logger to our mock logger
	logger = mockLogger

	// Create a mock IBM Cloud Secrets Manager client
	mockSMClient := new(MockSecretsManagerClient)
	mockSMClient.On("GetSecret", mock.Anything).
		Return(&sm.ArbitrarySecret{
			Payload: &JFrogServiceCredentialsSecretBearerToken,
			ID:      &loginSecretId,
		},
			&core.DetailedResponse{
				StatusCode: http.StatusOK,
			},
			nil)

	// Create a mock config
	mockConfig := Config{}

	// Create a mock Resty client
	mockRestyClient := new(MockRestyClient)
	resp := resty.Response{
		RawResponse: &http.Response{
			StatusCode: http.StatusOK,
		},
	}
	resp.SetBody([]byte(fmt.Sprintf(`{"access_token": "%s", "token_id": "%s"}`, JFrogValidAccessToken, JFrogValidTokenId)))

	mockRestyClient.On("Post", mock.Anything, mock.Anything, mock.Anything).
		Return(&resp, nil)

	accessToken, tokenId, err := createJFrogAccessToken(mockSMClient, mockRestyClient, &mockConfig)

	// Validate access token
	assert.Equal(t, JFrogValidAccessToken, accessToken)
	assert.Equal(t, JFrogValidTokenId, tokenId)
	assert.Nil(t, err)
}

// TestRevokeJFrogAccessToken tests the revokeJFrogAccessToken function
func TestRevokeJFrogAccessToken(t *testing.T) {
	JFrogServiceCredentialsSecretBearerToken := "jfrog-bearer-token"
	loginSecretId := "login-secret-id"

	// Create a mock logger
	mockLogger := utils.NewLogger("secret-task-id", "revoke-jfrog-access-token")

	// Store the original logger and restore it after the test
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Set the global logger to our mock logger
	logger = mockLogger

	// Create a mock IBM Cloud Secrets Manager client
	mockSMClient := new(MockSecretsManagerClient)
	mockSMClient.On("GetSecret", mock.Anything).
		Return(&sm.ArbitrarySecret{
			Payload: &JFrogServiceCredentialsSecretBearerToken,
			ID:      &loginSecretId,
		},
			&core.DetailedResponse{
				StatusCode: http.StatusOK,
			},
			nil)

	// Create a mock config
	mockConfig := Config{
		SM_CREDENTIALS_ID: JFrogValidTokenId,
	}

	// Create a mock Resty client
	mockRestyClient := new(MockRestyClient)
	resp := resty.Response{
		RawResponse: &http.Response{
			StatusCode: http.StatusNoContent,
		},
	}

	mockRestyClient.On("Delete", mock.Anything, mock.Anything).
		Return(&resp, nil)

	err := revokeJFrogAccessToken(mockSMClient, mockRestyClient, &mockConfig)

	// Validate no error
	assert.Nil(t, err)
}
