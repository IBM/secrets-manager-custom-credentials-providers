package job

import (
	"bytes"
	"encoding/json"
	"github.com/IBM/go-sdk-core/v5/core"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"slack-rotation-provider-go/internal/utils"
	"testing"
)

// MockSecretsManagerClient implements SecretsManagerClient for testing
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
	if args.Get(0) != nil {
		return args.Get(0).(func(*sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error))(options)
	}
	return nil, nil, nil
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
func (m *MockRestyClient) PostWithFormData(data map[string]string, res interface{}, url string) (*resty.Response, error) {
	args := m.Called(data, url)
	return args.Get(0).(*resty.Response), args.Error(1)
}

func (m *MockRestyClient) Delete(authToken string, url string) (*resty.Response, error) {
	args := m.Called(authToken, url)
	return args.Get(0).(*resty.Response), args.Error(1)
}
func TestCreateSlackToken(t *testing.T) {
	slackExchangeToken := `{"refresh_token":"your_refresh_token_value","access_token":"your_access_token_value","client_id":"your_client_id_value","client_secret":"your_client_secret_value"}`
	currentSecretCredentials := map[string]interface{}{
		"SLACK_REFRESH_TOKEN": "current_secret_refresh_token",
	}
	slackExchangeTokenId := "someId"
	// Create a mock logger
	mockLogger := utils.NewLogger("secret-task-id", "create-jfrog-access-token")

	// Store the original logger and restore it after the test
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Set the global logger to our mock logger
	logger = mockLogger

	// Create a mock IBM Cloud Secrets Manager client
	mockSMClient := MockSecretsManagerClient{}

	// Create a mock config
	mockConfig := Config{}
	mockConfig.SM_EXCHANGE_TOKENS_SECRET_ID = "some_exchange_tokens_id"
	mockConfig.SM_SECRET_ID = "current_secret_id"

	mockSMClient.On("GetSecret", mock.Anything).
		Return(func(gso *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
			if *gso.ID == mockConfig.SM_EXCHANGE_TOKENS_SECRET_ID {
				return &sm.ArbitrarySecret{
					Payload: &slackExchangeToken,
					ID:      &slackExchangeTokenId,
				}, &core.DetailedResponse{StatusCode: http.StatusOK}, nil
			}

			if *gso.ID == mockConfig.SM_SECRET_ID {
				var versionsTotal int64 = 1
				return &sm.CustomCredentialsSecret{
					CredentialsContent: currentSecretCredentials,
					VersionsTotal:      &versionsTotal,
				}, &core.DetailedResponse{StatusCode: http.StatusOK}, nil
			}

			return nil, nil, nil
		})

	// Create a mock Resty client
	mockRestyClient := new(MockRestyClient)

	slackRes := SlackRenewTokenResponse{AccessToken: "new_access_token", RefreshToken: "new_refresh_token", Ok: true, Error: ""}
	jsonBytes, _ := json.Marshal(slackRes)

	mockResp := new(resty.Response)
	mockResp.RawResponse = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonBytes)),
	}
	mockResp.Request = &resty.Request{
		Result: &slackRes,
	}

	// Only safe if your code only uses RawResponse and not internal methods
	mockRestyClient.On("PostWithFormData", mock.Anything, "https://slack.com/api/oauth.v2.access").
		Return(mockResp, nil)

	accessToken, refreshToken, err := createSlackAccessToken(&mockSMClient, mockRestyClient, &mockConfig)

	assert.Nil(t, err)
	assert.NotNil(t, accessToken)
	assert.Equal(t, accessToken, "new_access_token")
	assert.Equal(t, refreshToken, "new_refresh_token")
}
func TestCreateSlackTokenWithError(t *testing.T) {
	slackExchangeToken := `{"refresh_token":"your_refresh_token_value","access_token":"your_access_token_value","client_id":"your_client_id_value","client_secret":"your_client_secret_value"}`
	currentSecretCredentials := map[string]interface{}{
		"SLACK_REFRESH_TOKEN": "current_secret_refresh_token",
	}
	slackExchangeTokenId := "someId"
	// Create a mock logger
	mockLogger := utils.NewLogger("secret-task-id", "create-jfrog-access-token")

	// Store the original logger and restore it after the test
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Set the global logger to our mock logger
	logger = mockLogger

	// Create a mock IBM Cloud Secrets Manager client
	mockSMClient := MockSecretsManagerClient{}

	mockConfig := Config{}
	mockConfig.SM_EXCHANGE_TOKENS_SECRET_ID = "some_exchange_tokens_id"
	mockConfig.SM_SECRET_ID = "current_secret_id"

	mockSMClient.On("GetSecret", mock.Anything).
		Return(func(gso *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
			if *gso.ID == mockConfig.SM_EXCHANGE_TOKENS_SECRET_ID {
				return &sm.ArbitrarySecret{
					Payload: &slackExchangeToken,
					ID:      &slackExchangeTokenId,
				}, &core.DetailedResponse{StatusCode: http.StatusOK}, nil
			}

			if *gso.ID == mockConfig.SM_SECRET_ID {
				var versionsTotal int64 = 1
				return &sm.CustomCredentialsSecret{
					CredentialsContent: currentSecretCredentials,
					VersionsTotal:      &versionsTotal,
				}, &core.DetailedResponse{StatusCode: http.StatusOK}, nil
			}

			return nil, nil, nil
		})

	// Create a mock Resty client
	mockRestyClient := new(MockRestyClient)

	slackRes := SlackRenewTokenResponse{AccessToken: "", RefreshToken: "", Ok: false, Error: "some error"}
	jsonBytes, _ := json.Marshal(slackRes)

	mockResp := new(resty.Response)
	mockResp.RawResponse = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(jsonBytes)),
	}
	mockResp.Request = &resty.Request{
		Result: &slackRes,
	}

	// Only safe if your code only uses RawResponse and not internal methods
	mockRestyClient.On("PostWithFormData", mock.Anything, "https://slack.com/api/oauth.v2.access").
		Return(mockResp, nil)

	accessToken, refreshToken, err := createSlackAccessToken(&mockSMClient, mockRestyClient, &mockConfig)
	assert.NotNil(t, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
	assert.Equal(t, err.Error(), "Slack error: some error")
}
