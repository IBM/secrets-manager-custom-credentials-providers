# IBM Cloud Secrets Manager Credentials Provider for Slack token rotation.

## Overview

When triggered by Secrets Manager, the job performs two main operations:

* **Credentials Creation** - Generate a new Slack Access Token.
* **Credentials Deletion** - Nothing to do

## Configuration

### Environment Variables

The job uses service and custom environment variables for configuration, which are automatically provided by Secrets Manager when the job is triggered.

#### Service Parameters

The service environment variables that are passed by Secrets Manager to the job:

| Environment Variable   | Description                                                                                                               |
|------------------------|---------------------------------------------------------------------------------------------------------------------------|
| `SM_ACTION`            | Specifies whether to create or delete credentials (`create_credentials` or `delete_credentials`)                          |
| `SM_INSTANCE_URL`      | The URL of the Secrets Manager instance                                                                                   |
| `SM_SECRET_TASK_ID`    | The ID of the task that the job run is currently operating on                                                             |
| `SM_SECRET_ID`         | The ID of the secret being processed                                                                                      |
| `SM_SECRET_NAME`       | The name of the secret being processed                                                                                    |
| `SM_SECRET_GROUP_ID`   | The ID of the Secrets Manager secret group that contains the secret                                                       |
| `SM_CREDENTIALS_ID`    | Provided only for the `delete_credentials` action. The credentials ID assigned at creation.                               |
| `SM_SECRET_VERSION_ID` | Provided only for the `delete_credentials` action. The Secrets Manager secret version ID that the job run is operating on |

#### Job Custom Parameters

The job custom environment variables are defined in: [job_config.json](./job_config.json)

##### Required Parameters

| Environment Variable   | Description                                                                                                                                                                                                                                 |
|------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `SMIN_EXCHANGE_TOKENS_SECRET_ID` | Arbitrary secret ID containing the credentials for creating the slack access token: <br/>```{"client_id": "<the slack app client id>", "client_secret": "<the slack app client secret>", "refresh_token": "<the initial refresh token>"}``` |


#### Output Values

The job produces these values that are stored in Secrets Manager:

| Environment Variable | Description                               |
|----------------------|-------------------------------------------|
| `SMOUT_SLACK_ACCESS_TOKEN` | The new slack access token.               |
| `SMOUT_SLACK_REFRESH_TOKEN` | The new slack refresh token. |

## Development

### Project Structure

```
slack-rotation-provider/
├── cmd/
│   └── main.go                 - Entry point for the application
├── internal/
│   ├── job/
│   │   └── slack_credentials_provider.go - Implements Slack Access token management
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
cd slack-rotation-provider

# Run test
go test ./internal/job

# Build the job binary
go build -o slack-rotation-provider ./cmd
```

### How It Works

1. **Initialization**: Reads configuration from environment variables.
2. **Login Credentials Retrieval**: Retrieves Slack credentials from a Secrets Manager Arbitrary secret.
3. **Credentials Generation**: Creates a new Slack access token.
4. **Output**: Provides new credentials back to Secrets Manager.

## Usage with IBM Cloud Secrets Manager

### Step-by-Step Setup

#### 1. Create a Code Engine Project

```bash
# Set variables
REGION=<region>
RESOURCE_GROUP=<resource-group> # Note that some regions use 'default' as the name of the default resource group.
CE_PROJECT_NAME=slack-rotation-provider
CE_JOB_NAME=slack-rotation-provider-job

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

Use the [**job-deployer**](https://github.com/IBM/secrets-manager-custom-credentials-providers/blob/main/tools/README.md#job-deployer) tool to create and deploy the job

```bash
# Make the script executable by running:
chmod +x ../tools/job-deployer.sh

# Create a job from local source code
../tools/job-deployer.sh --jobdir . --name $CE_JOB_NAME --action create

# Review the command and execute it. This command can take few minutes to complete
Execute this command? (y/n): 
```

This command will:

* Upload your local source code to Code Engine
* Use the specified Dockerfile to build an image and store it in IBM Cloud Container Registry
* Create a job definition using the built image and using the environment variables defined in: [job_config.json](./job_config.json)

#### 3. Create an IAM Service ID for Secrets Manager access

```bash
# Create Service ID
ibmcloud iam service-id-create slack-token-rotation-provider-sid --description "Service ID for Secret Manager Slack Access Token provider"

# Capture the ID of this Service ID
SERVICEID_ID=<serviceid_id>
```

This Service ID will later be assigned with IAM **SecretTaskUpdater** and **SecretsReader** service policies scoped to the Secrets Manager instance Secret Group containing the Slack Access Token provider secrets.

#### 4. Configure a Secrets Manager Secret Group

```bash
# Read your Secrets Manager instance configuration
ibmcloud resource service-instance "<your-secrets-manager-instance-name>"

# Capture the Secrets Manager instance ID
SM_INSTANCE_ID=<instance_guid>

# Configure Secrets Manager CLI to use the Secrets Manager instance public endpoint
ibmcloud secrets-manager config set service-url https://$SM_INSTANCE_ID.$REGION.secrets-manager.appdomain.cloud

# Create a Secret Group to contain the Slack access token provider secrets
ibmcloud secrets-manager secret-group-create \
    --name slack-token-rotation-provider-go-sg \
    --description "Secret Group containing Slack Access Token provider secrets"

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

Create an IAM service ID policy assigning **SecretTaskUpdater** and **SecretsReader** roles scoped to the Secrets Manager instance Secret Group containing the Slack Access Token provider secrets.

```bash
# Create an IAM service ID policy
ibmcloud iam service-policy-create $SERVICEID_ID \
    --roles SecretTaskUpdater,SecretsReader \
    --service-name secrets-manager \
    --service-instance $SM_INSTANCE_ID \
    --resource-type secret-group \
    --resource $SECRET_GROUP_ID
```

#### 6. Obtain initial slack refresh token

1. Enable Advanced token security via token rotation
2. Add redirect URL (For now we can use https://cloud.ibm.com) as it's a manual one-time process.
3. Manage distribution -> Click the add to slack button
4. Copy the code from the url
5. Run the script in the slack-rotation-provider repository : sh exchange_authentication_code.sh <CLIENT_ID>  <CLIENT_SECRET> <REDIRECT_URI> <CODE>
#### 7. Create an Arbitrary secret

Create an Arbitrary Credentials secret managing the login credentials for the Slack Platform.
```
{"client_id": "<the slack app client id>", "client_secret": "<the slack app client secret>", "refresh_token": "<the refresh token>"}
```

#### 8. Create a Secrets Manager Custom Credentials configuration

Create a Custom Credentials configuration for the Slack Access Token provider.

```bash
# Create a custom credentials configuration
ibmcloud secrets-manager configuration-create --config-type=custom_credentials_configuration --name=slack_custom_credentials_configration --custom-credentials-apikey-ref <the_iam_credential_secret_id(from part 1)> --custom-credentials-code-engine '{"project_id":"<code_engine_project_id>", "region":"<region>", "job_name":"slack-rotation-provider-job"}'
```

#### 9. Create a Secrets Manager Custom Credentials secret
1. Create a Secrets Manager Custom Credentials secret using `slack_custom_credentials_configration`
2. Enable autorotation for 8h


