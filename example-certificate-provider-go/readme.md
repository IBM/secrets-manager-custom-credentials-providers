# IBM Cloud Secrets Manager Certificate Provider

This is an example Go application designed to run as an IBM Cloud Code Engine [job](https://cloud.ibm.com/docs/codeengine?topic=codeengine-job-plan) for IBM Cloud Secrets Manager [custom credentials](https://cloud.ibm.com/docs/secrets-manager?topic=secrets-manager-getting-started) secret type. This job generates self-signed SSL/TLS certificates for development and testing.

## Overview

When triggered by Secrets Manager, the job performs two main operations:

* **Certificate Creation** - Generates new self-signed certificates with customizable parameters in memory and stores them in Secrets Manager
* **Certificate Deletion** - Reports successful deletion to Secrets Manager without performing any actual deletion, as certificates are generated in-memory only and not persisted by the job itself. Secrets Manager handles secret lifecycle management outside the job.

## Configuration

### Environment Variables

The job uses service and custom environment variables for configuration, which are automatically provided by Secrets Manager when the job is triggered.

#### Service Parameters

The service environment variables that are passed by Secrets Manager to the job:

| Environment Variable | Description |
|---------------------|-------------|
| `SM_ACTION` | Specifies whether to create or delete credentials (`create_credentials` or `delete_credentials`) |
| `SM_INSTANCE_URL` | The URL of the Secrets Manager instance |
| `SM_SECRET_TASK_ID` | The ID of the task that the job run is currently operating on |
| `SM_SECRET_ID` | The ID of the secret being processed |
| `SM_SECRET_NAME` | The name of the secret being processed |
| `SM_SECRET_GROUP_ID` | The ID of the Secrets Manager secret group that contains the secret |
| `SM_CREDENTIALS_ID` | Only provided for `delete_credentials` action. The credentials ID assigned at creation |
| `SM_SECRET_VERSION_ID` | Only provided for `delete_credentials` action. The Secrets Manager secret version ID that the job run is operating on |

#### Job Custom Parameters

The job custom environment variables are defined in: [job_config.json](./job_config.json)

##### Required Parameters

| Environment Variable | Description |
|---------------------|-------------|
| `SMIN_COMMON_NAME` | The common name (CN) for the certificate (required) |

##### Optional Parameters

| Environment Variable | Description | Default Value |
|---------------------|-------------|---------------|
| `SMIN_ORG` | Organization name to include in the certificate | (empty) |
| `SMIN_COUNTRY` | Country code to include in the certificate | (empty) |
| `SMIN_SAN` | Subject Alternative Names as a comma-separated list | (empty) |
| `SMIN_EXPIRATION_DAYS` | Number of days until certificate expiration | 90 |
| `SMIN_KEY_ALGO` | Key algorithm to use (RSA or ECDSA) | RSA |
| `SMIN_SIGN_ALGO` | Signature algorithm to use (SHA256 or SHA512) | SHA256 |

#### Output Values

The job produces these values that are stored in Secrets Manager:

| Environment Variable | Description |
|---------------------|-------------|
| `SMOUT_PRIVATE_KEY_BASE64` | The base64-encoded private key (PEM format) (required) |
| `SMOUT_CERTIFICATE_BASE64` | The base64-encoded certificate (PEM format) (required) |


## Development

### Project Structure

```
example-certificate-provider-go/
├── cmd/
│   └── main.go                 - Entry point for the application
├── internal/
│   ├── job/
│   │   ├── certificate_provider.go - Contains the core logic for certificate generation
│   │   └── secrets_manager_job.go  - Manages integration with Secrets Manager API
│   └── utils/
│       └── logger.go           - Provides logging functionality
└── job_config.json             - Defines the input and output parameters for the job
```

* `main.go` - Simple entry point that calls the job's Run function
* `job_config.json` - Defines the input and output parameters for the job
* `certificate_provider.go` - Contains the core logic for certificate generation and management
* `secrets_manager_job.go` - Manages interaction with Secrets Manager API including configuration loading and task updates. This file was automatically generated using the [job-code-generator](../tools/README.md#using-the-job-code-generator) tool.
* `logger.go` - Provides structured logging capabilities with task and action identifiers

### Building and Testing

```bash
# Clone the secrets-manager-custom-credentials-providers repository to your local machine
git clone https://github.com/IBM/secrets-manager-custom-credentials-providers

# navigate to the example directory
cd example-certificate-provider-go

# Run test
go test ./internal/job

# Build the job binary
go build -o certificate-provider ./cmd
```

### How It Works

1. **Initialization**: The job reads the configuration from environment variables
2. **Key Generation**: A private key is generated using the specified algorithm (RSA or ECDSA)
3. **Certificate Creation**: A self-signed certificate is created based on the provided parameters
4. **Output**: The certificate and private key are base64-encoded and stored in Secrets Manager

### Certificate Properties

The generated certificates have the following properties:

* Self-signed X.509 certificates
* Configurable key algorithm (RSA 2048-bit or ECDSA P-256)
* Configurable signature algorithm (SHA256 or SHA512)
* Server authentication extended key usage
* Configurable validity period (default: 90 days)
* Support for Subject Alternative Names (SANs)

## Usage with IBM Cloud Secrets Manager

For ease of use, this example assumes that all services are deployed within the same IBM Cloud account, region, and resource group.

### Prerequisites

1. **Install IBM Cloud CLI**:

   ```bash
   curl -fsSL https://clis.cloud.ibm.com/install/linux | sh
   ```
   For macOS:
   ```bash
   curl -fsSL https://clis.cloud.ibm.com/install/osx | sh
   ```

2. **Install Code Engine CLI Plugin**:

   ```bash
   ibmcloud plugin install code-engine
   ```

3. **Install Secrets Manager CLI Plugin**:

   ```bash
   ibmcloud plugin install secrets-manager
   ```

4. **Login to IBM Cloud**:

   ```bash
   ibmcloud login [--sso]
   ```

### Step-by-Step Setup

#### 1. Create a Code Engine Project

```bash
# Set variables
REGION=us-south
RESOURCE_GROUP=Default
CE_PROJECT_NAME=certificate-provider
CE_JOB_NAME=certificate-provider-job

# Target your region and resource group
ibmcloud target -r $REGION -g $RESOURCE_GROUP

# Create a Code Engine project
ibmcloud ce project create --name $CE_PROJECT_NAME

# Capture the project ID
CE_PROJECT_ID=<project_id>

# Select the project
ibmcloud ce project select --name $CE_PROJECT_NAME
```

#### 2. Create the Job in Code Engine from Local Source Code

Use the [**job-deployer**](../tools/README.md#using-the-job-deployer-tool) tool to create and deploy the job

From the **certificate-provider-job** directory:

```bash
# Make the script executable by running:
chmod +x ../tools/job-deployer.sh
```

```bash
# Create a job from local example source code
../tools/job-deployer.sh --jobdir . --name $CE_JOB_NAME --action create

# Review the command and execute it. This command can take a few minutes to complete
ibmcloud ce job create --name certificate-provider-job 
  --build-source . 
  --build-dockerfile Dockerfile 
  --env SMIN_COMMON_NAME="type:string, required:true" 
  --env SMIN_ORG="type:string, required:false" 
  --env SMIN_COUNTRY="type:string, required:false" 
  --env SMIN_SAN="type:string, required:false" 
  --env SMIN_EXPIRATION_DAYS="type:integer, required:false" 
  --env SMIN_KEY_ALGO="type:enum[RSA|ECDSA], required:false" 
  --env SMIN_SIGN_ALGO="type:enum[SHA256|SHA512], required:false" 
  --env SMOUT_PRIVATE_KEY_BASE64="type:string, required:true" 
  --env SMOUT_CERTIFICATE_BASE64="type:string, required:true" 

Execute this command? (y/n): 
```

This command will:

* Upload your local source code to Code Engine
* Use the specified Dockerfile to build an image and store it in IBM Cloud Container Registry
* Create a job definition using the built image and using the environment variables defined in: [job_config.json](./job_config.json)  

#### 3. Create an IAM Service ID for Secrets Manager access

```bash
# Create Service ID
ibmcloud iam service-id-create certificate-provider-sid --description "Service ID for Secrets Manager Certificate provider"

# Capture the ID of this Service ID from the GUID field 
SERVICEID_ID=<serviceid_id>
```

This Service ID will later be assigned with an IAM **SecretTaskUpdater** service policy scoped to the Secrets Manager **Secret Group** containing the certificate-provider secrets.

#### 4. Configure Secrets Manager

Create a new Secrets Manager instance. Skip this step if you already have an instance of Secrets Manager service for testing:

```bash
# Provide a Secrets Manager instance name
SM_INSTANCE_NAME=secrets-manager-test

# Create an instance. This process takes a few minutes to complete
ibmcloud resource service-instance-create $SM_INSTANCE_NAME secrets-manager trial $REGION --resource-group $RESOURCE_GROUP

# Capture the instance ID
SM_INSTANCE_ID=<instance_id>

# Configure Secrets Manager CLI to use the Secrets Manager instance public endpoint
ibmcloud secrets-manager config set service-url https://$SM_INSTANCE_ID.$REGION.secrets-manager.appdomain.cloud

# Create a Secret Group to contain the certificate provider secrets
ibmcloud secrets-manager secret-group-create \
    --name certificate-provider-sg \
    --description "Secret Group containing certificate provider secrets"

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

Create an IAM service ID policy assigning **SecretTaskUpdater** role scoped to the Secrets Manager instance Secret Group containing the certificate-provider secrets:

```bash
# Create service policy
ibmcloud iam service-policy-create $SERVICEID_ID \
    --roles SecretTaskUpdater \
    --service-name secrets-manager \
    --service-instance $SM_INSTANCE_ID \
    --resource-type secret-group \
    --resource $SECRET_GROUP_ID
```

#### 6. Create an Secrets Manager IAM Credentials secret

Create an IAM Credentials secret for managing the IAM Service ID API key that the certificate-provider will use to authenticate back with Secrets Manager:

```bash
# Create IAM Credentials secret
ibmcloud secrets-manager secret-create \
    --secret-type iam_credentials \
    --secret-name certificate-provider-apikey \
    --secret-description "Secret managing the apikey for the certificate provider" \
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

#### 7. Create a Secrets Manager Custom Credentials configuration

Create a Custom Credentials Configuration for the Certificate provider:

```bash
# Create a configuration
ibmcloud secrets-manager configuration-create \
    --config-type custom_credentials_configuration \
    --name certificate-provider \
    --custom-credentials-apikey-ref "$IAM_CREDENTIALS_SECRET_ID" \
    --configuration-task-timeout 10m \
    --custom-credentials-code-engine "{
        \"region\": \"$REGION\",
        \"project_id\": \"$CE_PROJECT_ID\",
        \"job_name\": \"$CE_JOB_NAME\"
    }"
```

#### 8. Review the configured Certificate Provider Job

Review the configured Certificate Provider Job:

```bash
# Get job
ibmcloud ce job get --name $CE_JOB_NAME
```

Secrets Manager configuration injected the following environment variables:

* **Secret full reference  sm-access-secretid** - A reference to a Code Engine secret injected by Secrets Manager. This secret contains the IAM API key extracted from the IAM Credentials secret attached to the Certificate Provider Configuration to allow the Job to authenticate back with Secrets Manager.
* **SM_INSTANCE_URL** - The Secrets Manager endpoint URL.
* **CE_REMOVE_COMPLETED_JOBS=IMMEDIATELY** - A flag indicating Code Engine to remove completed jobs immediately. This is to avoid the accumulation of job runs and reaching the Code Engine 100 job runs per project quota limit.

#### **Tip:** When testing your Job you can change the value of **CE_REMOVE_COMPLETED_JOBS** to review completed job runs configurations and logs. For example using the Code Engine UI:

1. In the IBM Cloud Console select: Containers->Projects and open the **certificate-provider** project.
2. In the certificate-provider project page select Jobs and click on the Jobs tab. Then click on the **certificate-provider-job**.
3. In the Configuration page select the Environment variables tab.
4. Click on the line with **CE_REMOVE_COMPLETED_JOBS**.
5. In the Edit environment variable panel change the value from **IMMEDIATELY** to **3d** and click **Done**. 
6. Back in the Environment variables page click the **Deploy** button located on the top right of the page to update the Job. Job runs will then be maintained for 3 days.

#### 9. Create a Certificate Provider secret

```bash
# Create a secret
ibmcloud secrets-manager secret-create \
  --secret-type custom_credentials \
  --secret-configuration certificate-provider \
  --secret-name example-com-cert \
  --secret-description "Self-signed certificate for example.com" \
  --secret-group-id $SECRET_GROUP_ID \
  --secret-ttl 90d \
  --secret-parameters '{
    "common_name": "example.com",
    "san": "www.example.com,api.example.com",
    "org": "Example Inc",
    "country": "US",
    "expiration_days": 90,
    "key_algo": "RSA",
    "sign_algo": "SHA256"
  }' \
  --secret-rotation '{
    "auto_rotate": true,
    "interval": 60,
    "unit": "day"
  }'

# Capture the secret ID
CERT_SECRET_ID=<secret_id> 
```

This command creates the secret in a **Pre-Activation** state and a **Secret Task** that triggers an asynchronous job run in Code Engine. Once the job completes it updates the task with the new certifcate and the secret state becomes **Active**.

#### 10. View the Secret task state

List secret tasks:

```bash
# List secret tasks
ibmcloud secrets-manager secret-tasks --secret-id $CERT_SECRET_ID

# Capture the task ID
TASK_ID=<task_id>
```

The task id has format **sm-task-uuid** and is used as the job run name in Code Engine.

Observe the Job run log (requires using this [Tip](./README.md#tip-when-testing-your-job-you-can-change-the-value-of-ce_remove_completed_jobs-to-review-completed-job-runs-configurations-and-logs-for-example-using-the-code-engine-ui))

```bash
ibmcloud ce jobrun logs -f -n $TASK_ID
```


### Retrieving the Certificate

Once the job completes successfully, you can retrieve the certificate and private key from Secrets Manager. The response will contain the base64-encoded private key and certificate in the credentials section.

```bash
ibmcloud secrets-manager secret --id $CERT_SECRET_ID 

created_by            IBMid-3900062DBC
created_at            2025-03-23T11:22:26.000Z
description           Self-signed certificate for example.com
downloaded            true
id                    f4907e17-c81c-14ae-abd2-5a439edfec91
locks_total           0
name                  example-com-cert
secret_group_id       ce4fbe28-bdfb-c8c3-7aa5-52d2ad39bfa3
secret_type           custom_credentials
state                 1
state_description     active
updated_at            2025-03-23T11:23:37.000Z
versions_total        1
ttl                   7776000
rotation
                      auto_rotate   true
                      interval      60
                      unit          day

next_rotation_date    2025-05-22T11:22:33.000Z
tasks_total           1
queued_tasks          0
configuration         certificate-provider
parameters
                      common_name       example.com
                      country           US
                      expiration_days   90
                      key_algo          RSA
                      org               Example Inc
                      san               www.example.com,api.example.com
                      sign_algo         SHA256

credentials_content
                      certificate_base64   LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURYekNDQWtlZ0F3SUJBZ0lSQU9zTnZ1MVRhTFhiSHU1WUNmQmllcDR3RFFZSktvWk
                                           lodmNOQVFFTEJRQXcKT1RFTE1Ba0dBMVVFQmhNQ1ZWTXhGREFTQmdOVkJBb1RDMFY0WVcxd2JHVWdTVzVqTVJRd0VnWURWUVFERXd0bApl
                                           R0Z0Y0d4bExtTnZiVEFlRncweU5UQXpNak14TVRJeU16SmFGdzB5TlRBMk1qRXhNVEl5TXpKYU1Ea3hDekFKCkJnTlZCQVlUQWxWVE1...
                      private_key_base64   LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBMHBOOHgwMDE5a09HV3J6TGhBN2RrL0VaQndxd1FtRH
                                           ptZmhyUUpNYXdRSzgvbThVCnlsVDlkMGtQdURocXZzaXNUc09UbGppQ3U0RUVwRE5JSDJWbnYzTnR5bmNybGVpaWpJQkhFcEpST1I2UDh2
                                           VjgKWXVNa05mcWRjZG8xeGVacEJkcHB2dUQ2SFg5T0xvYmhodjl3djVES3EzZTNRNjh2U3VWSXRTZHBaalBuanVIMApKZ3JSOVlZTWl...

```

#### 11. Rotate the Certificate Provider secret

```bash
# Rotate the secret
ibmcloud secrets-manager secret-version-create \
        --secret-id $CERT_SECRET_ID \
        --secret-version-prototype {}

...

auto_rotated          false
created_by            IBMid-2400032DAH
created_at            2025-03-25T13:01:50.000Z
downloaded            false
id                    00000000-0000-0000-0000-000000000000
secret_name           example-com-cert
secret_type           custom_credentials
secret_group_id       ce4fbe28-bdfb-c8c3-7aa5-52d2ad39bfa3
payload_available     false
secret_id             f4907e17-c81c-14ae-abd2-5a439edfec91
credentials_content   -
```

Secrets Manager returns a temporary version with an ID of all zeros (00000000-0000-0000-0000-000000000000) and `payload_available: false`. This transient version acts as a placeholder until the new secret version is available, at which point Secrets Manager replaces it with the actual credentials.

### License

This example is open-source using Apache License 2.0.
