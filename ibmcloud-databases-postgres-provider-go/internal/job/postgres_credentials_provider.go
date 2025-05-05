package job

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"postgres-credentials-provider/internal/utils"
	"strconv"
	"strings"

	sm "github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	certificatePath = "connection/postgres/certificate/certificate_base64"
	composedPath    = "connection/postgres/composed/0"
)

type pgAssembly struct {
	dbPool            *pgxpool.Pool
	certificate       []byte
	certificateBase64 string
	compose           string
}

// Global logger instance
var logger *utils.Logger

func Run() {

	// Setup
	config, err := ConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	client, err := NewSecretsManagerClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	logger = utils.NewLogger(config.SM_SECRET_TASK_ID, config.SM_ACTION)

	// Perform action
	switch config.SM_ACTION {
	case sm.SecretTask_Type_CreateCredentials:
		generatePGCredentials(client, &config)
	case sm.SecretTask_Type_DeleteCredentials:
		deletePGCredentials(client, &config)
	default:
		updateTaskAboutErrorAndExit(client, &config, Err10000, fmt.Sprintf("unknown action: '%s'", config.SM_ACTION))
	}
}

// generatePGCredentials generates postgres credentials for a given schema.
func generatePGCredentials(client SecretsManagerClient, config *Config) {

	// Set default values for non required config variables if not set by the user
	setDefaultValues(config)

	pg, err := obtainPGAssembly(client, config)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10001, fmt.Sprintf("error: %s", err.Error()))
	}
	defer pg.dbPool.Close()

	composedUrl, err := url.Parse(pg.compose)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10002, fmt.Sprintf("cannot parse postgres composed url: '%s' url. error:%s", pg.compose, err))
	}

	password, err := generateRolePassword(64) // Generate a 64-character password
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10003, fmt.Sprintf("cannot generate a new password: %s", err))
	}

	roleName := generateRoleName()
	schemaName := config.SM_SCHEMA_NAME

	roleOID, err := createReadOnlyRole(pg.dbPool, roleName, password, schemaName)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10004, fmt.Sprintf("cannot generate a new postgres role for schema:'%s'. error: %s", schemaName, err))
	}

	logger.Info(fmt.Sprintf("created role oid: %d for schema '%s'", roleOID, schemaName))

	composedUrl.User = url.UserPassword(roleName, password)

	config.SM_CREDENTIALS_ID = uint32ToString(roleOID)

	credentialsPayload := CredentialsPayload{
		CERTIFICATE_BASE64: pg.certificateBase64,
		USERNAME:           roleName,
		PASSWORD:           password,
		COMPOSED:           composedUrl.String(),
	}

	result, err := UpdateTaskAboutCredentialsCreated(client, config, credentialsPayload)
	if err != nil {
		var errBuilder strings.Builder
		errBuilder.WriteString(fmt.Sprintf("cannot update task: %s", err.Error()))
		err = deleteReadOnlyRole(pg.dbPool, roleOID, schemaName)
		if err != nil {
			errBuilder.WriteString(fmt.Sprintf("cannot undo the creation of role with id: '%s'. error: %s", config.SM_CREDENTIALS_ID, err.Error()))
		} else {
			errBuilder.WriteString(fmt.Sprintf("role with id: '%s' was deleted. ", config.SM_CREDENTIALS_ID))
		}
		logger.Error(errors.New(errBuilder.String()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: role with id: '%s' was created by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))
}

// deletePGCredentials deletes postgres credentials from the database.
func deletePGCredentials(client SecretsManagerClient, config *Config) {
	setDefaultValues(config)
	roleOID, err := stringToUint32(config.SM_CREDENTIALS_ID)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10022, fmt.Sprintf("cannot convert credentials id: '%s' to int: %s", config.SM_CREDENTIALS_ID, err.Error()))
	}

	pg, err := obtainPGAssembly(client, config)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10023, err.Error())
	}

	defer pg.dbPool.Close()
	schemaName := config.SM_SCHEMA_NAME
	err = deleteReadOnlyRole(pg.dbPool, roleOID, schemaName)
	if err != nil {
		updateTaskAboutErrorAndExit(client, config, Err10024, fmt.Sprintf("cannot delete postgres role for schema:'%s'. error: %s", schemaName, err))
	}

	result, err := UpdateTaskAboutCredentialsDeleted(client, config)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about credentials deleted with role id: '%s'. error: %s. ", config.SM_CREDENTIALS_ID, err.Error()))
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("task successfully updated: role id: '%s' was deleted by: %s ", config.SM_CREDENTIALS_ID, *result.UpdatedBy))
}

