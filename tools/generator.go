package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// JobEnvVariable represents a single environment variable entry.
type JobEnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// JobConfig represents the user input job configuration.
type JobConfig struct {
	JobEnvVariables []JobEnvVariable `json:"job_env_variables"`
}

// CommonJobConfig represents the common job configuration.
type CommonJobConfig struct {
	CommonEnvVariables []JobEnvVariable `json:"common_env_variables"`
}

// ValidationError represents an error during validation
type ValidationError struct {
	VariableName string
	Message      string
}

// Built-in job configuration.
const builtinJobConfig = `{
    "common_env_variables": [
        {
            "name": "SM_ACCESS_APIKEY",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_INSTANCE_URL",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_SECRET_GROUP_ID",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_SECRET_NAME",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_SECRET_TASK_ID",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_CREDENTIALS_ID",
            "value": "type:string"
        },
        {
            "name": "SM_SECRET_VERSION_ID",
            "value": "type:string"
        },
        {
            "name": "SM_SECRET_ID",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_ACTION",
            "value": "type:string, required:true"
        },
        {
            "name": "SM_TRIGGER",
            "value": "type:string, required:true"
        }
    ]
}`

func main() {
	// Define and parse command-line flags.
	jobDir := flag.String("jobdir", "", "Path to the job project directory")
	jobFileDir := flag.String("jobfiledir", "", "Directory where the secrets manager job file will be generated")
	packageName := flag.String("package", "job", "Optional package name for the generated file")
	force := flag.Bool("force", false, "Overwrite existing files if set to true")
	flag.Parse()

	if *jobDir == "" || *jobFileDir == "" {
		fmt.Println("Usage: secrets-manager-job-generator -jobdir=<job_directory> -jobfiledir=<job_file_directory> [-package=<package_name>] [--force]")
		os.Exit(1)
	}

	// Read and parse the user input job configuration file.
	userData, err := os.ReadFile(fmt.Sprintf("%s/job_config.json", *jobDir))
	if err != nil {
		fmt.Printf("Error reading job configuration file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processing configuration file:\n%s\n", string(userData))
	var userSchema *JobConfig
	if err := json.Unmarshal(userData, &userSchema); err != nil {
		fmt.Printf("Error parsing job configuration file: %v\n", err)
		os.Exit(1)
	}

	if userSchema.JobEnvVariables == nil || len(userSchema.JobEnvVariables) == 0 {
		fmt.Printf("Job configuration file does not define any variables")
		os.Exit(1)
	}

	// Validate the user input job configuration
	if errors := validateJobConfig(userSchema); len(errors) > 0 {
		fmt.Printf("Invalid job configuration. Found %d validation errors:\n", len(errors))
		for i, err := range errors {
			fmt.Printf("%d. Variable '%s': %s\n", i+1, err.VariableName, err.Message)
		}
		os.Exit(1)
	}

	// Parse the built-in common job configuration.
	var commonJobConfig *CommonJobConfig
	if err := json.Unmarshal([]byte(builtinJobConfig), &commonJobConfig); err != nil {
		fmt.Printf("Error parsing common job configuration: %v\n", err)
		os.Exit(1)
	}

	// Ensure the job file directory exists.
	if err := os.MkdirAll(*jobFileDir, 0755); err != nil {
		fmt.Printf("Error creating job file directory: %v\n", err)
		os.Exit(1)
	}

	outputPath := filepath.Join(*jobFileDir, "secrets_manager_job.go")
	if _, err := os.Stat(outputPath); err == nil && !*force {
		fmt.Printf("File %s already exists. Use --force to overwrite.\n", outputPath)
		os.Exit(1)
	}

	// Generate the code
	code, err := GenerateCode(commonJobConfig, userSchema, *packageName)
	if err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}

	// Write the code to file
	err = os.WriteFile(outputPath, []byte(code), 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Code generated successfully.")
}

// validateJobConfig validates the user input job configuration according to the specified rules
func validateJobConfig(jobConfig *JobConfig) []ValidationError {
	var errors []ValidationError

	// Regex for valid variable names
	namePattern := regexp.MustCompile(`^(SMIN_|SMOUT_)[A-Z0-9_]+$`)

	// Valid types
	validTypes := map[string]bool{
		"string":    true,
		"integer":   true,
		"boolean":   true,
		"secret_id": true,
	}

	// Validate each variable
	for _, envVar := range jobConfig.JobEnvVariables {
		name := envVar.Name
		value := envVar.Value

		// Check prefix
		if !strings.HasPrefix(name, "SMIN_") && !strings.HasPrefix(name, "SMOUT_") {
			errors = append(errors, ValidationError{
				VariableName: name,
				Message:      "Variable name must start with 'SMIN_' or 'SMOUT_'",
			})
		}

		// Check variable name format (no lowercase, no spaces)
		if !namePattern.MatchString(name) {
			errors = append(errors, ValidationError{
				VariableName: name,
				Message:      "Variable name should only contain uppercase letters, numbers, and underscores",
			})
		}

		// Validate value attributes
		attrType, validations, err := parseAttributes(value)
		if err != nil {
			errors = append(errors, ValidationError{
				VariableName: name,
				Message:      fmt.Sprintf("Invalid attribute format: %v", err),
			})
			continue
		}

		// Check if type is specified
		if attrType == "" {
			errors = append(errors, ValidationError{
				VariableName: name,
				Message:      "Variable value must specify a 'type' attribute",
			})
		}

		// Check if type is valid
		if !validTypes[attrType] && !strings.HasPrefix(attrType, "enum[") {
			errors = append(errors, ValidationError{
				VariableName: name,
				Message:      fmt.Sprintf("Invalid type '%s'. Must be one of: string, integer, boolean, secret_id, or enum[options]", attrType),
			})
		}

		// Check enum format
		if strings.HasPrefix(attrType, "enum[") {
			if !strings.HasSuffix(attrType, "]") || len(attrType) <= 6 {
				errors = append(errors, ValidationError{
					VariableName: name,
					Message:      "Invalid enum format. Must be in format 'enum[optionA|optionB|...]'",
				})
			} else {
				// Check if enum options are properly formatted
				options := attrType[5 : len(attrType)-1] // Remove 'enum[' prefix and ']' suffix
				if options == "" || !strings.Contains(options, "|") {
					errors = append(errors, ValidationError{
						VariableName: name,
						Message:      "Enum must have at least two options separated by '|'",
					})
				}
			}
		}

		// Check if 'required' attribute value is valid
		if reqVal, ok := validations["required"]; ok {
			if reqVal != "true" && reqVal != "false" {
				errors = append(errors, ValidationError{
					VariableName: name,
					Message:      "Required attribute must be 'true' or 'false'",
				})
			}
		}

		// Check if there are invalid attributes
		for key := range validations {
			if key != "required" {
				errors = append(errors, ValidationError{
					VariableName: name,
					Message:      fmt.Sprintf("Invalid attribute '%s'. Only 'type' and 'required' attributes are accepted", key),
				})
			}
		}
	}

	return errors
}

func GenerateMustGetEnvVar(fileBuilder *strings.Builder) {
	fileBuilder.WriteString("// MustGetEnvVar returns the value of the environment variable or an error if it's not set\n")
	fileBuilder.WriteString("func MustGetEnvVar(key string) (string, error) {\n")
	fileBuilder.WriteString("\tvalue := os.Getenv(key)\n")
	fileBuilder.WriteString("\tif value == \"\" {\n")
	fileBuilder.WriteString("\t\treturn \"\", fmt.Errorf(\"environment variable %s is required but not set\", key)\n")
	fileBuilder.WriteString("\t}\n")
	fileBuilder.WriteString("\treturn value, nil\n")
	fileBuilder.WriteString("}\n\n")
}

func GenerateGetEnvVar(fileBuilder *strings.Builder) {
	fileBuilder.WriteString("// GetEnvVar returns the value of the environment variable for the given key\n")
	fileBuilder.WriteString("func GetEnvVar(key string) string {\n")
	fileBuilder.WriteString("\treturn os.Getenv(key)\n")
	fileBuilder.WriteString("}\n\n")
}

func GenerateProcessValue(fileBuilder *strings.Builder) {
	fileBuilder.WriteString("// Helper function to process values based on their type\n")
	fileBuilder.WriteString("func processValue(value string, valueType string) (interface{}, error) {\n")
	fileBuilder.WriteString("\tswitch valueType {\n")
	fileBuilder.WriteString("\tcase \"string\":\n")
	fileBuilder.WriteString("\t\treturn value, nil\n")
	fileBuilder.WriteString("\tcase \"integer\":\n")
	fileBuilder.WriteString("\t\treturn strconv.Atoi(value)\n")
	fileBuilder.WriteString("\tcase \"boolean\":\n")
	fileBuilder.WriteString("\t\treturn strconv.ParseBool(value)\n")
	fileBuilder.WriteString("\tdefault:\n")
	fileBuilder.WriteString("\t\treturn value, nil // Default to string if type is unknown\n")
	fileBuilder.WriteString("\t}\n")
	fileBuilder.WriteString("}\n\n")
}

// GenerateConfigFromEnv generates the ConfigFromEnv function that loads and validates config from environment variables
func GenerateConfigFromEnv(fileBuilder *strings.Builder, commonJobConfig *CommonJobConfig, userSchema *JobConfig) {
	// Generate the ConfigFromEnv function
	fileBuilder.WriteString("// ConfigFromEnv creates a Config from environment variables and validates it\n")
	fileBuilder.WriteString("func ConfigFromEnv() (Config, error) {\n")
	fileBuilder.WriteString("\tvar config Config\n")
	fileBuilder.WriteString("\tvar errs []string\n\n")

	// Declare the variables outside the loops to avoid redeclaration
	fileBuilder.WriteString("\t// Declare common variables\n")
	fileBuilder.WriteString("\tvar value string\n")
	fileBuilder.WriteString("\tvar processedValue interface{}\n")
	fileBuilder.WriteString("\tvar err error\n")

	// Process common variables with direct mapping
	fileBuilder.WriteString("\t// Process common variables\n")
	for _, envVar := range commonJobConfig.CommonEnvVariables {
		name := strings.TrimSpace(envVar.Name)
		value := strings.TrimSpace(envVar.Value)
		// Parse attributes to determine if required
		_, validations, err := parseAttributes(envVar.Value)
		if err != nil {
			fmt.Printf("Error parsing attributes '%s' for common variable '%s': %v\n", value, name, err)
			os.Exit(1)
		}

		// Check if this common variable is explicitly required
		isRequired := false
		if reqVal, ok := validations["required"]; ok {
			if reqVal == "true" {
				isRequired = true
			}
		}
		if isRequired {
			fileBuilder.WriteString(fmt.Sprintf("\tvalue, err = MustGetEnvVar(\"%s\")\n", name))
			fileBuilder.WriteString("\tif err != nil {\n")
			fileBuilder.WriteString("\t\terrs = append(errs, err.Error())\n")
			fileBuilder.WriteString("\t} else {\n")
			fileBuilder.WriteString(fmt.Sprintf("\t\tconfig.%s = value\n", name))
			fileBuilder.WriteString("\t}\n\n")
		} else {
			fileBuilder.WriteString(fmt.Sprintf("\tvalue = GetEnvVar(\"%s\")\n", name))
			fileBuilder.WriteString(fmt.Sprintf("\tconfig.%s = value\n", name))
			fileBuilder.WriteString("\n")
		}
	}

	// Process user variables with special mapping
	fileBuilder.WriteString("\t// Process user variables\n")
	for _, envVar := range userSchema.JobEnvVariables {
		if strings.HasPrefix(envVar.Name, "SMIN_") {
			// Field name in Config: remove "SMIN_" prefix and add "SM_" prefix
			fieldName := "SM_" + strings.TrimPrefix(envVar.Name, "SMIN_")
			// Environment variable name: replace "SMIN_" with "SM_" and append "_VALUE"
			envVarName := "SM_" + strings.TrimPrefix(envVar.Name, "SMIN_") + "_VALUE"

			// Parse attributes to determine if a required variable and the type for potential conversion
			attrType, validations, err := parseAttributes(envVar.Value)
			if err != nil {
				fmt.Printf("Error parsing attributes '%s' for user variable '%s': %v\n", envVar.Value, envVar.Name, err)
				os.Exit(1)
			}

			// Check if this user variable is explicitly required
			isRequired := false
			if reqVal, ok := validations["required"]; ok {
				if reqVal == "true" {
					isRequired = true
				}
			}
			fileBuilder.WriteString(fmt.Sprintf("\t// Process %s as %s\n", fieldName, attrType))
			fileBuilder.WriteString(fmt.Sprintf("\tvalue = GetEnvVar(\"%s\")\n", envVarName))
			fileBuilder.WriteString("\t\n")
			fileBuilder.WriteString("\t// Skip if value is empty and not explicitly required\n")
			fileBuilder.WriteString("\tif value == \"\" {\n")
			fileBuilder.WriteString(fmt.Sprintf("\t\tisRequired := %t\n", isRequired))
			fileBuilder.WriteString("\t\tif isRequired {\n")
			fileBuilder.WriteString(fmt.Sprintf("\t\t\terrs = append(errs, \"required environment variable %s is not set\")\n", envVarName))
			fileBuilder.WriteString("\t\t}\n")
			fileBuilder.WriteString("\t} else {\n")

			fileBuilder.WriteString("\t\t// Process the value based on type\n")
			fileBuilder.WriteString(fmt.Sprintf("\t\tprocessedValue, err = processValue(value, \"%s\")\n", attrType))
			fileBuilder.WriteString("\t\tif err != nil {\n")
			fileBuilder.WriteString("\t\t\terrs = append(errs, err.Error())\n")
			fileBuilder.WriteString("\t\t} else {\n")
			//fileBuilder.WriteString("\t\t\n")
			fileBuilder.WriteString("\t\t\t// Add to config\n")
			fileBuilder.WriteString(fmt.Sprintf("\t\t\treflect.ValueOf(&config).Elem().FieldByName(\"%s\").Set(reflect.ValueOf(processedValue))\n", fieldName))
			fileBuilder.WriteString("\t\t}\n")
			fileBuilder.WriteString("\t}\n\n")
		}
	}

	// Return errors or the config
	fileBuilder.WriteString("\tif len(errs) > 0 {\n")
	fileBuilder.WriteString("\t\treturn config, fmt.Errorf(\"configuration errors: %s\", strings.Join(errs, \"; \"))\n")
	fileBuilder.WriteString("\t}\n\n")
	fileBuilder.WriteString("\treturn config, nil\n")
	fileBuilder.WriteString("}\n\n")
}

// GenerateCode is the main function to generate all the code
func GenerateCode(commonJobConfig *CommonJobConfig, userSchema *JobConfig, packageName string) (string, error) {
	var fileBuilder strings.Builder

	// Generate imports
	fileBuilder.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	fileBuilder.WriteString("// Auto-generated by secrets-manager-job-generator\n\n")
	fileBuilder.WriteString("import (\n")
	fileBuilder.WriteString("\t\"encoding/json\"\n")
	fileBuilder.WriteString("\t\"errors\"\n")
	fileBuilder.WriteString("\t\"fmt\"\n")
	fileBuilder.WriteString("\t\"net/http\"\n")
	fileBuilder.WriteString("\t\"os\"\n")
	fileBuilder.WriteString("\t\"reflect\"\n")
	fileBuilder.WriteString("\t\"strconv\"\n")
	fileBuilder.WriteString("\t\"strings\"\n\n")
	fileBuilder.WriteString("\t\"github.com/IBM/go-sdk-core/v5/core\"\n")
	fileBuilder.WriteString("\tsm \"github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2\"\n")
	fileBuilder.WriteString("\t\"github.com/go-playground/validator\"\n")
	fileBuilder.WriteString(")\n\n")

	// Generate Config struct
	GenerateConfigStruct(&fileBuilder, commonJobConfig, userSchema)

	// Generate CredentialsPayload struct
	GenerateCredentialsPayloadStruct(&fileBuilder, commonJobConfig, userSchema)

	// Generate ConfigFromEnv function
	GenerateConfigFromEnv(&fileBuilder, commonJobConfig, userSchema)

	// GenerateSecretsManagerClient artifacts
	GenerateSecretsManagerClient(&fileBuilder)

	// Generate helper functions
	GenerateGetEnvVar(&fileBuilder)
	GenerateMustGetEnvVar(&fileBuilder)
	GenerateProcessValue(&fileBuilder)
	GenerateUpdateTaskFunctions(&fileBuilder)

	return fileBuilder.String(), nil
}

// GenerateConfigStruct generates the Config struct based on the commonJobConfig and userSchema
func GenerateConfigStruct(fileBuilder *strings.Builder, commonJobConfig *CommonJobConfig, userSchema *JobConfig) {
	fileBuilder.WriteString("// Config holds all configuration settings\n")
	fileBuilder.WriteString("type Config struct {\n")

	// Add fields for common variables
	fileBuilder.WriteString("\t// Common fields\n")
	for _, envVar := range commonJobConfig.CommonEnvVariables {
		name := strings.TrimSpace(envVar.Name)
		fileBuilder.WriteString(fmt.Sprintf("\t%s string\n", name))
	}

	// Add fields for user variables
	fileBuilder.WriteString("\n\t// User fields\n")
	for _, envVar := range userSchema.JobEnvVariables {
		if strings.HasPrefix(envVar.Name, "SMIN_") {
			// Field name in Config: remove "SMIN_" prefix and add "SM_" prefix
			fieldName := "SM_" + strings.TrimPrefix(envVar.Name, "SMIN_")

			// Parse attributes to determine type
			attrType, _, err := parseAttributes(envVar.Value)
			if err != nil {
				fmt.Printf("Error parsing attributes '%s' for variable: '%s': %v\n", envVar.Value, envVar.Name, err)
				os.Exit(1)
			}

			// Map attribute type to Go type
			goType := "string"
			switch attrType {
			case "integer":
				goType = "int"
			case "boolean":
				goType = "bool"
			}

			// Write field with comment
			fileBuilder.WriteString(fmt.Sprintf("\t%s %s // From env: %s\n", fieldName, goType, envVar.Name))
		}
	}

	fileBuilder.WriteString("}\n")
}

func GenerateCredentialsPayloadStruct(fileBuilder *strings.Builder, commonJobConfig *CommonJobConfig, userSchema *JobConfig) {
	// Generate the CredentialsPayload struct for SMOUT_ variables from the user schema.
	fileBuilder.WriteString("// CredentialsPayload contains fields for SMOUT_ environment variables\n")
	fileBuilder.WriteString("type CredentialsPayload struct {\n")
	requiredOutputVariableFound := false
	for _, envVar := range userSchema.JobEnvVariables {
		if strings.HasPrefix(envVar.Name, "SMOUT_") {
			// Field name: remove the "SMOUT_" prefix.
			fieldName := strings.TrimPrefix(envVar.Name, "SMOUT_")
			// Use an uppercase field name.
			fieldNameUpper := strings.ToUpper(fieldName)
			// Parse the attributes from the value string.
			attrType, validations, err := parseAttributes(envVar.Value)
			if err != nil {
				fmt.Printf("Error parsing attributes '%s' for user output variable '%s': %v\n", envVar.Value, envVar.Name, err)
				os.Exit(1)
			}

			// Check if this user variable is explicitly required
			isRequired := false
			if reqVal, ok := validations["required"]; ok {
				if reqVal == "true" {
					isRequired = true
					requiredOutputVariableFound = true
				}
			}
			goType := mapType(attrType)
			// Build JSON tag: use lower-case field name.
			jsonTag := strings.ToLower(fieldName)
			// Add validate tag with max=100000 for strings.
			validateTag := ""
			if goType == "string" {
				validateTag = "max=100000"
				if isRequired {
					validateTag = "required," + validateTag
				}
			}
			if validateTag != "" {
				fileBuilder.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\" validate:\"%s\"`\n", fieldNameUpper, goType, jsonTag, validateTag))
			} else {
				fileBuilder.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", fieldNameUpper, goType, jsonTag))
			}
		}
	}
	if requiredOutputVariableFound == false {
		fmt.Printf("Job configuration file must define at least one required output variable")
		os.Exit(1)
	}
	fileBuilder.WriteString("}\n\n")
}

