# IBM Cloud Secrets Manager Credentials Provider for JFrog Platform

This is an example Go application designed to run as an IBM Cloud Code Engine [job](https://cloud.ibm.com/docs/codeengine?topic=codeengine-job-plan) for IBM Cloud Secrets Manager [custom credentials](https://cloud.ibm.com/docs/secrets-manager?topic=secrets-manager-getting-started) secret type. This job dynamically generates credentials for the JFrog platform.

## Overview

When triggered by Secrets Manager, the job performs two main operations:

* **Credentials Creation** - Generates a new JFrog Platform Access Token.
* **Credentials Deletion** - Revokes the previously created JFrog Platform Access Token.

## Configuration

### Environment Variables

The job uses service and custom environment variables for configuration, which are automatically provided by Secrets Manager when the job is triggered.

#### Service Parameters

The service environment variables that are passed by Secrets Manager to the job:

| Environment Variable   | Description                                                                                                                                                                                               |
|------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `SM_ACTION`            | Specifies whether to create or delete credentials (`create_credentials` or `delete_credentials`)                                                                                                          |
| `SM_TRIGGER`           | Specifies the actions that triggered this task. Allowed values are: `secret_creation`, `manual_secret_rotation`, `automatic_secret_rotation`, `secret_version_expiration`, `secret_version_data_deletion` |
| `SM_INSTANCE_URL`      | The URL of the Secrets Manager instance                                                                                                                                                                   |
| `SM_SECRET_TASK_ID`    | The ID of the task that the job run is currently operating on                                                                                                                                             |
| `SM_SECRET_ID`         | The ID of the secret being processed                                                                                                                                                                      |
| `SM_SECRET_NAME`       | The name of the secret being processed                                                                                                                                                                    |
| `SM_SECRET_GROUP_ID`   | The ID of the Secrets Manager secret group that contains the secret                                                                                                                                       |
| `SM_CREDENTIALS_ID`    | Provided only for the `delete_credentials` action. The credentials ID assigned at creation.                                                                                                               |
| `SM_SECRET_VERSION_ID` | Provided only for the `delete_credentials` action. The Secrets Manager secret version ID that the job run is operating on                                                                                 |

#### Job Custom Parameters

The job custom environment variables are defined in: [job_config.json](./job_config.json)

##### Required Parameters

| Environment Variable   | Description                                                                           |
|------------------------|---------------------------------------------------------------------------------------|
| `SMIN_LOGIN_SECRET_ID` | Arbitrary secret ID containing the login credentials to the JFrog platform            |
| `SMIN_JFROG_BASE_URL`  | Your JFrog platform base URL. For example: https://<JFROG_PLATFORM_URL>:<ROUTER_PORT> |

##### Optional Parameters

| Environment Variable           | Description                                                                                                                                                                                                | Default Value                       |
|--------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------|
| `SMIN_USERNAME`                | The user name for which this token is created. Administrators can assign a token to any subject (user); non-admin users who create tokens can only assign tokens to themselves. Limited to 255 characters. | `subject` from authentication token |
| `SMIN_SCOPE`                   | The scope of access that the token provides. For more information and configuration options, see [Create a JFrog Scoped Token](https://jfrog.com/help/r/HmVki7GUNPbjnGgpFbjGmw/l2UYqIBsb1aZ3I4nhXIYgA).    | `applied-permissions/user`          |
| `SMIN_EXPIRES_IN_SECONDS`      | The amount of time, in seconds, it would take for the token to expire. Must be non-negative.                                                                                                               | `7776000 (90 days)`                 |
| `SMIN_REFRESHABLE`             | The token is not refreshable by default.                                                                                                                                                                   | `false`                             |
| `SMIN_DESCRIPTION`             | Free text token description. Useful for filtering and managing tokens. Limited to 1024 characters.                                                                                                         | `""`                                |
| `SMIN_AUDIENCE`                | A space-separated list of the other instances or services that should accept this token identified by their Service-IDs. Limited to 255 characters.                                                        | `*@*`                               |
| `SMIN_INCLUDE_REFERENCE_TOKEN` | Generate a Reference Token (alias to Access Token) in addition to the full token (available from Artifactory 7.38.10).                                                                                     | `false`                             |

#### Output Values

The job produces these values that are stored in Secrets Manager:

| Environment Variable | Description                            |
|----------------------|----------------------------------------|
| `SMOUT_ACCESS_TOKEN` | Generated JFrog Platform Access Token. |

## Security Features

Uses JFrog platform login credentials managed as an Arbitrary secret.

## Development

### Project Structure

```
jfrog-access-token-provider-go/
├── cmd/
│   └── main.go                 - Entry point for the application
├── internal/
│   ├── job/
│   │   └── credentials_provider.go - Implements JFrog Access token management
│   │   └── secrets_manager_job.go  - Handles integration with IBM Cloud Secrets Manager
│   └── utils/
│       └── logger.go           - Provides logging functionality
        └── resty_client.go     - Provides http client functionality
└── job_config.json             - Defines the input and output parameters for the job
```

### Building and Testing

```bash
# Clone the secrets-manager-custom-credentials-providers repository to your local machine
git clone https://github.com/IBM/secrets-manager-custom-credentials-providers

# navigate to the provider directory
cd jfrog-access-token-provider-go

# Run test
go test ./internal/job

# Build the job binary
go build -o jfrog-access-token-provider-go ./cmd
```

### How It Works

1. **Initialization**: Reads configuration from environment variables.
2. **Login Credentials Retrieval**: Retrieves JFrog platform's login credentials from a Secrets Manager Arbitrary secret.
3. **Credentials Generation**: Creates a new JFrog access token.
4. **Output**: Provides new credentials back to Secrets Manager.

## Usage with IBM Cloud Secrets Manager

**For ease of use, this example assumes that all services are deployed within the same IBM Cloud account, region, and resource group.**

### Prerequisites

* GO development environment
* IBM Cloud CLI with:
    * Code Engine CLI Plugin
    * Secrets Manager CLI Plugin
* A JFrog user account with:
    * Admin privileges if the token will be generated for other users.
* Valid JFrog platform credentials - a Bearer token.
* An IBM Cloud Secrets Manager instance

### Step-by-Step Setup

#### 1. Create a Code Engine Project

```bash
# Set variables
REGION=us-south
RESOURCE_GROUP=Default # Note that some regions use 'default' as the name of the default resource group.
CE_PROJECT_NAME=jfrog-access-token-provider-go
CE_JOB_NAME=jfrog-access-token-provider-go-job

# Login to your IBM Cloud account
ibmcloud login [--sso]

# Target your region and resource group
ibmcloud target -r $REGION -g $RESOURCE_GROUP

# Create a Code Engine project
ibmcloud ce project create --name $CE_PROJECT_NAME

# Get the Code Engine project
ibmcloud ce project get --name $CE_PROJECT_NAME

# Capture the project ID and CRN
CE_PROJECT_ID=<project_id>
CE_PROJECT_CRN=<project_crn>

# Select the project
ibmcloud ce project select --name $CE_PROJECT_NAME
```

#### 2. Create the Job in Code Engine from Local Source Code

Use the [**job-deployer**](../tools/README.md#using-the-job-deployer-tool) tool to create and deploy the job

```bash
# Make the script executable by running:
chmod +x ../tools/job-deployer.sh

# Create a job from local source code
../tools/job-deployer.sh --jobdir . --name $CE_JOB_NAME --action create

# Review the command and execute it. This command can take few minutes to complete
ibmcloud ce job create --name jfrog-access-token-job 
  --build-source .
  --build-dockerfile Dockerfile 
  --env SMIN_USERNAME="type:string, required:false" 
  --env SMIN_SCOPE="type:string, required:false" 
  --env SMIN_EXPIRES_IN_SECONDS="type:integer, required:false" 
  --env SMIN_REFRESHABLE="type:boolean, required:false" 
  --env SMIN_DESCRIPTION="type:string, required:false" 
  --env SMIN_AUDIENCE="type:string, required:false" 
  --env SMIN_INCLUDE_REFERENCE_TOKEN="type:boolean, required:false" 
  --env SMIN_LOGIN_SECRET_ID="type:secret_id, required:true" 
  --env SMIN_JFROG_BASE_URL="type:string, required:true" 
  --env SMOUT_ACCESS_TOKEN="type:string, required:true" 

Execute this command? (y/n): 
```

This command will:

* Upload your local source code to Code Engine
* Use the specified Dockerfile to build an image and store it in IBM Cloud Container Registry
* Create a job definition using the built image and using the environment variables defined in: [job_config.json](./job_config.json)

#### 3. Create an IAM Service ID for Secrets Manager access

```bash
# Create Service ID
ibmcloud iam service-id-create jfrog-access-token-provider-go-sid --description "Service ID for Secret Manager JFrog Access Token provider"

# Capture the ID of this Service ID
SERVICEID_ID=<serviceid_id>
```

This Service ID will later be assigned with IAM **SecretTaskUpdater** and **SecretsReader** service policies scoped to the Secrets Manager instance Secret Group containing the JFrog Access Token provider secrets.

#### 4. Configure a Secrets Manager Secret Group

```bash
# Read your Secrets Manager instance configuration
ibmcloud resource service-instance "<your-secrets-manager-instance-name>"

# Capture the Secrets Manager instance ID
SM_INSTANCE_ID=<instance_guid>

# Configure Secrets Manager CLI to use the Secrets Manager instance public endpoint
ibmcloud secrets-manager config set service-url https://$SM_INSTANCE_ID.$REGION.secrets-manager.appdomain.cloud

# Create a Secret Group to contain the JFrog access token provider secrets
ibmcloud secrets-manager secret-group-create \
    --name jfrog-access-token-provider-go-sg \
    --description "Secret Group containing JFrog Access Token provider secrets"

# Capture the ID of this secret group
SECRET_GROUP_ID=<secret_group_id>
```

#### 5. Configure IAM policies

Create an IAM authorization policy assigning **Viewer** and **Writer** roles to the Secrets Manager instance for the Code Engine project:

```bash
# Create authorization policy
ibmcloud iam authorization-policy-create \
    secrets-manager codeengine \
    Viewer,Writer \
    --source-service-instance-id $SM_INSTANCE_ID \
    --target-service-instance-id $CE_PROJECT_ID
```

Create an IAM service ID policy assigning **SecretTaskUpdater** and **SecretsReader** roles scoped to the Secrets Manager instance Secret Group containing the JFrog Access Token provider secrets.

```bash
# Create an IAM service ID policy
ibmcloud iam service-policy-create $SERVICEID_ID \
    --roles SecretTaskUpdater,SecretsReader \
    --service-name secrets-manager \
    --service-instance $SM_INSTANCE_ID \
    --resource-type secret-group \
    --resource $SECRET_GROUP_ID
```

#### 6. Configure a Secrets Manager IAM Credentials secret

Create an IAM Credentials secret for managing the IAM Service ID API key that the JFrog Access Token provider will use to authenticate back with Secrets Manager.

```bash
# Create an IAM Credentials secret
ibmcloud secrets-manager secret-create \
    --secret-type iam_credentials \
    --secret-name jfrog-access-token-provider-go-apikey \
    --secret-description "Secret managing the apikey for the JFrog Access Token provider" \
    --secret-group-id $SECRET_GROUP_ID \
    --secret-ttl 90d \
    --iam-credentials-service-id $SERVICEID_ID \
    --iam-credentials-reuse-apikey true \
    --secret-rotation '{
        "auto_rotate": true,
        "interval": 60,
        "unit": "day"
    }'

# Capture the ID of this secret
IAM_CREDENTIALS_SECRET_ID=<iam_credentials_secret_id>
```

#### 7. Create an Arbitrary secret

Create an Arbitrary Credentials secret managing the login credentials for the JFrog Platform.

```bash
# Set variables
JFROG_LOGIN_CREDENTIALS=<your-JFrog-platform-login-credentials> # A Bearer token.

ibmcloud secrets-manager secret-create \
    --secret-type arbitrary \
    --secret-name jfrog-access-token-provider-go-login \
    --secret-description "Secret managing the login credentials for the JFrog Access Token provider" \
    --secret-group-id $SECRET_GROUP_ID \
    --arbitrary-payload $JFROG_LOGIN_CREDENTIALS

# Capture the ID of this secret
LOGIN_SECRET_ID=<secret_id>
```

#### 8. Create a Secrets Manager Custom Credentials configuration

Create a Custom Credentials configuration for the JFrog Access Token provider.

```bash
# Create a custom credentials configuration
ibmcloud secrets-manager configuration-create \
    --config-type custom_credentials_configuration \
    --name jfrog-access-token-provider-go \
    --custom-credentials-apikey-ref "$IAM_CREDENTIALS_SECRET_ID" \
    --configuration-task-timeout 10m \
    --custom-credentials-code-engine "{
        \"region\": \"$REGION\",
        \"project_id\": \"$CE_PROJECT_ID\",
        \"job_name\": \"$CE_JOB_NAME\"
    }"
```

#### 9. Create a Secrets Manager Custom Credentials secret

```bash
# Set variables
JFROG_PLATFORM_BASE_URL=<your-JFrog-platform-base-URL>

# Create a custom credentials secret
ibmcloud secrets-manager secret-create \
  --secret-type custom_credentials \
  --secret-configuration jfrog-access-token-provider-go \
  --secret-name example-jfrog-access-token \
  --secret-description "JFrog Access Token" \
  --secret-group-id $SECRET_GROUP_ID \
  --secret-ttl 90d \
  --secret-parameters "{
    \"jfrog_base_url\": \"$JFROG_PLATFORM_BASE_URL\",
    \"login_secret_id\": \"$LOGIN_SECRET_ID\"
  }" \
  --secret-rotation '{
    "auto_rotate": true,
    "interval": 60,
    "unit": "day"
  }'

# Capture the secret ID
JFROG_ACCESS_TOKEN_SECRET_ID=<secret_id>
```

#### 10. Test the JFrog Access Token

Retrieve the Token from Secrets Manager and then call JFrog's GET Access Tokens API to test its validity:

```bash
# Retrieve the secret
ibmcloud secrets-manager secret --id=$JFROG_ACCESS_TOKEN_SECRET_ID

# Capture the JFrog Access token
JFROG_ACCESS_TOKEN=<credentials_content/access_token>

# Call JFrog GET Access Tokens request
curl  -H "Authorization: Bearer $JFROG_ACCESS_TOKEN" "$JFROG_PLATFORM_BASE_URL/access/api/v1/tokens"

{
  "tokens": <list_of_accessible_tokens>
}
```

### Troubleshooting

If the secret did not become active, check the task status:

```bash
# Retrieve the secret
ibmcloud secrets-manager secret --id=$JFROG_ACCESS_TOKEN_SECRET_ID

# Capture the task ID
TASK_ID=<processing_task_id>

# Check the Secret Task Status
ibmcloud secrets-manager task --secret-id $JFROG_ACCESS_TOKEN_SECRET_ID --id $TASK_ID

# View Job Run Logs. Note: To observe job logs, modify the environment variable CE_REMOVE_COMPLETED_JOBS to a different value (e.g., 3d), then create a new secret.:
ibmcloud ce jobrun logs -f -n $TASK_ID
```

## Limitations

A Bearer token is the only supported authentication method for the JFrog platform. 

## License

This provider is open-source using Apache License 2.0.

