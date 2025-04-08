#!/bin/bash

# Function to display usage information
usage() {
    echo "Usage: $0 --jobdir <jobdir> --name <job_name> --action <create|update>"
    echo "Example: $0 --jobdir ./my-job --name certificate-provider-job --action create"
    exit 1
}

# Define colors
BLUE='\033[38;2;41;184;219m'  # #29B8DB in RGB
RED='\033[0;31m'             # Red color for errors
RESET='\033[0m'               # Reset text color

# Function to validate environment variables
validate_env_variables() {
    local config_file="$1"
    local has_errors=false
    local has_required_smout=false

    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is required but not installed. Please install jq.${RESET}"
        return 1
    fi

    # First validate that the JSON is well-formed
    if ! jq empty "$config_file" 2>/tmp/jq_error; then
        echo -e "${RED}Error: Invalid JSON format in $config_file${RESET}"
        cat /tmp/jq_error
        rm -f /tmp/jq_error
        return 1
    fi
    
    # Make sure job_env_variables exists and is an array
    if ! jq -e '.job_env_variables | type=="array"' "$config_file" >/dev/null 2>&1; then
        echo -e "${RED}Error: job_env_variables is missing or not an array in $config_file${RESET}"
        return 1
    fi

    # Extract all environment variables
    local num_vars=$(jq '.job_env_variables | length' "$config_file")
    
    for (( i=0; i<$num_vars; i++ )); do
        # Get name and value
        local name=$(jq -r ".job_env_variables[$i].name" "$config_file")
        local value=$(jq -r ".job_env_variables[$i].value" "$config_file")
        
        # Check if name and value exist
        if [[ "$name" == "null" ]]; then
            echo -e "${RED}Error: Missing 'name' field for variable at index $i${RESET}"
            has_errors=true
            continue
        fi
        
        if [[ "$value" == "null" ]]; then
            echo -e "${RED}Error: Missing 'value' field for variable '$name'${RESET}"
            has_errors=true
            continue
        fi
        
        # Check 1: Variable name must start with SMIN_ or SMOUT_
        if [[ ! "$name" =~ ^(SMIN_|SMOUT_) ]]; then
            echo -e "${RED}Error: Variable name '$name' must start with 'SMIN_' or 'SMOUT_'${RESET}"
            has_errors=true
        fi

        # Check 2: Variable name should not contain lowercase characters
        if [[ "$name" =~ [a-z] ]]; then
            echo -e "${RED}Error: Variable name '$name' should not contain lowercase characters${RESET}"
            has_errors=true
        fi

        # Check 3: Variable name should not have spaces
        if [[ "$name" =~ [[:space:]] ]]; then
            echo -e "${RED}Error: Variable name '$name' should not contain spaces${RESET}"
            has_errors=true
        fi

        # Parse attributes from value
        local has_type=false
        local type_value=""
        local required_value=""
        
        # Split value by comma and process each attribute
        IFS=',' read -ra ATTRS <<< "$value"
        for attr in "${ATTRS[@]}"; do
            # Trim whitespace
            attr=$(echo "$attr" | xargs)
            
            # Extract attribute name and value
            if [[ "$attr" =~ ^([^:]+):(.+)$ ]]; then
                local attr_name="${BASH_REMATCH[1]}"
                local attr_val="${BASH_REMATCH[2]}"
                
                # Trim whitespace
                attr_name=$(echo "$attr_name" | xargs)
                attr_val=$(echo "$attr_val" | xargs)
                
                # Check attribute name
                if [[ "$attr_name" == "type" ]]; then
                    has_type=true
                    type_value="$attr_val"
                    
                    # Check type value is valid
                    if [[ "$attr_val" =~ ^enum\[ ]]; then
                        # Check enum format (enum[optionA|optionB|...])
                        if [[ ! "$attr_val" =~ ^enum\[[^]]+\]$ ]]; then
                            echo -e "${RED}Error: Variable '$name' has invalid enum format. Expected: enum[optionA|optionB|...]${RESET}"
                            has_errors=true
                        fi
                    elif [[ "$attr_val" != "string" && "$attr_val" != "integer" && "$attr_val" != "boolean" && "$attr_val" != "secret_id" && "$attr_val" != "enum" ]]; then
                        echo -e "${RED}Error: Variable '$name' has invalid type '$attr_val'. Must be one of: string, integer, boolean, secret_id, enum${RESET}"
                        has_errors=true
                    fi
                elif [[ "$attr_name" == "required" ]]; then
                    required_value="$attr_val"
                    
                    # Check required value is valid
                    if [[ "$attr_val" != "true" && "$attr_val" != "false" ]]; then
                        echo -e "${RED}Error: Variable '$name' has invalid 'required' value '$attr_val'. Must be true or false${RESET}"
                        has_errors=true
                    fi
                    
                    # Check for SMOUT_ variable with required:true
                    if [[ "$name" =~ ^SMOUT_ && "$attr_val" == "true" ]]; then
                        has_required_smout=true
                    fi
                else
                    # Invalid attribute
                    echo -e "${RED}Error: Variable '$name' contains invalid attribute '$attr_name'${RESET}"
                    has_errors=true
                fi
            else
                # Attribute doesn't follow name:value format
                echo -e "${RED}Error: Variable '$name' has invalid attribute format: '$attr'${RESET}"
                has_errors=true
            fi
        done
        
        # Check 4: Value must contain "type" attribute
        if [ "$has_type" = false ]; then
            echo -e "${RED}Error: Variable '$name' value does not contain a 'type' attribute${RESET}"
            has_errors=true
        fi
    done
    
    # Check that at least one SMOUT_ variable with required:true is defined
    if [ "$has_required_smout" = false ]; then
        echo -e "${RED}Error: At least one SMOUT_ variable with attribute required:true must be defined${RESET}"
        has_errors=true
    fi

    # Return 1 if there are errors, 0 otherwise
    if [ "$has_errors" = true ]; then
        return 1
    fi
    return 0
}

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --jobdir) JOB_DIR="$2"; shift ;;
        --name) JOB_NAME="$2"; shift ;;
        --action) ACTION="$2"; shift ;;
        *) echo "Unknown parameter: $1"; usage ;;
    esac
    shift
