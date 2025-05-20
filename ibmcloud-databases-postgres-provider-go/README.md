# IBM Cloud Secrets Manager Credentials Provider for IBM Cloud Databases for PostgreSQL

This is a Go **credentials provider** application designed to run as an IBM Cloud Code Engine [job](https://cloud.ibm.com/docs/codeengine?topic=codeengine-job-plan) for IBM Cloud Secrets Manager [custom credentials](https://cloud.ibm.com/docs/secrets-manager?topic=secrets-manager-getting-started) secret type. This **credentials provider** dynamically generates read-only credentials for IBM Cloud Databases for PostgreSQL with schema-level access.

## Synonyms and Terminology

PostgreSQL is the official name of the relational database management system. “Postgres” is a commonly used abbreviation. Both terms refer to the same system and are used interchangeably in this document.

## Overview

When triggered by Secrets Manager, the job performs two main operations:

* **Credentials Creation** - Generates a new PostgreSQL role with read-only access to a specific database schema.
* **Credentials Deletion** - Removes the previously created PostgreSQL role from the database.

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
| `SM_CREDENTIALS_ID`    | Only provided for `delete_credentials` action. The credentials ID assigned at creation                                                                                                                    |
| `SM_SECRET_VERSION_ID` | Only provided for `delete_credentials` action. The Secrets Manager secret version ID that the job run is operating on                                                                                     |

#### Job Custom Parameters

The job custom environment variables are defined in: [job_config.json](./job_config.json)

##### Required Parameters

| Environment Variable | Description |
|---------------------|-------------|
| `SMIN_LOGIN_SECRET_ID` | Service Credentials secret ID containing the login credentials to the PostgreSQL database  |

##### Optional Parameters

| Environment Variable | Description | Default Value |
|---------------------|-------------|---------------|
| `SMIN_SCHEMA_NAME` | PostgreSQL schema to grant read access | `public` |

#### Output Values

The job produces these values that are stored in Secrets Manager:

| Environment Variable | Description |
|---------------------|-------------|
| `SMOUT_USERNAME` | Dynamically generated PostgreSQL role name (required) |
| `SMOUT_PASSWORD` | Securely generated random password (required) |
| `SMOUT_COMPOSED` | A fully composed PostgreSQL connection string with the generated credentials (required) |
| `SMOUT_CERTIFICATE_BASE64` | Base64-encoded TLS certificate for secure connection (required) |

## Security Features

* **Dynamic Credentials**: Credentials are dynamically generated with minimal privileges and are automatically deleted after second rotation.
* **Least Privilege**: Grants read-only access to a specific database schema.
* **Secure Password Generation**:
  * 64-character, randomly generated password.
  * Contains a mix of uppercase, lowercase, numbers, and special characters.
* **Transactional Role Management**: Uses database transactions to ensure atomic role creation and privilege assignment.
* **Secured Connection**: Supports secure TLS connections using certificates generated by IBM Cloud Databases for PostgreSQL.
* **Automatic Rotation**:<br>
  * Uses PostgreSQL login credentials managed as a Service Credentials secret.
  * Uses an IAM API key for Secrets Manager access, managed as an IAM Credentials secret.
  * Both Secrets Manager-referenced secrets are configured with automatic rotation.

## Development

### Project Structure

```
postgres-credentials-provider/
├── cmd/
│   └── main.go                 - Entry point for the application
├── internal/
│   ├── job/
│   │   └── postgres_custom_credentials.go - Implements PostgreSQL credential management
│   │   └── secrets_manager_job.go  - Handles integration with IBM Cloud Secrets Manager
│   └── utils/
│       └── logger.go           - Provides logging functionality
└── job_config.json             - Defines the input and output parameters for the job
```

### Building and Testing

```bash
# Clone the secrets-manager-custom-credentials-providers repository to your local machine
git clone https://github.com/IBM/secrets-manager-custom-credentials-providers

# navigate to the provider directory
cd ibmcloud-databases-postgres-provider-go

# Run test
go test ./internal/job

# Build the job binary
go build -o postgres-credentials-provider ./cmd
```

### How It Works

1. **Initialization**: Reads configuration from environment variables.
2. **Login Credentials Retrieval**: Fetches PostgreSQL login connection details from a Secrets Manager **Service Credentials** secret.
3. **Role Generation**:
   * Creates a new PostgreSQL role with a unique name prefixed with `secrets_manager_`.
   * Generates a secure random password.
4. **Privilege Assignment**:
   * Grants USAGE on the specified schema.
   * Grants SELECT permissions on all existing tables in the schema.
5. **Output**: Sends the newly generated credentials back to Secrets Manager.

## Usage with IBM Cloud Secrets Manager

**For ease of use, this example assumes that all services are deployed within the same IBM Cloud account, region, and resource group.**

### Prerequisites

* GO development environment.
* IBM Cloud CLI with:
  * Code Engine CLI Plugin.
  * Cloud Databases CLI Plugin.
  * Secrets Manager CLI Plugin.
* An IBM Cloud Databases for PostgreSQL instance.
* An IBM Cloud Secrets Manager instance.
* jq command-line JSON processor.

### Step-by-Step Setup

#### 1. Create a Code Engine Project

```bash
# Set variables
REGION=us-south
RESOURCE_GROUP=Default # Note that some regions use 'default' as the name of the default resource group.
CE_PROJECT_NAME=postgres-credentials-provider
CE_JOB_NAME=postgres-credentials-provider-job

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
ibmcloud ce job create --name postgres-credentials-provider-job 
  --build-source . 
  --build-dockerfile Dockerfile 
  --env SMIN_SCHEMA_NAME="type:string, required:false" 
  --env SMIN_LOGIN_SECRET_ID="type:secret_id, required:true" 
  --env SMOUT_CERTIFICATE_BASE64="type:string, required:true" 
  --env SMOUT_COMPOSED="type:string, required:true" 
  --env SMOUT_PASSWORD="type:string, required:true" 
  --env SMOUT_USERNAME="type:string, required:true" 

Execute this command? (y/n): 
```

This command will:

* Upload your local source code to Code Engine
* Use the specified Dockerfile to build an image and store it in IBM Cloud Container Registry
* Create a job definition using the built image and using the environment variables defined in: [job_config.json](./job_config.json)  

#### 3. Create an IAM Service ID for Secrets Manager access

```bash
# Create Service ID
ibmcloud iam service-id-create postgres-credentials-provider-sid --description "Service ID for Secret Manager Postgres Credentials provider"

# Capture the ID of this Service ID
SERVICEID_ID=<serviceid_id>
```

This Service ID will later be assigned with IAM **SecretTaskUpdater** and **SecretsReader** service policies scoped to the Secrets Manager instance Secret Group containing the postgres credentials provider secrets.

#### 4. Configure a Secrets Manager Secret Group

```bash
# Read your Secrets Manager instance configuration
ibmcloud resource service-instance "<your-secrets-manager-instance-name>"

# Capture the Secrets Manager instance ID
SM_INSTANCE_ID=<instance_guid>

# Configure Secrets Manager CLI to use the Secrets Manager instance public endpoint
ibmcloud secrets-manager config set service-url https://$SM_INSTANCE_ID.$REGION.secrets-manager.appdomain.cloud

# Create a Secret Group to contain the postgres provider secrets
ibmcloud secrets-manager secret-group-create \
    --name postgres-credentials-provider-sg \
    --description "Secret Group containing postgres credentials provider secrets"

# Capture the ID of this secret group
SECRET_GROUP_ID=<secret_group_id>
```

#### 5. Configure IAM policies

Read your Databases for PostgreSQL instance configuration

```bash
# Capture the name of your Cloud Databases for PostgeSQL 
PG_INSTANCE_NAME=<your-postgres-instance-name>

# Read instance configuration
ibmcloud resource service-instance $PG_INSTANCE_NAME

# Capture the PostgreSQL instance ID and CRN
PG_INSTANCE_ID=<instance_guid>   
PG_INSTANCE_CRN=<instance_id> 
```

Create an IAM authorization policy assigning the **Key Manager** role to the Secrets Manager instance instance for the Databases for PostgreSQL instance.

```bash
ibmcloud iam authorization-policy-create \
    secrets-manager databases-for-postgresql \
    "Key Manager" \
    --source-service-instance-id $SM_INSTANCE_ID \
    --target-service-instance-id $PG_INSTANCE_ID
```

Create an IAM authorization policy assigning **Viewer** and **Writer** roles to the Secrets Manager instance for the Code Engine project:

```bash
# Create authorization policy
ibmcloud iam authorization-policy-create \
    secrets-manager codeengine \
    Viewer,Writer \
    --source-service-instance-id $SM_INSTANCE_ID \
    --target-service-instance-id $CE_PROJECT_ID
```

Create an IAM service ID policy assigning **SecretTaskUpdater** and **SecretsReader** roles scoped to the Secrets Manager instance Secret Group containing the postgres credentials provider secrets.

```bash
# Create an IAM service ID policy
ibmcloud iam service-policy-create $SERVICEID_ID \
    --roles SecretTaskUpdater,SecretsReader \
    --service-name secrets-manager \
    --service-instance $SM_INSTANCE_ID \
    --resource-type secret-group \
    --resource $SECRET_GROUP_ID
```

#### 6. Configure an Secrets Manager IAM Credentials secret

Create an IAM Credentials secret for managing the IAM Service ID API key that the postgres credentials provider will use to authenticate back with Secrets Manager.

```bash
# Create an IAM Credentials secret
ibmcloud secrets-manager secret-create \
    --secret-type iam_credentials \
    --secret-name postgres-credentials-provider-apikey \
    --secret-description "Secret managing the apikey for the postgres credentials provider" \
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

#### 7. Create a Service Credentials secret

Create a Service Credentials secret managing the login credentials for the postgres credentials provider.

```bash
ibmcloud secrets-manager secret-create \
    --secret-type service_credentials \
    --secret-name postgres-credentials-provider-login \
    --secret-description "Secret managing the login credentials for the postgres credentials provider" \
    --secret-group-id $SECRET_GROUP_ID \
    --secret-ttl 90d \
    --secret-source-service "{
      \"instance\": {
            \"crn\": \"$PG_INSTANCE_CRN\"
      }
    }" \
    --secret-rotation '{
        "auto_rotate": true,
        "interval": 60,
        "unit": "day"
    }'

