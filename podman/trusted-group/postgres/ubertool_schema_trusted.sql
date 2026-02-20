-- Trusted Ubertool Database Schema
-- Compatible with PostgreSQL 15+

-- 0. Global Types & Enums
-- CREATE TYPE join_request_status_enum AS ENUM ('PENDING', 'INVITED', 'JOINED', 'REJECTED');
-- CREATE TYPE user_org_status_enum AS ENUM ('ACTIVE', 'SUSPEND', 'BLOCK');
-- CREATE TYPE user_org_role_enum AS ENUM ('SUPER_ADMIN', 'ADMIN', 'MEMBER');
-- CREATE TYPE tool_duration_unit_enum AS ENUM ('day', 'week', 'month');
-- CREATE TYPE tool_status_enum AS ENUM ('AVAILABLE', 'UNAVAILABLE', 'RENTED');
-- CREATE TYPE tool_condition_enum AS ENUM ('EXCELLENT', 'GOOD', 'ACCEPTABLE', 'DAMAGED/NEEDS_REPAIR');
-- CREATE TYPE ledger_transaction_type_enum AS ENUM ('RENTAL_DEBIT', 'LENDING_CREDIT', 'REFUND', 'ADJUSTMENT');
-- CREATE TYPE rental_status_enum AS ENUM ('PENDING', 'APPROVED', 'REJECTED', 'SCHEDULED', 'ACTIVE', 'COMPLETED', 'CANCELLED', 'OVERDUE', 'RETURN_DATE_CHANGED', 'RETURN_DATE_CHANGE_REJECTED');

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
    renting_blocked BOOLEAN DEFAULT FALSE,
    lending_blocked BOOLEAN DEFAULT FALSE,
    blocked_due_to_bill_id INTEGER, -- FK constrain added after bills table creation
    blocked_reason TEXT,
    blocked_on Date,
    PRIMARY KEY (user_id, org_id)
);

CREATE INDEX idx_users_orgs_renting_blocked ON users_orgs(user_id, org_id) WHERE renting_blocked = TRUE;
CREATE INDEX idx_users_orgs_lending_blocked ON users_orgs(user_id, org_id) WHERE lending_blocked = TRUE;

CREATE TABLE join_requests (
    id SERIAL PRIMARY KEY,
    org_id INTEGER REFERENCES orgs(id),
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    note TEXT,
    status TEXT DEFAULT 'PENDING',
    reason TEXT,
    rejected_by_user_id INTEGER REFERENCES users(id), -- Admin who rejected the request
    created_on DATE DEFAULT CURRENT_DATE
);

CREATE TABLE invitations (
    id SERIAL PRIMARY KEY,
    invitation_code TEXT NOT NULL,
    org_id INTEGER REFERENCES orgs(id),
    email TEXT NOT NULL,
    join_request_id INTEGER REFERENCES join_requests(id), -- Optional link to a join request
    created_by INTEGER REFERENCES users(id),
    expires_on DATE NOT NULL,
    used_on DATE, -- NULL if unused
    used_by_user_id INTEGER REFERENCES users(id), -- User who used the invitation
    created_on DATE DEFAULT CURRENT_DATE,
    UNIQUE(invitation_code, email) -- Ensure uniqueness of invitation tuple
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
    replacement_cost_cents INTEGER NOT NULL DEFAULT 0,
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
    last_agreed_end_date DATE, -- Last agreed return date (agreed by both renter and owner,can be updated with return date change flow)
    end_date DATE NOT NULL, -- Actual return date
    duration_unit TEXT NOT NULL DEFAULT 'day',
    daily_price_cents INTEGER NOT NULL,
    weekly_price_cents INTEGER NOT NULL,
    monthly_price_cents INTEGER NOT NULL,
    replacement_cost_cents INTEGER NOT NULL,
    total_cost_cents INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    pickup_note TEXT,
    rejection_reason TEXT,
    completed_by INTEGER,
    return_condition TEXT,
    return_note TEXT,
    surcharge_or_credit_cents INTEGER, -- For late return or damage fees or credits for early return
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

-- 7. Bill Splitting & Dispute Resolution

-- Captures user account balance snapshot before bill splitting calculation
CREATE TABLE balance_snapshots (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id INTEGER NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    balance_cents INTEGER NOT NULL,
    settlement_month TEXT NOT NULL, -- Format: 'YYYY-MM' (e.g., '2026-01')
    snapshot_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_on TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, org_id, settlement_month)
);

CREATE INDEX idx_balance_snapshots_user_org ON balance_snapshots(user_id, org_id);
CREATE INDEX idx_balance_snapshots_settlement ON balance_snapshots(settlement_month);
CREATE INDEX idx_balance_snapshots_org_settlement ON balance_snapshots(org_id, settlement_month);

-- Bills table: result of bill splitting calculation (who should pay whom how much)
CREATE TABLE bills (
    id SERIAL PRIMARY KEY,
    org_id INTEGER NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    debtor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creditor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    settlement_month TEXT NOT NULL, -- Format: 'YYYY-MM' (e.g., '2026-01')
    
    -- Bill status: PENDING -> PAID (or -> DISPUTED -> ADMIN_RESOLVED/SYSTEM_DEFAULT_ACTION)
    status TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, PAID, DISPUTED, ADMIN_RESOLVED, SYSTEM_DEFAULT_ACTION
    
    -- Timestamps for tracking state transitions (denormalized for quick queries)
    notice_sent_at TIMESTAMP,
    debtor_acknowledged_at TIMESTAMP,
    creditor_acknowledged_at TIMESTAMP,
    disputed_at TIMESTAMP,
    resolved_at TIMESTAMP,
    
    -- Dispute tracking
    dispute_reason TEXT, -- DEBTOR_NO_ACK, CREDITOR_NO_ACK
    resolution_outcome TEXT, -- GRACEFUL, DEBTOR_FAULT, CREDITOR_FAULT, BOTH_FAULT
    resolution_notes TEXT,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    CHECK (debtor_user_id != creditor_user_id),
    UNIQUE(org_id, debtor_user_id, creditor_user_id, settlement_month)
);

