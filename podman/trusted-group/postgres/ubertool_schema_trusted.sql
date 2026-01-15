-- Trusted Ubertool Database Schema
-- Compatible with PostgreSQL 15+

-- 0. Global Types & Enums
CREATE TYPE user_org_status_enum AS ENUM ('ACTIVE', 'SUSPEND', 'BLOCK');
CREATE TYPE user_org_role_enum AS ENUM ('SUPER_ADMIN', 'ADMIN', 'MEMBER');
CREATE TYPE tool_duration_unit_enum AS ENUM ('day', 'week', 'month');
CREATE TYPE tool_status_enum AS ENUM ('AVAILABLE', 'UNAVAILABLE', 'RENTED');
CREATE TYPE tool_condition_enum AS ENUM ('EXCELLENT', 'GOOD', 'ACCEPTABLE', 'DAMAGED/NEEDS_REPAIR');
CREATE TYPE ledger_transaction_type_enum AS ENUM ('RENTAL_DEBIT', 'LENDING_CREDIT', 'REFUND', 'ADJUSTMENT');
CREATE TYPE rental_status_enum AS ENUM ('PENDING', 'APPROVED', 'SCHEDULED', 'ACTIVE', 'COMPLETED', 'CANCELLED', 'OVERDUE');

-- 1. Organizations (Community/Church Groups)
CREATE TABLE orgs (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
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
    balance_credits INTEGER DEFAULT 0,
    status user_org_status_enum NOT NULL DEFAULT 'ACTIVE',
    role user_org_role_enum NOT NULL DEFAULT 'MEMBER',
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE invitations (
    token UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- Tokens should still be random UUIDs for security
    org_id INTEGER REFERENCES orgs(id),
    email TEXT NOT NULL,
    created_by INTEGER REFERENCES users(id),
    expires_on DATE NOT NULL,
    used_on DATE, -- NULL if unused
    created_on DATE DEFAULT CURRENT_DATE
);

CREATE TABLE join_requests (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
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
    info TEXT, -- Description/Details
    category TEXT NOT NULL,
    daily_rental_price INTEGER NOT NULL DEFAULT 0,
    weekly_rental_price INTEGER NOT NULL DEFAULT 0,
    monthly_rental_price INTEGER NOT NULL DEFAULT 0,
    replacement_price INTEGER,
    duration_unit tool_duration_unit_enum NOT NULL DEFAULT 'day',
    condition tool_condition_enum NOT NULL DEFAULT 'GOOD',
    metro TEXT, -- Optional location indicator
    location_string TEXT, 
    status tool_status_enum NOT NULL DEFAULT 'AVAILABLE',
    created_on DATE DEFAULT CURRENT_DATE,
    deleted_on DATE
);

CREATE TABLE tool_images (
    id SERIAL PRIMARY KEY,
    tool_id INTEGER REFERENCES tools(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    display_order INTEGER DEFAULT 0
);

-- 4. Rentals
CREATE TABLE rentals (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    tool_id INTEGER REFERENCES tools(id),
    renter_id INTEGER REFERENCES users(id),
    owner_id INTEGER REFERENCES users(id),
    start_date DATE NOT NULL,
    scheduled_end_date DATE NOT NULL, -- Expected return
    end_date DATE, -- Actual return date
    total_cost_credits INTEGER NOT NULL,
    status rental_status_enum NOT NULL DEFAULT 'PENDING',
    pickup_note TEXT,
    created_on DATE DEFAULT CURRENT_DATE,
    updated_on DATE DEFAULT CURRENT_DATE
);

-- 5. Ledger
CREATE TABLE ledger_transactions (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    user_id INTEGER REFERENCES users(id),
    amount INTEGER NOT NULL,
    type ledger_transaction_type_enum NOT NULL,
    related_rental_id INTEGER REFERENCES rentals(id),
    description TEXT,
    charged_on DATE DEFAULT CURRENT_DATE,
    created_on DATE DEFAULT CURRENT_DATE
);

-- Function to update balance on insert
CREATE OR REPLACE FUNCTION update_user_balance() RETURNS TRIGGER AS $$
BEGIN
    UPDATE users_orgs SET balance_credits = balance_credits + NEW.amount WHERE user_id = NEW.user_id AND org_id = NEW.org_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_balance
AFTER INSERT ON ledger_transactions
FOR EACH ROW
EXECUTE FUNCTION update_user_balance();
