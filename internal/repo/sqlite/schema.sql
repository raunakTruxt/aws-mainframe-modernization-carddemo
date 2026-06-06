-- CardDemo relational schema (ADR 0004).
-- VSAM KSDS files map to one table each. Money and rate fields are stored as
-- TEXT (shopspring/decimal serialises to/from a decimal string); int64 keys are
-- INTEGER. Alternate VSAM indexes (AIX) become secondary indexes here.

CREATE TABLE IF NOT EXISTS accounts (
    acct_id                INTEGER PRIMARY KEY,
    acct_active_status     TEXT NOT NULL,
    acct_curr_bal          TEXT NOT NULL,
    acct_credit_limit      TEXT NOT NULL,
    acct_cash_credit_limit TEXT NOT NULL,
    acct_open_date         TEXT NOT NULL,
    acct_expiration_date   TEXT NOT NULL,
    acct_reissue_date      TEXT NOT NULL,
    acct_curr_cyc_credit   TEXT NOT NULL,
    acct_curr_cyc_debit    TEXT NOT NULL,
    acct_addr_zip          TEXT NOT NULL,
    acct_group_id          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cards (
    card_num             TEXT PRIMARY KEY,
    card_acct_id         INTEGER NOT NULL,
    card_cvv_code        INTEGER NOT NULL,
    card_embossed_name   TEXT NOT NULL,
    card_expiration_date TEXT NOT NULL,
    card_active_status   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cards_acct_id ON cards (card_acct_id);

CREATE TABLE IF NOT EXISTS customers (
    cust_id                  INTEGER PRIMARY KEY,
    cust_first_name          TEXT NOT NULL,
    cust_middle_name         TEXT NOT NULL,
    cust_last_name           TEXT NOT NULL,
    cust_addr_line_1         TEXT NOT NULL,
    cust_addr_line_2         TEXT NOT NULL,
    cust_addr_line_3         TEXT NOT NULL,
    cust_addr_state_code     TEXT NOT NULL,
    cust_addr_country_code   TEXT NOT NULL,
    cust_addr_zip            TEXT NOT NULL,
    cust_phone_num_1         TEXT NOT NULL,
    cust_phone_num_2         TEXT NOT NULL,
    cust_ssn                 INTEGER NOT NULL,
    cust_govt_issued_id      TEXT NOT NULL,
    cust_dob                 TEXT NOT NULL,
    cust_eft_account_id      TEXT NOT NULL,
    cust_pri_card_holder_ind TEXT NOT NULL,
    cust_fico_credit_score   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS card_xrefs (
    xref_card_num TEXT PRIMARY KEY,
    xref_cust_id  INTEGER NOT NULL,
    xref_acct_id  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_card_xrefs_acct_id ON card_xrefs (xref_acct_id);
CREATE INDEX IF NOT EXISTS idx_card_xrefs_cust_id ON card_xrefs (xref_cust_id);

CREATE TABLE IF NOT EXISTS transactions (
    tran_id            TEXT PRIMARY KEY,
    tran_type_code     TEXT NOT NULL,
    tran_cat_code      INTEGER NOT NULL,
    tran_source        TEXT NOT NULL,
    tran_desc          TEXT NOT NULL,
    tran_amt           TEXT NOT NULL,
    tran_merchant_id   INTEGER NOT NULL,
    tran_merchant_name TEXT NOT NULL,
    tran_merchant_city TEXT NOT NULL,
    tran_merchant_zip  TEXT NOT NULL,
    tran_card_num      TEXT NOT NULL,
    tran_orig_ts       TEXT NOT NULL,
    tran_proc_ts       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_transactions_card_num ON transactions (tran_card_num);

CREATE TABLE IF NOT EXISTS tran_types (
    tran_type      TEXT PRIMARY KEY,
    tran_type_desc TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tran_cats (
    tran_type_cd      TEXT NOT NULL,
    tran_cat_cd       INTEGER NOT NULL,
    tran_cat_type_desc TEXT NOT NULL,
    PRIMARY KEY (tran_type_cd, tran_cat_cd)
);
CREATE INDEX IF NOT EXISTS idx_tran_cats_type_cd ON tran_cats (tran_type_cd);

CREATE TABLE IF NOT EXISTS disc_groups (
    dis_acct_group_id TEXT NOT NULL,
    dis_tran_type_cd  TEXT NOT NULL,
    dis_tran_cat_cd   INTEGER NOT NULL,
    dis_int_rate      TEXT NOT NULL,
    PRIMARY KEY (dis_acct_group_id, dis_tran_type_cd, dis_tran_cat_cd)
);
CREATE INDEX IF NOT EXISTS idx_disc_groups_group_id ON disc_groups (dis_acct_group_id);

CREATE TABLE IF NOT EXISTS tran_cat_bals (
    trancat_acct_id INTEGER NOT NULL,
    trancat_type_cd TEXT NOT NULL,
    trancat_cd      INTEGER NOT NULL,
    tran_cat_balance TEXT NOT NULL,
    PRIMARY KEY (trancat_acct_id, trancat_type_cd, trancat_cd)
);
CREATE INDEX IF NOT EXISTS idx_tran_cat_bals_acct_id ON tran_cat_bals (trancat_acct_id);

CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    pwd_hash   TEXT NOT NULL,
    user_type  TEXT NOT NULL
);
