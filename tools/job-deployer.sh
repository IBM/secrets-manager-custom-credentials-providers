#!/bin/bash

# Function to display usage information
usage() {
    echo "Usage: $0 --jobdir <jobdir> --name <job_name> --action <create|update>"
    echo "Example: $0 --jobdir ./my-job --name certificate-provider-job --action create"
    exit 1
}

# Define colors
BLUE='\033[38;2;41;184;219m'  # #29B8DB in RGB
RESET='\033[0m'               # Reset text color

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
    echo "Error: Missing required parameters!"
    usage
fi

# Validate action parameter
if [ "$ACTION" != "create" ] && [ "$ACTION" != "update" ]; then
    echo "Error: Action must be either 'create' or 'update'!"
    usage
fi

CONFIG_FILE="$JOB_DIR/job_config.json"
DOCKERFILE="$JOB_DIR/Dockerfile"

# Check if job_config.json exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: $CONFIG_FILE not found!"
    exit 1
fi

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    echo "Error: $DOCKERFILE not found!"
    exit 1
fi

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
ENV_FLAGS=""
while read -r name value; do
    # Remove any quotes from the name and value
    name=$(echo "$name" | tr -d '"')
    value=$(echo "$value" | tr -d '"')
    
    # Skip empty lines
    if [ -z "$name" ]; then
        continue
    fi
    
    # Print each environment variable on a new line
    echo -e "${BLUE}  --env $name=\"$value\" ${RESET}"
    
    # Build the actual command string for execution
    ENV_FLAGS="$ENV_FLAGS --env $name=\"$value\""
done < <(jq -r '.job_env_variables[] | "\(.name) \(.value)"' "$CONFIG_FILE")

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