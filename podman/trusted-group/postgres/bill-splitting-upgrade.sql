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
CREATE INDEX idx_bill_actions_created ON bill_actions(created_on);

-- Add bill-related blocking fields to users_orgs for tracking rental/lending restrictions
-- Note: These columns track blocking due to bill disputes
ALTER TABLE users_orgs 
    ADD COLUMN IF NOT EXISTS renting_blocked BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS lending_blocked BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS blocked_due_to_bill_id INTEGER REFERENCES bills(id),
    ADD COLUMN IF NOT EXISTS bill_block_reason TEXT;

CREATE INDEX idx_users_orgs_renting_blocked ON users_orgs(user_id, org_id) WHERE renting_blocked = TRUE;
CREATE INDEX idx_users_orgs_lending_blocked ON users_orgs(user_id, org_id) WHERE lending_blocked = TRUE;

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
        updated_on = NOW()
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
            updated_on = NOW()
        WHERE id = bill_record.id;
        
        -- Block debtor from renting
        UPDATE users_orgs
        SET renting_blocked = TRUE,
            blocked_due_to_bill_id = bill_record.id,
            bill_block_reason = 'Blocked due to unresolved payment dispute'
        WHERE user_id = bill_record.debtor_user_id 
            AND org_id = p_org_id;
        
        -- Block creditor from lending
        UPDATE users_orgs
        SET lending_blocked = TRUE,
            blocked_due_to_bill_id = bill_record.id,
            bill_block_reason = 'Blocked due to unresolved payment dispute'
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