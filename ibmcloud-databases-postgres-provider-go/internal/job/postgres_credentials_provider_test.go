package job

import (
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
)

// MockSecretsManagerClient implements SecretsManagerClient for testing
type MockSecretsManagerClient struct {
	GetSecretFunc                          func(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error)
	ReplaceSecretTaskFunc                  func(options *sm.ReplaceSecretTaskOptions) (*sm.SecretTask, *core.DetailedResponse, error)
	NewSecretTaskErrorFunc                 func(code, description string) (*sm.SecretTaskError, error)
	NewCustomCredentialsNewCredentialsFunc func(id string, credentials map[string]interface{}) (*sm.CustomCredentialsNewCredentials, error)
}

func (m *MockSecretsManagerClient) GetSecret(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
	return m.GetSecretFunc(options)
}

func (m *MockSecretsManagerClient) ReplaceSecretTask(options *sm.ReplaceSecretTaskOptions) (*sm.SecretTask, *core.DetailedResponse, error) {
	return m.ReplaceSecretTaskFunc(options)
}

func (m *MockSecretsManagerClient) NewSecretTaskError(code, description string) (*sm.SecretTaskError, error) {
	return m.NewSecretTaskErrorFunc(code, description)
}

func (m *MockSecretsManagerClient) NewCustomCredentialsNewCredentials(id string, credentials map[string]interface{}) (*sm.CustomCredentialsNewCredentials, error) {
	return m.NewCustomCredentialsNewCredentialsFunc(id, credentials)
}

// Example test
func TestGetSecret(t *testing.T) {
	// Setup mock
	mockClient := &MockSecretsManagerClient{
		GetSecretFunc: func(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
			// Verify input
			if *options.ID != "test-secret-id" {
				t.Errorf("Expected ID 'test-secret-id', got '%s'", *options.ID)
			}

			// Return mock data
			secret := &sm.ArbitrarySecret{
				Name: core.StringPtr("TestSecret"),
				ID:   core.StringPtr("test-secret-id"),
			}

			return secret, &core.DetailedResponse{StatusCode: 200}, nil
		},
	}

	// Call function under test
	result, err := GetSecret(mockClient, "test-secret-id")

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	secret, ok := result.(*sm.ArbitrarySecret)
	if !ok {
		t.Error("Result is not the expected type")
	}

	if *secret.Name != "TestSecret" {
		t.Errorf("Expected name 'TestSecret', got '%s'", *secret.Name)
	}
}