// createReadOnlyRole creates a read-only role with the specified name and password in the given schema.
// It returns the OID of the created role.
func createReadOnlyRole(pool *pgxpool.Pool, roleName, password, schemaName string) (uint32, error) {

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx) // safe to call even after commit
	}()

	createRoleQuery := fmt.Sprintf(
		"CREATE ROLE %s WITH LOGIN PASSWORD %s;",
		quoteIdentifier(roleName),
		quoteLiteral(password),
	)
	if _, err := tx.Exec(ctx, createRoleQuery); err != nil {
		return 0, fmt.Errorf("cannot create role with login password. error: %w", err)
	}

	var roleOID uint32
	if err := tx.QueryRow(ctx, "SELECT oid FROM pg_roles WHERE rolname = $1;", roleName).Scan(&roleOID); err != nil {
		return 0, fmt.Errorf("cannot retrieve role oid. error: %w", err)
	}

	grantUsageQuery := fmt.Sprintf(
		"GRANT USAGE ON SCHEMA %s TO %s;",
		quoteIdentifier(schemaName),
		quoteIdentifier(roleName),
	)

	if _, err := tx.Exec(ctx, grantUsageQuery); err != nil {
		return 0, fmt.Errorf("cannot grant role: '%d' usage on schema: '%s'. error: %w", roleOID, schemaName, err)
	}

	grantSelectQuery := fmt.Sprintf(
		"GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s;",
		quoteIdentifier(schemaName),
		quoteIdentifier(roleName),
	)

	if _, err := tx.Exec(ctx, grantSelectQuery); err != nil {
		return 0, fmt.Errorf("cannot grant role: '%d' select on all tables in schema '%s'. error: %w", roleOID, schemaName, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("cannot commit transaction: %w", err)
	}

	return roleOID, nil
}

// deleteReadOnlyRole deletes a role with the specified OID from the specified schema.
// It revokes all privileges on the schema and all tables in the schema from the role,
// and then drops the role if it exists.
func deleteReadOnlyRole(pool *pgxpool.Pool, roleOID uint32, schemaName string) error {
	// Use a context that could be passed from the caller if desired.
	ctx := context.Background()

	// Begin a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cannot begin transaction: %w", err)
	}
	// Ensure rollback if commit isn't reached.
	defer tx.Rollback(ctx)

	// Retrieve the role name using a parameterized query.
	var roleName string
	if err = tx.QueryRow(ctx, "SELECT rolname FROM pg_roles WHERE oid = $1;", roleOID).Scan(&roleName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			logger.Info(fmt.Sprintf("no operation required, role with oid '%d' not found", roleOID))
			return nil
		}
		return fmt.Errorf("error checking role with oid '%d' existence: %w", roleOID, err)
	}

	// Build the SQL queries using safe quoting for identifiers.
	revokeSQL := fmt.Sprintf(`
		REVOKE ALL PRIVILEGES ON SCHEMA %s FROM %s;
		REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s FROM %s;`,
		quoteIdentifier(schemaName), quoteIdentifier(roleName),
		quoteIdentifier(schemaName), quoteIdentifier(roleName),
	)
	if _, err = tx.Exec(ctx, revokeSQL); err != nil {
		return fmt.Errorf("cannot revoke privileges for role with oid: `%d`. error: %w", roleOID, err)
	}

	dropRoleSQL := fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(roleName))
	if _, err = tx.Exec(ctx, dropRoleSQL); err != nil {
		return fmt.Errorf("cannot drop role with oid: '%d'. error: %w", roleOID, err)
	}

	// Commit the transaction.
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("cannot commit transaction: %w", err)
	}

	logger.Info(fmt.Sprintf("Role with oid '%d' dropped successfully for schema '%s'", roleOID, schemaName))
	return nil
}

