-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create the application_logs table
CREATE TABLE IF NOT EXISTS application_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_application_logs_timestamp ON application_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_application_logs_severity ON application_logs(severity);
CREATE INDEX IF NOT EXISTS idx_application_logs_operation ON application_logs(operation);
CREATE INDEX IF NOT EXISTS idx_application_logs_process_id ON application_logs(process_id);
CREATE INDEX IF NOT EXISTS idx_application_logs_process_type ON application_logs(process_type);
CREATE INDEX IF NOT EXISTS idx_application_logs_target_name ON application_logs(target_name);
CREATE INDEX IF NOT EXISTS idx_application_logs_client_ip ON application_logs(client_ip);

-- Create a view for log analysis
CREATE OR REPLACE VIEW log_summary AS
SELECT 
    operation,
    process_type,
    severity,
    COUNT(*) as count,
    DATE_TRUNC('hour', timestamp) as hour
FROM application_logs 
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY operation, process_type, severity, DATE_TRUNC('hour', timestamp)
ORDER BY hour DESC, count DESC;

-- Create a view for process tracking
CREATE OR REPLACE VIEW process_summary AS
SELECT 
    process_id,
    process_type,
    MIN(timestamp) as process_start,
    MAX(timestamp) as process_end,
    EXTRACT(EPOCH FROM (MAX(timestamp) - MIN(timestamp))) * 1000 as duration_ms,
    COUNT(*) as log_count,
    COUNT(CASE WHEN severity IS NOT NULL THEN 1 END) as error_count
FROM application_logs 
WHERE timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY process_id, process_type
ORDER BY process_start DESC;