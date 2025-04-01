package job

import (
	"bytes"
	"certificate-provider/internal/job/utils"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"

	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
				SM_EXPIRATION_DAYS: 30,
				SM_KEY_ALGO:        KEY_ALGO_ECDSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA512,
			},
			expectedConfig: Config{
				SM_EXPIRATION_DAYS: 30,
				SM_KEY_ALGO:        KEY_ALGO_ECDSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA512,
			},
		},
		{
			name:        "No values set",
			inputConfig: Config{},
			expectedConfig: Config{
				SM_EXPIRATION_DAYS: 90,
				SM_KEY_ALGO:        KEY_ALGO_RSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA256,
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

// TestGenerateCertificate tests the generateCertificate function
func TestGenerateCertificate(t *testing.T) {
	testCases := []struct {
		name             string
		config           Config
		expectedKeyAlgo  string
		expectedSigAlgo  x509.SignatureAlgorithm
		expectedSANs     []string
		expectedLifetime time.Duration
	}{
		{
			name: "RSA Certificate with SHA256",
			config: Config{
				SM_COMMON_NAME:     "test.example.com",
				SM_ORG:             "Test Org",
				SM_COUNTRY:         "US",
				SM_EXPIRATION_DAYS: 30,
				SM_KEY_ALGO:        KEY_ALGO_RSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA256,
				SM_SAN:             "test.example.com,www.test.example.com",
			},
			expectedKeyAlgo:  KEY_ALGO_RSA,
			expectedSigAlgo:  x509.SHA256WithRSA,
			expectedSANs:     []string{"test.example.com", "www.test.example.com"},
			expectedLifetime: 30 * 24 * time.Hour,
		},
		{
			name: "ECDSA Certificate with SHA512",
			config: Config{
				SM_COMMON_NAME:     "ecdsa.example.com",
				SM_ORG:             "ECDSA Org",
				SM_COUNTRY:         "CA",
				SM_EXPIRATION_DAYS: 60,
				SM_KEY_ALGO:        KEY_ALGO_ECDSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA512,
			},
			expectedKeyAlgo:  KEY_ALGO_ECDSA,
			expectedSigAlgo:  x509.ECDSAWithSHA512,
			expectedSANs:     []string{},
			expectedLifetime: 60 * 24 * time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockSecretsManagerClient)
			mockClient.On("UpdateTaskAboutError", mock.Anything, mock.Anything, mock.Anything).
				Return(&sm.SecretTask{UpdatedBy: core.StringPtr(mock.Anything)}, nil)

			privKeyPEM, certPEM := generateCertificate(mockClient, &tc.config)

			// Validate private key
			privKeyBlock, _ := pem.Decode(privKeyPEM)
			assert.NotNil(t, privKeyBlock, "Private key PEM should be valid")

			// Validate certificate
			certBlock, _ := pem.Decode(certPEM)
			assert.NotNil(t, certBlock, "Certificate PEM should be valid")

			cert, err := x509.ParseCertificate(certBlock.Bytes)
			if cert.DNSNames == nil {
				cert.DNSNames = []string{}
			}
			assert.NoError(t, err, "Certificate parsing should succeed")

			// Verify certificate details
			assert.Equal(t, tc.config.SM_COMMON_NAME, cert.Subject.CommonName)
			assert.Equal(t, []string{tc.config.SM_ORG}, cert.Subject.Organization)
			assert.Equal(t, []string{tc.config.SM_COUNTRY}, cert.Subject.Country)
			assert.Equal(t, tc.expectedSigAlgo, cert.SignatureAlgorithm)
			assert.Equal(t, tc.expectedSANs, cert.DNSNames)

			// Verify certificate lifetime
			lifetime := cert.NotAfter.Sub(cert.NotBefore)
			assert.InDelta(t, tc.expectedLifetime, lifetime, float64(time.Hour))

			// Verify key type
			switch tc.expectedKeyAlgo {
			case KEY_ALGO_RSA:
				_, ok := cert.PublicKey.(*rsa.PublicKey)
				assert.True(t, ok, "Should be an RSA public key")
			case KEY_ALGO_ECDSA:
				_, ok := cert.PublicKey.(*ecdsa.PublicKey)
				assert.True(t, ok, "Should be an ECDSA public key")
			}
		})
	}
}

// TestCredentialsPayload tests the payload creation and encoding
func TestCredentialsPayload(t *testing.T) {
	mockClient := new(MockSecretsManagerClient)
	config := Config{
		SM_COMMON_NAME:     "test.example.com",
		SM_EXPIRATION_DAYS: 30,
	}

	privKeyPEM, certPEM := generateCertificate(mockClient, &config)

	payload := CredentialsPayload{
		PRIVATE_KEY_BASE64: base64.StdEncoding.EncodeToString(privKeyPEM),
		CERTIFICATE_BASE64: base64.StdEncoding.EncodeToString(certPEM),
	}

	// Validate base64 encoding
	privKeyDecoded, err := base64.StdEncoding.DecodeString(payload.PRIVATE_KEY_BASE64)
	assert.NoError(t, err, "Private key base64 decoding should succeed")
	assert.True(t, bytes.Equal(privKeyPEM, privKeyDecoded), "Decoded private key should match original")

	certDecoded, err := base64.StdEncoding.DecodeString(payload.CERTIFICATE_BASE64)
	assert.NoError(t, err, "Certificate base64 decoding should succeed")
	assert.True(t, bytes.Equal(certPEM, certDecoded), "Decoded certificate should match original")
}

// TestDeleteCredentials tests the deleteCredentials function
func TestDeleteCredentials(t *testing.T) {
	// Create a mock logger
	mockLogger := utils.NewLogger("secret-task-id", "delete-credentials")

	// Store the original logger and restore it after the test
	originalLogger := logger
	defer func() { logger = originalLogger }()

	// Set the global logger to our mock logger
	logger = mockLogger
	mockClient := new(MockSecretsManagerClient)
	config := Config{
		SM_CREDENTIALS_ID: "test-credentials-id",
	}

	mockTask := &sm.SecretTask{
		UpdatedBy: core.StringPtr("test-user"),
	}

	mockClient.On("ReplaceSecretTask", mock.Anything).
		Return(mockTask, &core.DetailedResponse{StatusCode: 200}, nil)

	deleteCredentials(mockClient, &config)

	mockClient.AssertExpectations(t)
}

// Benchmark key generation performance
func BenchmarkGenerateCertificate(b *testing.B) {
	benchmarkCases := []struct {
		name   string
		config Config
	}{
		{
			name: "RSA Certificate",
			config: Config{
				SM_COMMON_NAME:     "bench.example.com",
				SM_KEY_ALGO:        KEY_ALGO_RSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA256,
				SM_EXPIRATION_DAYS: 30,
			},
		},
		{
			name: "ECDSA Certificate",
			config: Config{
				SM_COMMON_NAME:     "bench.example.com",
				SM_KEY_ALGO:        KEY_ALGO_ECDSA,
				SM_SIGN_ALGO:       SIGN_ALGO_SHA512,
				SM_EXPIRATION_DAYS: 30,
			},
		},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			mockClient := new(MockSecretsManagerClient)
			mockClient.On("UpdateTaskAboutError", mock.Anything, mock.Anything, mock.Anything).
				Return(&sm.SecretTask{UpdatedBy: core.StringPtr(mock.Anything)}, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				generateCertificate(mockClient, &bc.config)
			}
		})
	}
}
