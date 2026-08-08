-- PostgreSQL DDL Script for Multiplayer AI System Database

-- Enable UUID extension if not enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Custom Enum Types
CREATE TYPE message_role AS ENUM ('system', 'user', 'assistant', 'tool');
CREATE TYPE tool_call_status AS ENUM ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'FAILED');
CREATE TYPE patch_status AS ENUM ('PENDING', 'APPLIED', 'REJECTED', 'FAILED');
CREATE TYPE session_status AS ENUM ('ACTIVE', 'ARCHIVED', 'TERMINATED');
CREATE TYPE member_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER', 'VIEWER');

-- 1. Users Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP WITH TIME ZONE
);

-- 2. Sessions Table
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_storage_path TEXT NOT NULL,
    current_version BIGINT NOT NULL DEFAULT 1,
    status session_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_active_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Session Members Table
CREATE TABLE IF NOT EXISTS session_members (
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role member_role NOT NULL DEFAULT 'MEMBER',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (session_id, user_id)
);

-- 4. Snapshots Table
CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    version BIGINT NOT NULL,
    storage_location TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 5. AIContext Table
CREATE TABLE IF NOT EXISTS ai_contexts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    provider VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    system_prompt TEXT,
    context_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 6. AIMessage Table
CREATE TABLE IF NOT EXISTS ai_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    sequence_number BIGINT NOT NULL,
    role message_role NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 7. AIToolCall Table
CREATE TABLE IF NOT EXISTS ai_tool_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    message_id UUID REFERENCES ai_messages(id) ON DELETE SET NULL,
    tool_name VARCHAR(100) NOT NULL,
    arguments JSONB,
    result JSONB,
    status tool_call_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- 8. AIPatch Table
CREATE TABLE IF NOT EXISTS ai_patches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    message_id UUID REFERENCES ai_messages(id) ON DELETE SET NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status patch_status NOT NULL DEFAULT 'PENDING',
    patch_content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_sessions_owner ON sessions(owner_id);
CREATE INDEX IF NOT EXISTS idx_session_members_user ON session_members(user_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_session ON snapshots(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_contexts_session ON ai_contexts(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_messages_context_seq ON ai_messages(ai_context_id, sequence_number);
CREATE INDEX IF NOT EXISTS idx_ai_tool_calls_context ON ai_tool_calls(ai_context_id);
CREATE INDEX IF NOT EXISTS idx_ai_tool_calls_message ON ai_tool_calls(message_id);
CREATE INDEX IF NOT EXISTS idx_ai_patches_context ON ai_patches(ai_context_id);
CREATE INDEX IF NOT EXISTS idx_ai_patches_message ON ai_patches(message_id);
