package job

import (
	"encoding/json"
	"errors"
	"fmt"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	resty "github.com/go-resty/resty/v2"
	"jfrog-access-token-provider-go/internal/utils"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	ACCESS_PATH                 = "/access"
	TOKENS_PATH                 = ACCESS_PATH + "/api/v1/tokens/"
	RETRY_COUNT                 = 3
	RETRY_MIN_WAIT_TIME_SECONDS = 5
	RETRY_MAX_WAIT_TIME_SECONDS = 15
)

type CreateAccessTokenRequestBody struct {
	Username              string `json:"username"`
	Scope                 string `json:"scope"`
	ExpiresInSeconds      int    `json:"expires_in"`
	Refreshable           bool   `json:"refreshable"`
	Description           string `json:"description"`
	Audience              string `json:"audience"`
	IncludeReferenceToken bool   `json:"include_reference_token"`
}

type JFrogErrorResponseBody struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

var logger *utils.Logger

func Run() {

	//start
	config, err := ConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}
	config.SM_JFROG_BASE_URL = strings.TrimSuffix(config.SM_JFROG_BASE_URL, "/")

	smClient, err := NewSecretsManagerClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	restyClient := utils.RestyClientStruct{
		Client: resty.New().
			SetRetryCount(RETRY_COUNT).
			SetRetryWaitTime(RETRY_MIN_WAIT_TIME_SECONDS * time.Second).
			SetRetryMaxWaitTime(RETRY_MAX_WAIT_TIME_SECONDS * time.Second).
			AddRetryCondition(
				func(r *resty.Response, err error) bool {
					return err != nil || r.StatusCode() >= http.StatusTooManyRequests
				},
			)}

	logger = utils.NewLogger(config.SM_SECRET_TASK_ID, config.SM_ACTION)

	switch config.SM_ACTION {
	case sm.SecretTask_Type_CreateCredentials:
		generateCredentials(smClient, &restyClient, &config)
	case sm.SecretTask_Type_DeleteCredentials:
		deleteCredentials(smClient, &restyClient, &config)

	default:
		updateTaskAboutErrorAndExit(smClient, &config, Err10000, fmt.Sprintf("unknown action: '%s'", config.SM_ACTION))
	}

}