done

# Check if all required parameters are provided
if [ -z "$JOB_DIR" ] || [ -z "$JOB_NAME" ] || [ -z "$ACTION" ]; then
    echo -e "${RED}Error: Missing required parameters!${RESET}"
    usage
fi

# Validate action parameter
if [ "$ACTION" != "create" ] && [ "$ACTION" != "update" ]; then
    echo -e "${RED}Error: Action must be either 'create' or 'update'!${RESET}"
    usage
fi

CONFIG_FILE="$JOB_DIR/job_config.json"
DOCKERFILE="$JOB_DIR/Dockerfile"

# Check if job_config.json exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: $CONFIG_FILE not found!${RESET}"
    exit 1
fi

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    echo -e "${RED}Error: $DOCKERFILE not found!${RESET}"
    exit 1
fi

# Validate environment variables in job_config.json
echo "Validating environment variables in $CONFIG_FILE..."
if ! validate_env_variables "$CONFIG_FILE"; then
    echo -e "${RED}Error: Invalid environment variables in $CONFIG_FILE. Please fix the issues and try again.${RESET}"
    exit 1
fi

echo "Environment variables validation passed."

# Print the command header in color
if [ "$ACTION" == "create" ]; then
    echo -e "${BLUE}ibmcloud ce job create --name $JOB_NAME ${RESET}"
    echo -e "${BLUE}  --build-source $JOB_DIR ${RESET}"
    echo -e "${BLUE}  --build-dockerfile Dockerfile ${RESET}"
else
    echo -e "${BLUE}ibmcloud ce job update --name $JOB_NAME ${RESET}"
    echo -e "${BLUE}  --build-source $JOB_DIR ${RESET}"
    echo -e "${BLUE}  --build-dockerfile Dockerfile ${RESET}"
fi

# Parse the job_config.json file and print each env parameter on a new line
# First validate that the JSON is valid to avoid errors
if jq empty "$CONFIG_FILE" 2>/dev/null; then
    ENV_FLAGS=""
    while read -r line; do
        # Skip empty lines
        if [ -z "$line" ]; then
            continue
        fi
        
        # Extract name and value
        name=$(echo "$line" | cut -d' ' -f1)
        value=${line#* }
        
        # Print each environment variable on a new line
        echo -e "${BLUE}  --env $name=\"$value\" ${RESET}"
        
        # Build the actual command string for execution
        ENV_FLAGS="$ENV_FLAGS --env $name=\"$value\""
    done < <(jq -r '.job_env_variables[] | "\(.name) \(.value)"' "$CONFIG_FILE" 2>/dev/null)
else
    echo -e "${RED}Warning: Skipping environment variables due to invalid JSON format.${RESET}"
    ENV_FLAGS=""
fi

# Remove the trailing backslash from the display
echo -e "${BLUE}${RESET}"

# Build the full command for execution
if [ "$ACTION" == "create" ]; then
    CMD="ibmcloud ce job create --name $JOB_NAME --build-source $JOB_DIR --build-dockerfile Dockerfile --retrylimit 0 --cpu 0.5 --memory 1G $ENV_FLAGS"
else
    CMD="ibmcloud ce job update --name $JOB_NAME --build-source $JOB_DIR --build-dockerfile Dockerfile $ENV_FLAGS"
fi

# Ask for confirmation
echo
read -p "Execute this command? (y/n): " confirm
if [[ $confirm != [yY] ]]; then
    echo "Operation cancelled."
    exit 0
fi

# Execute the command
eval "$CMD"

# Report status
if [ $? -eq 0 ]; then
    echo "Job $ACTION operation completed successfully for: $JOB_NAME"
else
    echo "Job $ACTION operation failed for: $JOB_NAME"
    exit 1
fi