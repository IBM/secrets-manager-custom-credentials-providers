package job

import (
	"errors"
	"fmt"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"ibmcloud-iam-user-apikey-provider-go/identity_services_wrapper"
	"ibmcloud-iam-user-apikey-provider-go/utils"
	"log"
	"os"
	"strings"
)

var logger *utils.Logger

func Run() {
	//start
	config, err := ConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}
	smClient, err := NewSecretsManagerClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	logger = utils.NewLogger(config.SM_SECRET_TASK_ID, config.SM_ACTION)

	switch config.SM_ACTION {
	case sm.SecretTask_Type_CreateCredentials:
		generateCredentials(smClient, &config)
	case sm.SecretTask_Type_DeleteCredentials:
		deleteCredentials(smClient, &config)
	default:
		updateTaskAboutErrorAndExit(smClient, &config, Err10000, fmt.Sprintf("unknown action: '%s'", config.SM_ACTION))
	}
}

func generateCredentials(smClient SecretsManagerClient, config *Config) {
	identityServices := initIdentityServices(smClient, config)
	apikey, err := identityServices.CreateApiKey(createOptionsFromConfig(config))

	if err != nil {
		logger.Error(fmt.Errorf("error creating API key: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10003, fmt.Sprintf("error: %s", err.Error()))
	}
	logger.Info(fmt.Sprintf("API key with ID '%s' was created", apikey.ID))

	config.SM_CREDENTIALS_ID = apikey.ID
	credentialsPayload := CredentialsPayload{
		APIKEY:     apikey.ApiKey,
		ID:         apikey.ID,
		CRN:        apikey.CRN,
		IAM_ID:     apikey.IamID,
		ACCOUNT_ID: apikey.AccountID,
	}

	// Update task about certificate created
	result, err := UpdateTaskAboutCredentialsCreated(smClient, config, credentialsPayload)
	if err != nil {
		rollbackAndExit(identityServices, config, apikey.ID, err.Error())
	}
	logger.Info(fmt.Sprintf("task successfully updated: IAM API key with id: '%s' was created by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))
}

func createOptionsFromConfig(config *Config) *identity_services_wrapper.CreateOptions {
	return &identity_services_wrapper.CreateOptions{
		Name:             generateApiKeyName(config),
		Description:      generateApiKeyDescription(config),
		IamID:            config.SM_IAM_ID,
		AccountID:        config.SM_ACCOUNT_ID,
		SupportSessions:  config.SM_SUPPORT_SESSIONS,
		ActionWhenLeaked: config.SM_ACTION_WHEN_LEAKED,
	}
}

func deleteCredentials(smClient SecretsManagerClient, config *Config) {
	identityServices := initIdentityServices(smClient, config)

	err := identityServices.DeleteApiKey(config.SM_CREDENTIALS_ID)
	if err != nil {
		logger.Error(fmt.Errorf("error deleting API key: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10004, fmt.Sprintf("error deleting API key with id: '%s': %s", config.SM_CREDENTIALS_ID, err.Error()))
	}

	result, err := UpdateTaskAboutCredentialsDeleted(smClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about deleted API key with id: '%s'. error: %s. ", config.SM_CREDENTIALS_ID, err.Error()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: API key with id: '%s' was deleted by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))
}

func initIdentityServices(smClient SecretsManagerClient, config *Config) identity_services_wrapper.Wrapper {
	apikey, err := fetchApiKey(smClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("error fetching API key secret reference: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10001, fmt.Sprintf("error: %s", err.Error()))
	}
	identityServices, err := identity_services_wrapper.New(config.SM_URL, apikey)
	if err != nil {
		logger.Error(fmt.Errorf("error initializing IAM Identity Services client: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10002, fmt.Sprintf("error: %s", err.Error()))
	}
	return identityServices
}

func fetchApiKey(smClient SecretsManagerClient, config *Config) (string, error) {
	logger.Info(fmt.Sprintf("Obtaining a secret with ID: %s", config.SM_APIKEY_SECRET_ID))
	secret, err := GetSecret(smClient, config.SM_APIKEY_SECRET_ID)
	if err != nil {
		return "", err
	}
	arbitrarySecret, ok := secret.(*sm.ArbitrarySecret)
	if !ok {
		return "", fmt.Errorf("get secret id: '%s' returned unexpected secret type: %T, expected arbitrary type", config.SM_APIKEY_SECRET_ID, secret)
	}
	logger.Info(fmt.Sprintf("Secret with ID: %s succesfully obtained.", config.SM_APIKEY_SECRET_ID))
	return *arbitrarySecret.Payload, nil
}

func rollbackAndExit(identityServices identity_services_wrapper.Wrapper, config *Config, apikeyID string, reason string) {
	var errBuilder strings.Builder
	errBuilder.WriteString(fmt.Sprintf("cannot update task: %s ", reason))
	err := identityServices.DeleteApiKey(apikeyID)
	if err != nil {
		errBuilder.WriteString(fmt.Sprintf("cannot revoke the IAM API key with id: '%s'. error: %s", config.SM_CREDENTIALS_ID, err.Error()))
	} else {
		errBuilder.WriteString(fmt.Sprintf("IAM API key with id: '%s' was deleted. ", config.SM_CREDENTIALS_ID))
	}
	logger.Error(errors.New(errBuilder.String()))
	os.Exit(1)
}

func generateApiKeyName(config *Config) string {
	return fmt.Sprintf("%s-%s", config.SM_SECRET_NAME, config.SM_SECRET_TASK_ID[len(config.SM_SECRET_TASK_ID)-6:])
}

func generateApiKeyDescription(config *Config) string {
	return fmt.Sprintf("Created by Secrets Manager IAM user API Key provider for secret %s (%s) by %s", config.SM_SECRET_NAME, config.SM_SECRET_ID, config.SM_SECRET_TASK_ID)
}

func updateTaskAboutErrorAndExit(smClient SecretsManagerClient, config *Config, code, description string) {
	result, err := UpdateTaskAboutError(smClient, config, code, description)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about error with code: '%s' and description: '%s'. returned error: %w", code, description, err))
	} else {
		logger.Info(fmt.Sprintf("updated task about error with code: '%s' and description: '%s'. task updated. by: %s", code, description, *result.UpdatedBy))
	}
	os.Exit(1)
}
