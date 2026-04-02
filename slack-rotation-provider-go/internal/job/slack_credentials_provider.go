package job

import (
	"encoding/json"
	"errors"
	"fmt"
	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	resty "github.com/go-resty/resty/v2"
	"log"
	"net/http"
	"os"
	"slack-rotation-provider-go/internal/utils"
	"strings"
	"time"
)

const (
	RETRY_COUNT                 = 3
	RETRY_MIN_WAIT_TIME_SECONDS = 5
	RETRY_MAX_WAIT_TIME_SECONDS = 15
)

type SlackErrorResponseBody struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

type SlackRenewTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Ok           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
}

type SlackAuthResponse struct {
	OK                  bool        `json:"ok"`
	AppID               string      `json:"app_id"`
	AuthedUser          AuthedUser  `json:"authed_user"`
	Team                Team        `json:"team"`
	Enterprise          interface{} `json:"enterprise"`
	IsEnterpriseInstall bool        `json:"is_enterprise_install"`
	Error               string      `json:"error,omitempty"`
}

type AuthedUser struct {
	ID           string `json:"id"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SlackRequest struct {
}
type SlackExchangeTokenPayload struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

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

func triggerSecretRotation(smClient SecretsManagerClient) {

}

func getRefreshToken(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) (*SlackExchangeTokenPayload, error) {
	if currentSecret, error := GetSecret(smClient, config.SM_EXCHANGE_TOKENS_SECRET_ID); error == nil {
		as := currentSecret.(*sm.ArbitrarySecret)
		payload := SlackExchangeTokenPayload{}
		err := json.Unmarshal([]byte(*as.Payload), &payload)
		if err != nil {
			fmt.Println("Error unmarshalling JSON:", err)
			return nil, err
		}
		return &payload, nil
	} else {
		return nil, error
	}
}
func getRefreshTokenFromPreviousVersion(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) (string, error) {
	currentSecret, err := GetSecret(smClient, config.SM_SECRET_ID)
	if err != nil || currentSecret == nil {
		return "", err
	}

	as, ok := currentSecret.(*sm.CustomCredentialsSecret)
	if !ok {
		return "", errors.New("unexpected secret type")
	}

	if as.VersionsTotal == nil || *as.VersionsTotal <= 0 {
		return "", nil
	}

	if token, ok := as.CredentialsContent["slack_refresh_token"].(string); ok {
		return token, nil
	}

	return "", nil
}

// generateCredentials generates the credentials for the given secret
func generateCredentials(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) {
	// Create Slack Access Token
	slackAccessToken, refreshToken, err := createSlackAccessToken(smClient, restyClient, config)
	if err != nil {
		logger.Error(fmt.Errorf("error generating credentials: %s", err.Error()))
		updateTaskAboutErrorAndExit(smClient, config, Err10001, fmt.Sprintf("error: %s", err.Error()))
	}

	config.SM_CREDENTIALS_ID = "na"

	// Create credentials payload
	credentialsPayload := CredentialsPayload{
		SLACK_ACCESS_TOKEN:  slackAccessToken,
		SLACK_REFRESH_TOKEN: refreshToken,
	}

	// Update task about certificate created
	result, err := UpdateTaskAboutCredentialsCreated(smClient, config, credentialsPayload)
	if err != nil {
		var errBuilder strings.Builder
		errBuilder.WriteString(fmt.Sprintf("cannot update task: %s", err.Error()))
		logger.Error(errors.New(errBuilder.String()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: slack token with token id: '%s' was created by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))

}

// deleteCredentials deletes the credentials identified by the credentials' id for the given secret
func deleteCredentials(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) {
	UpdateTaskAboutCredentialsDeleted(smClient, config)
}

// createSlackAccessToken creates Slack Access Token
func createSlackAccessToken(smClient SecretsManagerClient, restyClient utils.RestyClientIntf, config *Config) (string, string, error) {

	//First get the refresh token from the slack exchange credentials.
	setp, error := getRefreshToken(smClient, restyClient, config)
	if error != nil {
		return "", "", error
	}

	//get the refresh token from the previous version of the custom credentials.
	lastRefreshToken, error := getRefreshTokenFromPreviousVersion(smClient, restyClient, config)

	//If we didn't find a refresh token in previous version then we try with the slack exchange tokens refresh token.
	if lastRefreshToken == "" {
		if error != nil {
			logger.Error(error)
		}
		logger.Info("Last refresh token not found. fallback to slack exchange token refresh token.")
		lastRefreshToken = setp.RefreshToken
	}

	accessToken, refreshToken, error := exchangeRefreshToken(setp.ClientId, setp.ClientSecret, lastRefreshToken, restyClient)

	if error != nil {
		if lastRefreshToken != setp.RefreshToken {
			//try again with the slack exchange refresh token
			logger.Info("Trying again with slack exchange refresh token after error:" + error.Error())
			accessToken, refreshToken, error = exchangeRefreshToken(setp.ClientId, setp.ClientSecret, setp.RefreshToken, restyClient)
			//No more tries. return failure.
			if error != nil {
				return "", "", error
			}
		}
	}
	return accessToken, refreshToken, error
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

func exchangeRefreshToken(clientID, clientSecret, refreshToken string, restyClient utils.RestyClientIntf) (string, string, error) {
	endpoint := "https://slack.com/api/oauth.v2.access"

	var slackRes SlackRenewTokenResponse
	resp, err := restyClient.PostWithFormData(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}, &slackRes, endpoint)

	if err != nil {
		log.Fatal("Request error:", err)
	}

	if resp.StatusCode() != 200 {
		log.Fatal("Request status error:", resp.StatusCode())
	}

	result := resp.Request.Result.(*SlackRenewTokenResponse)

	if !result.Ok {
		return "", "", fmt.Errorf("Slack error: %s", result.Error)
	}

	return result.AccessToken, result.RefreshToken, nil
}

func main() {
	Run()
}
