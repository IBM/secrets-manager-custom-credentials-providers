# IBM Cloud Secrets Manager Credentials Provider for User IAM API Keys

This is an example Go application designed to run as an IBM Cloud Code Engine [job](https://cloud.ibm.com/docs/codeengine?topic=codeengine-job-plan) for IBM Cloud Secrets Manager [custom credentials](https://cloud.ibm.com/docs/secrets-manager?topic=secrets-manager-getting-started) secret type. This job dynamically generates [User IBM Cloud IAM API Keys](https://cloud.ibm.com/docs/account?topic=account-manapikey).

## Overview

When triggered by Secrets Manager, the job performs two main operations:

* **Credentials Creation** - Generates a new user IAM API Key.
* **Credentials Deletion** - Deletes a previously created user IAM API Key.

## Configuration

### Environment Variables

The job uses service and custom environment variables for configuration, which are automatically provided by Secrets Manager when the job is triggered.

#### Service Parameters

The service environment variables that are passed by Secrets Manager to the job:

| Environment Variable   | Description                                                                                                                                                                                              |
|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `SM_ACTION`            | Specifies whether to create or delete credentials (`create_credentials` or `delete_credentials`)                                                                                                         |
| `SM_TRIGGER`           | Specifies the action that triggered this task. Allowed values are: `secret_creation`, `manual_secret_rotation`, `automatic_secret_rotation`, `secret_version_expiration`, `secret_version_data_deletion` |
| `SM_INSTANCE_URL`      | The URL of the Secrets Manager instance                                                                                                                                                                  |
| `SM_SECRET_TASK_ID`    | The ID of the task that the job run is currently operating on                                                                                                                                            |
| `SM_SECRET_ID`         | The ID of the secret being processed                                                                                                                                                                     |
| `SM_SECRET_NAME`       | The name of the secret being processed                                                                                                                                                                   |
| `SM_SECRET_GROUP_ID`   | The ID of the Secrets Manager secret group that contains the secret                                                                                                                                      |
| `SM_CREDENTIALS_ID`    | Provided only for the `delete_credentials` action. The credentials ID assigned at creation.                                                                                                              |
| `SM_SECRET_VERSION_ID` | Provided only for the `delete_credentials` action. The Secrets Manager secret version ID that the job run is operating on                                                                                |

#### Job Custom Parameters

The job custom environment variables are defined in: [job_config.json](./job_config.json)

##### Required Parameters

| Environment Variable   | Description                                                                           |
|------------------------|---------------------------------------------------------------------------------------|
| `SMIN_APIKEY_SECRET_ID` | Arbitrary secret ID containing the an API key to use for authentication.              |

##### Optional Parameters

| Environment Variable      | Description                                                                                                                                         | Default Value                                                    |
|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `SMIN_IAM_ID`             | The IAM ID that the created API key authenticates.                                                                                                  | Inherited from the API key referenced in `SMIN_APIKEY_SECRET_ID` |
| `SMIN_ACCOUNT_ID`         | The account ID for the created API key.                                                                                                             | Inherited from the API key referenced in `SMIN_APIKEY_SECRET_ID` |
| `SMIN_SUPPORT_SESSIONS`   | Defines whether you can manage CLI login sessions for the API key.                                                                                  | `false`                                                          |
| `SMIN_ACTION_WHEN_LEAKED` | Defines the action to take when API key is leaked, valid values are `none`, `disable` and `delete`.                                                 | `none`                                                           |
| `SMIN_URL`                | The URL of the IAM service.                                                                                                                         | `https://iam.cloud.ibm.com`                                                             |

#### Output Values

The job produces these values that are stored in Secrets Manager:

| Environment Variable | Description                                         |
|----------------------|-----------------------------------------------------|
| `SMOUT_APIKEY`       | The generated API key value.                        |
| `SMOUT_ID`           | The ID of the generated API key.                    |
| `SMOUT_CRN`          | The CRN of the generated API key.                   |
| `SMOUT_IAM_ID`       | The IAM ID associated with the generated API key.   |
| `SMOUT_ACCOUNT_ID`   | The account ID where the generated API key resides. |

## Development

### Project Structure

```
ibmcloud-iam-user-apikey-provider-go/
├── cmd/
│   └── main.go                            - Entry point for the application
├── identity_services_wrapper/
│   └── identity_services_wrapper.go       - Handles interacting with IAM Identity Services Go SDK
├── internal/
│   ├── job/
│   │   └── credentials_provider.go        - Implements user IAM API keys management
│   │   └── secrets_manager_job.go         - Handles integration with IBM Cloud Secrets Manager
│   └── utils/
│       └── logger.go                      - Provides logging functionality
└── job_config.json                        - Defines the input and output parameters for the job
```

### Building and Testing

```bash
# Clone the secrets-manager-custom-credentials-providers repository to your local machine
git clone https://github.com/IBM/secrets-manager-custom-credentials-providers

# navigate to the provider directory
cd ibmcloud-iam-user-apikey-provider-go

# Build the job binary
go build -o ibmcloud-iam-user-apikey-provider-go ./cmd
```

### How It Works

1. **Initialization**: Reads configuration from environment variables.
2. **Login Credentials Retrieval**: Retrieves API key from a Secrets Manager Arbitrary secret.
3. **Credentials Generation**: Creates a new user IAM API key.
4. **Output**: Provides new credentials back to Secrets Manager.

## License

This provider is open-source using Apache License 2.0.