CREATE INDEX idx_bills_org_settlement ON bills(org_id, settlement_month);
CREATE INDEX idx_bills_debtor_status ON bills(debtor_user_id, status);
CREATE INDEX idx_bills_creditor_status ON bills(creditor_user_id, status);
CREATE INDEX idx_bills_status ON bills(status);
CREATE INDEX idx_bills_settlement ON bills(settlement_month);
CREATE INDEX idx_bills_notice_sent ON bills(notice_sent_at) WHERE status = 'PENDING';
CREATE INDEX idx_bills_disputed ON bills(disputed_at) WHERE status = 'DISPUTED';

-- Add FK constraint in users_orgs now
ALTER TABLE users_orgs 
    ADD CONSTRAINT fk_blocked_bill 
    FOREIGN KEY (blocked_due_to_bill_id) 
    REFERENCES bills(id);

-- Bill actions: audit log for all debtor/creditor acknowledgments, admin resolutions, system actions
CREATE TABLE bill_actions (
    id SERIAL PRIMARY KEY,
    bill_id INTEGER NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    actor_user_id INTEGER REFERENCES users(id), -- NULL for system actions
    action_type TEXT NOT NULL, -- NOTICE_SENT, DEBTOR_ACKNOWLEDGED, CREDITOR_ACKNOWLEDGED, 
                                -- DISPUTE_OPENED, ADMIN_COMMENT, ADMIN_RESOLUTION, SYSTEM_AUTO_RESOLVE
    action_details JSONB, -- Flexible storage for action metadata
    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_bill_actions_bill ON bill_actions(bill_id);
CREATE INDEX idx_bill_actions_actor ON bill_actions(actor_user_id);
CREATE INDEX idx_bill_actions_type ON bill_actions(action_type);
CREATE INDEX idx_bill_actions_created ON bill_actions(created_at);

-- Function to automatically initiate disputes after 10 days
CREATE OR REPLACE FUNCTION check_overdue_bills() RETURNS void AS $$
BEGIN
    -- Identify bills that are overdue (10+ days) and not yet disputed
    UPDATE bills
    SET status = 'DISPUTED',
        disputed_at = NOW(),
        dispute_reason = CASE 
            WHEN debtor_acknowledged_at IS NULL THEN 'DEBTOR_NO_ACK'
            WHEN creditor_acknowledged_at IS NULL THEN 'CREDITOR_NO_ACK'
        END,
        updated_at = NOW()
    WHERE status = 'PENDING' 
        AND notice_sent_at IS NOT NULL
        AND notice_sent_at < NOW() - INTERVAL '10 days'
        AND disputed_at IS NULL;
    
    -- Create bill actions for newly disputed bills
    INSERT INTO bill_actions (bill_id, actor_user_id, action_type, notes)
    SELECT 
        id,
        NULL,
        'DISPUTE_OPENED',
        'Automatically opened dispute after 10 days without resolution'
    FROM bills
    WHERE disputed_at >= NOW() - INTERVAL '1 minute'
        AND status = 'DISPUTED';
END;
$$ LANGUAGE plpgsql;

-- Function to auto-resolve disputed bills at end of month (blocks both parties by default)
CREATE OR REPLACE FUNCTION auto_resolve_disputed_bills(p_org_id INTEGER, p_settlement_month TEXT) RETURNS void AS $$
DECLARE
    bill_record RECORD;
BEGIN
    -- Find all disputed bills for the organization and settlement month
    FOR bill_record IN 
        SELECT id, debtor_user_id, creditor_user_id, amount_cents
        FROM bills
        WHERE org_id = p_org_id 
            AND settlement_month = p_settlement_month
            AND status = 'DISPUTED'
    LOOP
        -- Update bill to system default action with both parties at fault
        UPDATE bills
        SET status = 'SYSTEM_DEFAULT_ACTION',
            resolution_outcome = 'BOTH_FAULT',
            resolution_notes = 'Auto-resolved by system at end of month - both parties blocked',
            resolved_at = NOW(),
            updated_at = NOW()
        WHERE id = bill_record.id;
        
        -- Block debtor from renting
        UPDATE users_orgs
        SET renting_blocked = TRUE,
            blocked_due_to_bill_id = bill_record.id,
            blocked_reason = 'Blocked due to unresolved payment dispute'
        WHERE user_id = bill_record.debtor_user_id 
            AND org_id = p_org_id;
        
        -- Block creditor from lending
        UPDATE users_orgs
        SET lending_blocked = TRUE,
            blocked_due_to_bill_id = bill_record.id,
            blocked_reason = 'Blocked due to unresolved payment dispute'
        WHERE user_id = bill_record.creditor_user_id 
            AND org_id = p_org_id;
        
        -- Create bill action for system auto-resolve
        INSERT INTO bill_actions (bill_id, actor_user_id, action_type, action_details, notes)
        VALUES (
            bill_record.id,
            NULL,
            'SYSTEM_AUTO_RESOLVE',
            jsonb_build_object(
                'resolution_outcome', 'BOTH_FAULT',
                'debtor_blocked', TRUE,
                'creditor_blocked', TRUE
            ),
            'Auto-resolved by system - both parties blocked from renting/lending'
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;