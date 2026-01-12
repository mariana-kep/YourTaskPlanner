CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    owner_id BIGINT NOT NULL,
    assigned_to BIGINT,
    shared BOOLEAN DEFAULT FALSE,
    due_at TIMESTAMP WITH TIME ZONE,
    remind_at TIMESTAMP WITH TIME ZONE,
    reminder_scheduled BOOLEAN DEFAULT FALSE,
    reminder_sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);