# Capture the ID of this secret
LOGIN_SECRET_ID=<secret_id>
```

#### 8. Create a Secrets Manager Custom Credentials configuration

Create a Custom Credentials configuration for the PostgreSQL credentials provider.

```bash
# Create a custom credentials configuration
ibmcloud secrets-manager configuration-create \
    --config-type custom_credentials_configuration \
    --name postgres-credentials-provider \
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
# Create a custom credentials secret
ibmcloud secrets-manager secret-create \
  --secret-type custom_credentials \
  --secret-configuration postgres-credentials-provider \
  --secret-name example-pg-credentials \
  --secret-description "Read-only credentials for $PG_INSTANCE_NAME database public schema" \
  --secret-group-id $SECRET_GROUP_ID \
  --secret-ttl 90d \
  --secret-parameters "{
    \"schema_name\": \"public\",
    \"login_secret_id\": \"$LOGIN_SECRET_ID\"
  }" \
  --secret-rotation '{
    "auto_rotate": true,
    "interval": 60,
    "unit": "day"
  }'

# Capture the secret ID
PG_SECRET_ID=<secret_id>
```

#### 10. Test the postgres credentials

Connect to the PostgreSQL database using the credentials from the custom credentials secret.

```bash
# Download the postgres certificate
ibmcloud secrets-manager secret --id $PG_SECRET_ID --output=JSON | \
    jq -r '.credentials_content.certificate_base64' | \
    base64 --decode > pg.crt

# Capture the composed connect string
COMPOSED=$(ibmcloud secrets-manager secret --id $PG_SECRET_ID --output=JSON | \
jq -r '.credentials_content.composed')

# Connect to the database
psql "$COMPOSED&sslrootcert=pg.crt"

# Display current user
SELECT SESSION_USER;
```

### Troubleshooting

If the secret did not become active, check the task status:

```bash
# Retrieve the secret
ibmcloud secrets-manager secret --id=$PG_SECRET_ID

# Capture the task ID
TASK_ID=<processing_task_id>

# Check the Secret Task Status
ibmcloud secrets-manager task --secret-id $PG_SECRET_ID --id $TASK_ID

# View Job Run Logs. Note: To observe job logs, modify the environment variable CE_REMOVE_COMPLETED_JOBS to a different value (e.g., 3d), then create a new secret.:
ibmcloud ce jobrun logs -f -n $TASK_ID
```

## Limitations

* Postgres credentials provider supports read-only access to a single schema.
* Credentials generated by the PostgreSQL credentials provider do not automatically grant access to tables created after their issuance. To obtain credentials with access to the new tables, rotate the Custom Credentials secret.

## License

This provider is open-source using Apache License 2.0.