// generateCredentials generates the credentials for the given secret
func generateCredentials(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) {
	// Set default values for non required config variables if not set by the user
	setDefaultValues(config)

	// Create JFrog Access Token
	accessToken, tokenId, err := createJFrogAccessToken(smClient, restyClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("error generating credentials: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10001, fmt.Sprintf("error: %s", err.Error()))
	}

	// Set the token ID as the credentials ID
	config.SM_CREDENTIALS_ID = tokenId

	// Create credentials payload
	credentialsPayload := CredentialsPayload{
		ACCESS_TOKEN: accessToken,
	}

	// Update task about certificate created
	result, err := UpdateTaskAboutCredentialsCreated(smClient, config, credentialsPayload)
	if err != nil {
		var errBuilder strings.Builder
		errBuilder.WriteString(fmt.Sprintf("cannot update task: %s", err.Error()))
		err = revokeJFrogAccessToken(smClient, restyClient, config)
		if err != nil {
			errBuilder.WriteString(fmt.Sprintf("cannot revoke the JFrog access token with token id: '%s'. error: %s", config.SM_CREDENTIALS_ID, err.Error()))
		} else {
			errBuilder.WriteString(fmt.Sprintf("JFrog access token with token id: '%s' was revoked. ", config.SM_CREDENTIALS_ID))
		}
		logger.Error(errors.New(errBuilder.String()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: JFrog access token with token id: '%s' was created by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))

}

// deleteCredentials deletes the credentials identified by the credentials' id for the given secret
func deleteCredentials(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) {
	err := revokeJFrogAccessToken(smClient, restyClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("error revoking credentials: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10002, fmt.Sprintf("error revoking credentials with credentials id: '%s': %s", config.SM_CREDENTIALS_ID, err.Error()))
	}

	result, err := UpdateTaskAboutCredentialsDeleted(smClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about revoked credentials with credentials id: '%s'. error: %s. ", config.SM_CREDENTIALS_ID, err.Error()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: credentials with credentials id: '%s' was revoked by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))

}

// createJFrogAccessToken creates JFrog Access Token
func createJFrogAccessToken(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) (string, string, error) {
	jfrogLoginSecret, err := fetchJFrogServiceCredentials(smClient, config)
	if err != nil {
		return "", "", err
	}

	createAccessTokenRequestBody := CreateAccessTokenRequestBody{
		Username:              config.SM_USERNAME,
		Scope:                 config.SM_SCOPE,
		ExpiresInSeconds:      config.SM_EXPIRES_IN_SECONDS,
		Refreshable:           config.SM_REFRESHABLE,
		Description:           config.SM_DESCRIPTION,
		Audience:              config.SM_AUDIENCE,
		IncludeReferenceToken: config.SM_INCLUDE_REFERENCE_TOKEN,
	}

	resp, err := restyClient.Post(*jfrogLoginSecret.Payload, createAccessTokenRequestBody, config.SM_JFROG_BASE_URL+TOKENS_PATH)
	if err != nil {
		return "", "", fmt.Errorf("client returned an error: %s", err.Error())
	}
	if resp.IsError() {
		message := extractErrorMessageFromJFrogErrorResponse(resp)
		return "", "", fmt.Errorf("JFrog returned an error: Status: %s. Error: %s", resp.Status(), message)
	}

	var tokenData map[string]interface{}
	err = json.Unmarshal(resp.Body(), &tokenData)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshaling token data: %s", err.Error())
	}
	accessToken := tokenData["access_token"].(string)
	tokenId := tokenData["token_id"].(string)

	logger.Info(fmt.Sprintf("Access Token successfully created. Credentials ID: %s", tokenId))

	return accessToken, tokenId, nil
}

// fetchJFrogServiceCredentials fetches the credentials for JFrog from Secrets Manager
func fetchJFrogServiceCredentials(smClient SecretsManagerClient, config *Config) (*sm.ArbitrarySecret, error) {
	secret, err := GetSecret(smClient, config.SM_LOGIN_SECRET_ID)
	if err != nil {
		if strings.Contains(err.Error(), "Provided API key could not be found") {
			logger.Error(fmt.Errorf("cannot call the secrets manager service: %v", err))
			os.Exit(1)
		}
		return nil, err
	}

	arbitrarySecret, ok := secret.(*sm.ArbitrarySecret)
	if !ok {
		return nil, fmt.Errorf("get secret id: '%s' returned unexpected secret type: %T, expected arbitrary type", config.SM_LOGIN_SECRET_ID, secret)
	}

	return arbitrarySecret, nil
}

// revokeJFrogAccessToken revokes JFrog access token with a given token ID
func revokeJFrogAccessToken(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) error {
	jfrogLoginSecret, err := fetchJFrogServiceCredentials(smClient, config)
	if err != nil {
		return err
	}

	resp, err := restyClient.Delete(*jfrogLoginSecret.Payload, config.SM_JFROG_BASE_URL+TOKENS_PATH+config.SM_CREDENTIALS_ID)

	if err != nil {
		err = fmt.Errorf("Resty client returned an error: %s", err.Error())
		return err
	}
	if resp.IsError() {
		message := extractErrorMessageFromJFrogErrorResponse(resp)
		err = fmt.Errorf("JFrog returned an error: Status: %s. Error: %s", resp.Status(), message)
		return err
	}

	logger.Info(fmt.Sprintf("Token: %s is successfully revoked", config.SM_CREDENTIALS_ID))

	return nil
}

// UpdateTaskAboutError updates the task with the given task id with the given error code and description
func updateTaskAboutErrorAndExit(smClient SecretsManagerClient, config *Config, code, description string) {
	result, err := UpdateTaskAboutError(smClient, config, code, description)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about error with code: '%s' and description: '%s'. returned error: %w", code, description, err))
	} else {
		logger.Info(fmt.Sprintf("updated task about error with code: '%s' and description: '%s'. task updated. by: %s", code, description, *result.UpdatedBy))
	}
	os.Exit(1)
}

// setDefaultValues sets default values for non required config variables if not set by the user
func setDefaultValues(config *Config) {
	if config.SM_SCOPE == "" {
		config.SM_SCOPE = "applied-permissions/user"
	}
	if config.SM_EXPIRES_IN_SECONDS == 0 {
		config.SM_EXPIRES_IN_SECONDS = 7776000 // 90 days
	}
	if config.SM_AUDIENCE == "" {
		config.SM_AUDIENCE = "*@*"
	}
}

// extractErrorMessageFromJFrogErrorResponse extracts the error message from the JFrog error response
func extractErrorMessageFromJFrogErrorResponse(resp *resty.Response) string {
	var responseBody JFrogErrorResponseBody
	err := json.Unmarshal(resp.Body(), &responseBody)
	if err != nil {
		return fmt.Sprintf("error unmarshaling JFrog response body: %s", err.Error())
	}

	if len(responseBody.Errors) > 0 {
		return responseBody.Errors[0].Message
	}
	return "error details were not provided by JFrog"
}
