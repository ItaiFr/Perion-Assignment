package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"Perion_Assignment/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupabaseConnection implements DatabaseConnection for Supabase PostgreSQL using pgxpool
type SupabaseConnection struct {
	pool *pgxpool.Pool
}

// NewSupabaseConnection creates a new Supabase database connection using connection string
func NewSupabaseConnection(connectionString string) (DatabaseConnection, error) {
	return newSupabaseConnection(connectionString)
}

// newSupabaseConnection creates the concrete implementation
func newSupabaseConnection(connectionString string) (*SupabaseConnection, error) {
	// Parse and create pool configuration
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse supabase connection string: %w", err)
	}

	// אופטימיזציה לחיבורי ענן (Supabase)
	config.MaxConns = 10
	config.MinConns = 2

	// חשוב: אל תשאיר על 0 בחיבורי ענן. זה עוזר לרענן חיבורים שהתנתקו שקטה ע"י הרשת.
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	
	// Disable statement caching to avoid "already exists" errors
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.StatementCacheCapacity = 0

	// הגדרת Dialer גמיש שתומך ב-IPv6 וגם ב-IPv4
	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		// משתמשים בברירת המחדל (tcp) שמאפשרת IPv6 אם זמין
		return d.DialContext(ctx, "tcp", addr)
	}

	// הדפסת פרטי החיבור לצרכי דיבאג (ללא סיסמה)
	fmt.Printf("Attempting to connect to host: %s on port: %d\n", config.ConnConfig.Host, config.ConnConfig.Port)

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create supabase connection pool: %w", err)
	}

	// בדיקת חיות החיבור עם Context מוגבל בזמן
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed. check if project is paused or network blocks port %d: %w", config.ConnConfig.Port, err)
	}

	conn := &SupabaseConnection{pool: pool}
	if err := conn.createTableIfNotExists(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create logs table: %w", err)
	}

	return conn, nil
}

// createTableIfNotExists creates the logs table if it doesn't exist
func (s *SupabaseConnection) createTableIfNotExists() error {
	query := `
		CREATE TABLE IF NOT EXISTS application_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			severity VARCHAR(10) CHECK (severity IN ('low', 'medium', 'high')),
			message TEXT NOT NULL,
			operation VARCHAR(100) NOT NULL,
			target_name VARCHAR(255),
			process_id UUID NOT NULL,
			process_type VARCHAR(20) NOT NULL CHECK (process_type IN ('request', 'internal')),
			client_ip INET,
			error_details TEXT,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		-- Indexes for better query performance
		CREATE INDEX IF NOT EXISTS idx_application_logs_timestamp ON application_logs(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_application_logs_severity ON application_logs(severity) WHERE severity IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_application_logs_operation ON application_logs(operation);
		CREATE INDEX IF NOT EXISTS idx_application_logs_process_id ON application_logs(process_id);
		CREATE INDEX IF NOT EXISTS idx_application_logs_process_type ON application_logs(process_type);
		CREATE INDEX IF NOT EXISTS idx_application_logs_created_at ON application_logs(created_at DESC);
	`

	_, err := s.pool.Exec(context.Background(), query)
	return err
}

// InsertLog inserts a log entry into the Supabase database
func (s *SupabaseConnection) InsertLog(ctx context.Context, entry *models.LogEntry) error {
	query := `
		INSERT INTO application_logs 
		(id, timestamp, severity, message, operation, target_name, process_id, process_type, client_ip, error_details, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	var clientIP interface{}
	if entry.ClientIP != "" {
		clientIP = entry.ClientIP
	}

	var targetNameVal interface{}
	if entry.TargetName != "" {
		targetNameVal = entry.TargetName
	}

	var errorDetails interface{}
	if entry.Error != "" {
		errorDetails = entry.Error
	}

	// Convert metadata to JSON string for JSONB if present
	var metadata interface{}
	if len(entry.Metadata) > 0 {
		// Marshal to JSON string for proper JSONB encoding
		jsonBytes, err := json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata to JSON: %w", err)
		}
		metadata = string(jsonBytes)
	}

	// Handle empty severity (for info/success logs)
	var severityVal interface{}
	if entry.Severity != "" {
		severityVal = string(entry.Severity)
	}

	_, err := s.pool.Exec(
		ctx, query,
		entry.ID,
		entry.Timestamp,
		severityVal,
		entry.Message,
		entry.Operation,
		targetNameVal,
		entry.ProcessID,
		string(entry.ProcessType),
		clientIP,
		errorDetails,
		metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to insert log entry to supabase: %w", err)
	}

	return nil
}

// Ping checks if the Supabase database connection is alive
func (s *SupabaseConnection) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes the Supabase database connection
func (s *SupabaseConnection) Close() error {
	s.pool.Close()
	return nil
}
