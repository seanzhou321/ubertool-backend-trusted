-- Trusted Ubertool Database Schema
-- Compatible with PostgreSQL 15+

-- 0. Global Types & Enums
-- CREATE TYPE user_org_status_enum AS ENUM ('ACTIVE', 'SUSPEND', 'BLOCK');
-- CREATE TYPE user_org_role_enum AS ENUM ('SUPER_ADMIN', 'ADMIN', 'MEMBER');
-- CREATE TYPE tool_duration_unit_enum AS ENUM ('day', 'week', 'month');
-- CREATE TYPE tool_status_enum AS ENUM ('AVAILABLE', 'UNAVAILABLE', 'RENTED');
-- CREATE TYPE tool_condition_enum AS ENUM ('EXCELLENT', 'GOOD', 'ACCEPTABLE', 'DAMAGED/NEEDS_REPAIR');
-- CREATE TYPE ledger_transaction_type_enum AS ENUM ('RENTAL_DEBIT', 'LENDING_CREDIT', 'REFUND', 'ADJUSTMENT');
-- CREATE TYPE rental_status_enum AS ENUM ('PENDING', 'APPROVED', 'REJECTED', 'SCHEDULED', 'ACTIVE', 'COMPLETED', 'CANCELLED', 'OVERDUE');

-- 1. Organizations (Community/Church Groups)
CREATE TABLE orgs (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    address TEXT,
    metro TEXT,
    admin_phone_number TEXT,
    admin_email TEXT,
    created_on DATE DEFAULT CURRENT_DATE
);

-- 2. Users & Auth
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    phone_number TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    avatar_url TEXT,
    created_on DATE DEFAULT CURRENT_DATE,
    updated_on DATE DEFAULT CURRENT_DATE
);

-- Join table for Many-to-Many (Users <-> Orgs)
CREATE TABLE users_orgs (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    org_id INTEGER REFERENCES orgs(id) ON DELETE CASCADE,
    joined_on DATE DEFAULT CURRENT_DATE,
    balance_cents INTEGER DEFAULT 0,
    last_balance_updated_on DATE,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    role TEXT NOT NULL DEFAULT 'MEMBER',
    blocked_date DATE,
    block_reason TEXT,
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE invitations (
    id SERIAL PRIMARY KEY,
    invitation_code TEXT NOT NULL,
    org_id INTEGER REFERENCES orgs(id),
    email TEXT NOT NULL,
    created_by INTEGER REFERENCES users(id),
    expires_on DATE NOT NULL,
    used_on DATE, -- NULL if unused
    used_by_user_id INTEGER REFERENCES users(id), -- User who used the invitation
    created_on DATE DEFAULT CURRENT_DATE,
    UNIQUE(invitation_code, email) -- Ensure uniqueness of invitation tuple
);

CREATE TABLE join_requests (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    note TEXT,
    status TEXT DEFAULT 'PENDING',
    created_on DATE DEFAULT CURRENT_DATE
);

-- 3. Tools
CREATE TABLE tools (
    id SERIAL PRIMARY KEY,
    owner_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT, -- Description/Details
    categories TEXT[], -- Array of categories
    price_per_day_cents INTEGER NOT NULL DEFAULT 0,
    price_per_week_cents INTEGER NOT NULL DEFAULT 0,
    price_per_month_cents INTEGER NOT NULL DEFAULT 0,
    replacement_cost_cents INTEGER,
    duration_unit TEXT NOT NULL DEFAULT 'day',
    condition TEXT NOT NULL DEFAULT 'GOOD',
    metro TEXT, -- Optional location indicator
    status TEXT NOT NULL DEFAULT 'AVAILABLE',
    created_on DATE DEFAULT CURRENT_DATE,
    deleted_on DATE
);

-- Unified table for both pending and confirmed tool images
CREATE TABLE tool_images (
    id SERIAL PRIMARY KEY,
    tool_id INTEGER NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    thumbnail_path TEXT,
    file_size INTEGER,
    mime_type TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, CONFIRMED, DELETED
    expires_at TIMESTAMP,                 -- For pending images
    created_on TIMESTAMP DEFAULT NOW(),
    confirmed_on TIMESTAMP,
    deleted_on TIMESTAMP
);

-- Unique constraint: only one primary image per confirmed tool
CREATE UNIQUE INDEX idx_tool_images_primary_unique ON tool_images(tool_id) 
    WHERE is_primary = TRUE AND status = 'CONFIRMED';

-- Index for fast queries
CREATE INDEX idx_tool_images_tool_id ON tool_images(tool_id) WHERE tool_id IS NOT NULL;
CREATE INDEX idx_tool_images_status ON tool_images(status);
CREATE INDEX idx_tool_images_user_pending ON tool_images(user_id, status) WHERE status = 'PENDING';
CREATE INDEX idx_tool_images_expires ON tool_images(expires_at) WHERE status = 'PENDING';

-- 4. Rentals
CREATE TABLE rentals (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    tool_id INTEGER REFERENCES tools(id),
    renter_id INTEGER REFERENCES users(id),
    owner_id INTEGER REFERENCES users(id),
    start_date DATE NOT NULL,
    scheduled_end_date DATE NOT NULL, -- Scheduled return date
    end_date DATE, -- Actual return date
    total_cost_cents INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    pickup_note TEXT,
    completed_by INTEGER,
    created_on DATE DEFAULT CURRENT_DATE,
    updated_on DATE DEFAULT CURRENT_DATE
);

-- 5. Ledger
CREATE TABLE ledger_transactions (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    user_id INTEGER REFERENCES users(id),
    amount INTEGER NOT NULL,
    type TEXT NOT NULL,
    related_rental_id INTEGER REFERENCES rentals(id), -- Nullable, immutable record
    description TEXT,
    charged_on DATE DEFAULT CURRENT_DATE,
    created_on DATE DEFAULT CURRENT_DATE
);

-- 6. Notifications
CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    org_id INTEGER REFERENCES orgs(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    attributes JSONB, -- For metadata map
    created_on DATE DEFAULT CURRENT_DATE
);

-- Function to update balance on insert
CREATE OR REPLACE FUNCTION update_user_balance() RETURNS TRIGGER AS $$
BEGIN
    UPDATE users_orgs SET balance_cents = balance_cents + NEW.amount WHERE user_id = NEW.user_id AND org_id = NEW.org_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_balance
AFTER INSERT ON ledger_transactions
FOR EACH ROW
EXECUTE FUNCTION update_user_balance();
