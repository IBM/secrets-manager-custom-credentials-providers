# Credentials Provider tools

## Job Code Generator

### Overview

The `job-code-generator` is a code generation tool designed to streamline the development of credentials provider jobs for Secrets Manager written in Go. It automates the integration with Secrets Manager, enabling developers to focus on the provider core business logic.

#### Key Benefits

* **Automatic code generation based on the `job_config.json` file:**<br>
  Generates a `secrets_manager_job.go` file containing the following artifacts:
  * Strongly-typed `Config` and `CredentialsPayload` structs
  * Functions for interacting with Secrets Manager
* **Type safety:** Ensures correct typing using Go structs with validation tags.
* **Built-in validation:** Includes rules for required fields and max string lengths.
* **Dependency injection:** Uses interfaces for Secrets Manager clients to support unit testing.
* **Simplified API interactions:** Abstracts environment variable handling, name mapping, and Secrets Manager API calls.

### Building the Code Generator

#### Prerequisites

* GO development environment

```bash
# navigate to the tools directory
cd tools

# build the generator
go build -o job-code-generator .
```

### Usage

To generate a Secrets Manager job file, run the following command:

```bash
./job-code-generator -jobdir=<job_directory> -jobfiledir=<job_file_directory> [-package=<package_name>] [--force]
```

### Options

* `-jobdir` (required): Path to the directory containing `job_config.json`
* `-jobfiledir` (required): Directory where `secrets_manager_job.go` will be generated
* `-package` (optional): Package name for the generated code (default: `job`)
* `--force` (optional): Overwrite existing files

### Example

Assuming the following job structure:

```
my-job/
├── cmd/
│   └── main.go
├── internal/
│   └── job/
│       ├── credentials_provider.go
│       └── secrets_manager_job.go
└── job_config.json
```

Run the generator:

```bash
./job-code-generator -jobdir=path_to/my-job -jobfiledir=path_to/my-job/internal/job
```

This will create the `secrets_manager_job.go` file in `./my-job/internal/job` containing the necessary structs, and helper functions.

### License

This tool is open-source using Apache License 2.0.

## Job Deployer

### Overview

**job-deployer.sh** is a Bash script designed to automate the deployment of Code Engine Jobs used for Secrets Manager custom credentials. It automates job creation and updates based on the `job_config.json` file.

### Features

* Reads environment variables from a `job_config.json` file.
* Supports specifying the local job directory, name, and action (create or update).
* Validates required files (`job_config.json` and `Dockerfile`)
* Prompts for confirmation before execution to prevent accidental changes.
* Deploys a job from **local source code** to IBM Cloud Code Engine project.

### Prerequisites

Before using this script, ensure you have:

* IBM Cloud CLI installed ([Installation Guide](https://cloud.ibm.com/docs/cli))
* IBM Cloud Code Engine plugin installed:

  ```bash
  ibmcloud plugin install code-engine
  ```

* `jq` for parsing JSON:

  ```bash
  sudo apt install jq  # Ubuntu/Debian
  brew install jq      # macOS
  ```

* Login to IBM Cloud and target your resource group and Code Engine project:

  ```bash
  ibmcloud login [--sso]
  ibmcloud target -g <resource-group>
  ibmcloud ce project select --name <project-name>
  ```

### Usage

#### Command Syntax

```bash
./job-deployer.sh --jobdir <jobdir> --name <job_name> --action <create|update>
```

#### Example Usage

##### Creating a New Job

```bash
./job-deployer.sh --jobdir path_to/my-job --name my-job --action create
```

##### Updating an Existing Job

```bash
./job-deployer.sh --jobdir path_to/my-job --name my-job --action update
```

### Troubleshooting

* If the script fails due to authentication issues, try to login again:

  ```bash
  ibmcloud login [--sso]
  ```

* Target your resource group:

  ```bash
  ibmcloud target -g <resource-group>
  ```

* Check that the correct project is selected:

  ```bash
  ibmcloud ce project list
  ibmcloud ce project select --name <project-name>
  ```

* If the script fails due to Code Engine issues, refer to [Code Engine Troubleshooting guide](https://cloud.ibm.com/docs/codeengine?topic=codeengine-troubleshooting_over).

### License

This tool is open-source using Apache License 2.0.