func GenerateSecretsManagerClient(fileBuilder *strings.Builder) {
	fileBuilder.WriteString(`// Create interfaces for secrets manager client APIs
type SecretsManagerClient interface {
	GetSecret(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error)
	ReplaceSecretTask(options *sm.ReplaceSecretTaskOptions) (*sm.SecretTask, *core.DetailedResponse, error)
	NewSecretTaskError(code, description string) (*sm.SecretTaskError, error)
	NewCustomCredentialsNewCredentials(id string, credentials map[string]interface{}) (*sm.CustomCredentialsNewCredentials, error)
}

// Implement the interface with a concrete struct that wraps the actual secret manager client
type SMClient struct {
	client *sm.SecretsManagerV2
}

var validate = validator.New()

func (s *SMClient) GetSecret(options *sm.GetSecretOptions) (sm.SecretIntf, *core.DetailedResponse, error) {
	return s.client.GetSecret(options)
}

func (s *SMClient) ReplaceSecretTask(options *sm.ReplaceSecretTaskOptions) (*sm.SecretTask, *core.DetailedResponse, error) {
	return s.client.ReplaceSecretTask(options)
}

func (s *SMClient) NewSecretTaskError(code, description string) (*sm.SecretTaskError, error) {
	return s.client.NewSecretTaskError(code, description)
}

func (s *SMClient) NewCustomCredentialsNewCredentials(id string, credentials map[string]interface{}) (*sm.CustomCredentialsNewCredentials, error) {
	return s.client.NewCustomCredentialsNewCredentials(id, credentials)
}

// Function to create new client with configuration
func NewSecretsManagerClient(config Config) (SecretsManagerClient, error) {
	iamURL := getIAMURL(config.SM_INSTANCE_URL)

	service, err := sm.NewSecretsManagerV2(&sm.SecretsManagerV2Options{
		URL: config.SM_INSTANCE_URL,
		Authenticator: &core.IamAuthenticator{
			URL:    iamURL,
			ApiKey: config.SM_ACCESS_APIKEY,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Secrets Manager service: %w", err)
	}

	return &SMClient{client: service}, nil
}

func getIAMURL(instanceURL string) string {
	if strings.Contains(instanceURL, "secrets-manager.test.appdomain.cloud") {
		return "https://iam.test.cloud.ibm.com"
	}
	return "https://iam.cloud.ibm.com"
}

// GetSecret retrieves a secret from the IBM Cloud Secret Manager service.
func GetSecret(client SecretsManagerClient, id string) (sm.SecretIntf, error) {
	options := &sm.GetSecretOptions{ID: core.StringPtr(id)}
	res, resp, err := client.GetSecret(options)
	if err != nil {
		return nil, fmt.Errorf("cannot get secret with ID '%s': %w", id, err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot get secret with ID '%s'. unexpected status code %d", id, resp.StatusCode)
	}
	return res, nil
}

`)
}

