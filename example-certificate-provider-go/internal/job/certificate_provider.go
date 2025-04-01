package job

import (
	"certificate-provider/internal/job/utils"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
)

const (
	KEY_ALGO_RSA     = "RSA"
	KEY_ALGO_ECDSA   = "ECDSA"
	SIGN_ALGO_SHA256 = "SHA256"
	SIGN_ALGO_SHA512 = "SHA512"
)

var logger *utils.Logger

// This job generates self-signed SSL/TLS certificates for development and testing only.

// Run runs the main logic of the application.
func Run() {

	config, err := ConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
		os.Exit(1)
	}

	client, err := NewSecretsManagerClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		os.Exit(1)
	}

	logger = utils.NewLogger(config.SM_SECRET_TASK_ID, config.SM_ACTION)

	switch config.SM_ACTION {
	case sm.SecretTask_Type_CreateCredentials:
		generateCredentials(client, &config)
	case sm.SecretTask_Type_DeleteCredentials:
		deleteCredentials(client, &config)

	default:
		updateTaskAboutErrorAndExit(client, &config, "Err10001", fmt.Sprintf("unknown action: '%s'", config.SM_ACTION))
	}

}

// generateCredentials generates the credentials for the given secret
func generateCredentials(client SecretsManagerClient, config *Config) {
	// Set default values for non required config variables if not set by the user
	setDefaultValues(config)

	// Generate private key and certificate
	privKeyPEM, certPEM := generateCertificate(client, config)

	// Create credentials payload
	credentialsPayload := CredentialsPayload{
		PRIVATE_KEY_BASE64: base64.StdEncoding.EncodeToString(privKeyPEM),
		CERTIFICATE_BASE64: base64.StdEncoding.EncodeToString(certPEM),
	}

	// Update task about certificate created
	result, err := UpdateTaskAboutCredentialsCreated(client, config, credentialsPayload)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task: certificate with serial number: '%s' is disposed. error: %s. ", config.SM_CREDENTIALS_ID, err.Error()))
		// The job only stores the certificate in-memory therefore there is no need to perform actual deletion
		os.Exit(1)
	} else {
		logger.Info(fmt.Sprintf("task successfully updated: certificate with serial number: '%s' was created by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))
	}

}

// deleteCredentials deletes the credentials identiifed by the credentials id for the given secret
func deleteCredentials(client SecretsManagerClient, config *Config) {
	// Nothing to delete since credentials are created by the job in memeory only
	result, err := UpdateTaskAboutCredentialsDeleted(client, config)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about certificate deleted with serial number: '%s'. error: %s. ", config.SM_CREDENTIALS_ID, err.Error()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: certificate with serial number: '%s' was deleted by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))

}

// UpdateTaskAboutError updates the task with the given task id with the given error code and description
func updateTaskAboutErrorAndExit(client SecretsManagerClient, config *Config, code, description string) {
	result, err := UpdateTaskAboutError(client, config, code, description)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about error with code: '%s' and description: '%s'. returned error: %w", code, description, err))
	} else {
		logger.Info(fmt.Sprintf("task was updated about error with code: '%s' and description: '%s' by: %s", code, description, *result.UpdatedBy))
	}
	os.Exit(1)
}

// setDefaultValues sets default values for non required config variables if not set by the user
func setDefaultValues(config *Config) {
	if config.SM_EXPIRATION_DAYS == 0 {
		config.SM_EXPIRATION_DAYS = 90
	}
	if config.SM_KEY_ALGO == "" {
		config.SM_KEY_ALGO = KEY_ALGO_RSA
	}
	if config.SM_SIGN_ALGO == "" {
		config.SM_SIGN_ALGO = SIGN_ALGO_SHA256
	}
}

// generateCertificate generates a certificate and private key based on the provided configuration.
func generateCertificate(client SecretsManagerClient, config *Config) ([]byte, []byte) {
	// Generate private key
	var privKey crypto.Signer
	var err error
	switch config.SM_KEY_ALGO {
	case KEY_ALGO_ECDSA:
		privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			updateTaskAboutErrorAndExit(client, config, "Err10002", fmt.Sprintf("cannot generate ECDSA private key: %s", err.Error()))
		}
	default:
		// Using RSA as default key algorithm
		privKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			updateTaskAboutErrorAndExit(client, config, "Err10003", fmt.Sprintf("cannot generate RSA private key: %s", err.Error()))
		}
	}

	// Create certificate serial number
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	// Set the certificate serial number as the credentials id
	config.SM_CREDENTIALS_ID = fmt.Sprintf("%d", serialNumber)

	// Create the certificate
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   config.SM_COMMON_NAME,
			Organization: []string{config.SM_ORG},
			Country:      []string{config.SM_COUNTRY},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Duration(config.SM_EXPIRATION_DAYS) * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	// Add SANs if provided
	if config.SM_SAN != "" {
		cert.DNSNames = strings.Split(config.SM_SAN, ",")
	}

	// Determine signature algorithm based on both key type and hash algorithm
	var signAlgoX509 x509.SignatureAlgorithm
	switch {
	case config.SM_KEY_ALGO == KEY_ALGO_ECDSA && config.SM_SIGN_ALGO == SIGN_ALGO_SHA256:
		signAlgoX509 = x509.ECDSAWithSHA256
	case config.SM_KEY_ALGO == KEY_ALGO_ECDSA && config.SM_SIGN_ALGO == SIGN_ALGO_SHA512:
		signAlgoX509 = x509.ECDSAWithSHA512
	case config.SM_KEY_ALGO == KEY_ALGO_RSA && config.SM_SIGN_ALGO == SIGN_ALGO_SHA256:
		signAlgoX509 = x509.SHA256WithRSA
	case config.SM_KEY_ALGO == KEY_ALGO_RSA && config.SM_SIGN_ALGO == SIGN_ALGO_SHA512:
		signAlgoX509 = x509.SHA512WithRSA
	default:
		// Default to SHA256WithRSA
		signAlgoX509 = x509.SHA256WithRSA
	}

	cert.SignatureAlgorithm = signAlgoX509

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, privKey.Public(), privKey)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, "Err10004", fmt.Sprintf("cannot create certificate with serial number: '%s'. error: %v", serialNumber, err))
	}

	// Convert to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	var privKeyPEM []byte
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		privKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	case *ecdsa.PrivateKey:
		privKeyBytes, _ := x509.MarshalECPrivateKey(k)
		privKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyBytes})
	}
	return privKeyPEM, certPEM
}