func quoteIdentifier(identifier string) string {
	// Escape double quotes and wrap the identifier in double quotes.
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func quoteLiteral(literal string) string {
	// Replace single quotes with two single quotes to escape them, then wrap in single quotes.
	return "'" + strings.ReplaceAll(literal, "'", "''") + "'"
}

func obtainPGAssembly(client SecretsManagerClient, config *Config) (*pgAssembly, error) {
	sc, err := fetchPGServiceCredentials(client, config)
	if err != nil {
		return nil, err
	}

	val, ok := GetValueByPath(sc, certificatePath)
	if !ok {
		return nil, fmt.Errorf("postgres certificate was not found in path: '%s'", certificatePath)
	}
	certificateBase64 := val.(string)
	certificate, err := base64.StdEncoding.DecodeString(certificateBase64)
	if err != nil {
		return nil, fmt.Errorf("postgres certificate decoding error: %s", err)
	}

	composed, ok := GetValueByPath(sc, composedPath)
	if !ok {
		return nil, fmt.Errorf("postgres composed was not found in path: '%s'", composedPath)
	}

	dbPool, err := connectToPostgres(composed.(string), certificate)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to postgres. error: %s", err)
	}
	return &pgAssembly{
		dbPool:            dbPool,
		certificate:       certificate,
		certificateBase64: certificateBase64,
		compose:           composed.(string),
	}, nil
}

// connectToPostgres establishes a connection to a PostgreSQL database using a provided connection string
// and a TLS certificate file for secure communication.
func connectToPostgres(connStr string, certificate []byte) (*pgxpool.Pool, error) {

	// Create a certificate pool and add the certificate
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(certificate) {
		return nil, fmt.Errorf("cannot connect to postgres, failed to append certificate to pool")
	}

	// Configure the connection pool with the TLS settings
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to postgres, failed to parse connection string: %w", err)
	}

	// Assign the custom certificate pool
	config.ConnConfig.TLSConfig.RootCAs = rootCAs

	// Create a connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("cannot create postgres connection pool: %w", err)
	}

	return pool, nil
}

func fetchPGServiceCredentials(client SecretsManagerClient, config *Config) (sc map[string]interface{}, err error) {

	secret, err := GetSecret(client, config.SM_LOGIN_SECRET_ID)
	if err != nil {
		if strings.Contains(err.Error(), "Provided API key could not be found") {
			logger.Error(fmt.Errorf("cannot call the secrets manager service: %v", err))
			os.Exit(1)
		}
		return nil, err
	}
	serviceCredentialsSecret, ok := secret.(*sm.ServiceCredentialsSecret)
	if !ok {
		return nil, fmt.Errorf("get secret id: '%s' returned unexpected secret type: %T, expected service credentials type", config.SM_LOGIN_SECRET_ID, secret)
	}

	return serviceCredentialsSecret.Credentials.GetProperties(), nil
}

func generateRoleName() string {
	newUUID := uuid.New()
	return fmt.Sprintf("secrets_manager_%s", strings.ReplaceAll(newUUID.String(), "-", "_"))
}

const passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!$-_*"

// creates a secure random password of given length.
func generateRolePassword(length int) (string, error) {
	if length < 12 { // Ensure minimum password length for security
		return "", fmt.Errorf("password length must be at least 12 characters")
	}

	password := make([]byte, length)
	for i := range password {
		randIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordChars))))
		if err != nil {
			return "", err
		}
		password[i] = passwordChars[randIndex.Int64()]
	}

	return string(password), nil
}

func updateTaskAboutErrorAndExit(client SecretsManagerClient, config *Config, code, description string) {
	result, err := UpdateTaskAboutError(client, config, code, description)
	if err != nil {
		logger.Error(fmt.Errorf("cannot update task about error with code: '%s' and description: '%s'. returned error: %w", code, description, err))
	} else {
		logger.Info(fmt.Sprintf("task was updated about error with code: '%s' and description: '%s' by: %s", code, description, *result.UpdatedBy))
	}
	os.Exit(1)
}

// setDefaultValues sets default values for non required config variables if not set by the user
func setDefaultValues(config *Config) {
	// optional env variable, use postgres default 'public' schema as default
	if config.SM_SCHEMA_NAME == "" {
		config.SM_SCHEMA_NAME = "public"
	}
}

// Uint32ToString converts a uint32 to a string.
func uint32ToString(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}

// StringToUint32 converts a string to a uint32.
func stringToUint32(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}