func GenerateUpdateTaskFunctions(fileBuilder *strings.Builder) {
	fileBuilder.WriteString(`// UpdateTaskAboutCredentialsCreated updates a task status to succeeded and adds credentials to it.
func UpdateTaskAboutCredentialsCreated(client SecretsManagerClient, config *Config, credentialsPayload CredentialsPayload) (*sm.SecretTask, error) {
	credentialsPayloadMap, err := ValidatedStructToMap(credentialsPayload)
	if err != nil {
		return nil, fmt.Errorf("cannot convert credentials payload to map: %w", err)
	}

	customCredentials, err := client.NewCustomCredentialsNewCredentials(config.SM_CREDENTIALS_ID, credentialsPayloadMap)
	if err != nil {
		return nil, fmt.Errorf("cannot construct a custom credentials resource: %w", err)
	}

	secretTaskPrototype := &sm.SecretTaskPrototypeUpdateSecretTaskCredentialsCreated{
		Status:      core.StringPtr(sm.SecretTask_Status_CredentialsCreated),
		Credentials: customCredentials,
	}

	return UpdateTask(client, config, secretTaskPrototype)
}

// UpdateTaskAboutCredentialsDeleted updates a task status to succeeded when credentials are deleted.
func UpdateTaskAboutCredentialsDeleted(client SecretsManagerClient, config *Config) (result *sm.SecretTask, err error) {
	secretTaskPrototype := &sm.SecretTaskPrototypeUpdateSecretTaskCredentialsDeleted{
		Status: core.StringPtr(sm.SecretTask_Status_CredentialsDeleted),
	}
	return UpdateTask(client, config, secretTaskPrototype)
}

// UpdateTaskAboutError updates a task with the given code and description as errors.
func UpdateTaskAboutError(client SecretsManagerClient, config *Config, code, description string) (result *sm.SecretTask, err error) {

	secretTaskError, err := client.NewSecretTaskError(code, description)
	if err != nil {
		return nil, fmt.Errorf("cannot construct a new secret task error resource: %w", err)
	}

	secretTaskPrototype := &sm.SecretTaskPrototypeUpdateSecretTaskFailed{
		Status: core.StringPtr(sm.SecretTask_Status_Failed),
		Errors: []sm.SecretTaskError{*secretTaskError},
	}

	return UpdateTask(client, config, secretTaskPrototype)
}

// UpdateTask updates a secret task.
func UpdateTask(client SecretsManagerClient, config *Config, secretTaskPrototypeIntf sm.SecretTaskPrototypeIntf) (*sm.SecretTask, error) {
	options := &sm.ReplaceSecretTaskOptions{
		SecretID: &config.SM_SECRET_ID,
		ID:       &config.SM_SECRET_TASK_ID,
		TaskPut:  secretTaskPrototypeIntf,
	}

	result, response, err := client.ReplaceSecretTask(options)
	if err != nil {
		return nil, fmt.Errorf("cannot update secret with ID: '%s' task with ID: '%s'. error: %w",
			config.SM_SECRET_ID, config.SM_SECRET_TASK_ID, err)
	}

	if response == nil {
		return nil, fmt.Errorf("cannot update secret task, no response")
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("cannot update secret with ID: '%s' task with ID: '%s'. status code is: '%d', response is %s",
			config.SM_SECRET_ID, config.SM_SECRET_TASK_ID, response.StatusCode, response.String())
	}

	return result, nil
}
	
// ValidatedStructToMap converts a struct to a map[string]interface{} while performing validation
// according to the struct's validation tags
func ValidatedStructToMap(input any) (map[string]interface{}, error) {
	if input == nil {
		return nil, errors.New("input cannot be nil")
	}

	// Validate the struct based on validation tags
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Marshal the struct to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	// Unmarshal JSON back to a map
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return result, nil
}
	
func GetValueByPath(data map[string]interface{}, path string) (interface{}, bool) {
	segments := strings.Split(path, "/")

	var current interface{} = data // Use interface{} to allow type switching

	for _, segment := range segments {
		switch v := current.(type) {
		case map[string]interface{}:
			// Handle map keys
			val, exists := v[segment]
			if !exists {
				return nil, false
			}
			current = val
		case []interface{}:
			// Handle array indices
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(v) {
				return nil, false
			}
			current = v[index]
		default:
			return nil, false
		}
	}
	return current, true
}`)
}

// parseAttributes extracts the type and validation rules from an attribute string
func parseAttributes(value string) (string, map[string]string, error) {
	// Initialize empty map for validation attributes
	validations := make(map[string]string)

	// Initialize empty string for type
	var attrType string

	// Trim whitespace
	value = strings.TrimSpace(value)

	// Split by comma to get individual attributes
	attributes := strings.Split(value, ",")

	for _, attr := range attributes {
		// Trim whitespace from each attribute
		attr = strings.TrimSpace(attr)

		// Skip empty attributes
		if attr == "" {
			return "", nil, fmt.Errorf("attribute cannot be empty")
		}

		// Split attribute into key and value
		parts := strings.SplitN(attr, ":", 2)
		if len(parts) != 2 {
			return "", nil, fmt.Errorf("attribute must be in 'key:value' format")
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Check for empty key or value
		if key == "" || val == "" {
			return "", nil, fmt.Errorf("attribute key and value cannot be empty")
		}

		// Handle the attributes based on key
		if key == "type" {
			attrType = val
		} else {
			validations[key] = val
		}
	}

	return attrType, validations, nil
}

// mapType maps an attribute type from the schema to a Go type.
// For enums, it always returns "string".
func mapType(attrType string) string {
	if strings.HasPrefix(attrType, "enum[") && strings.HasSuffix(attrType, "]") {
		return "string"
	}
	switch attrType {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "secret_id":
		return "string"
	default:
		return "string"
	}
}
