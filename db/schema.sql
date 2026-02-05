-- db/schema.sql

-- Enable uuid extension if you plan to use UUIDs later,
-- though your code currently uses SERIAL (int) IDs.
-- CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ==========================================
-- 1. USERS
-- ==========================================
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT,
    hash TEXT NOT NULL,
    role TEXT NOT NULL, -- e.g., 'admin', 'clerk'
    active BOOLEAN NOT NULL DEFAULT TRUE,
    setting JSONB, -- Stores map[string]any
    custom JSONB   -- Stores map[string]any
);

-- Index for searching users
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_active ON users(active);

-- ==========================================
-- 2. INVENTORY
-- ==========================================
CREATE TABLE inventory (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    "desc" TEXT, -- "desc" is a reserved keyword in SQL, so it must be quoted
    tag TEXT,
    label TEXT,
    stock BIGINT NOT NULL DEFAULT 0,
    custom JSONB
);

-- Indexes for filtering and searching
CREATE INDEX idx_inventory_tag ON inventory(tag);
CREATE INDEX idx_inventory_label ON inventory(label);
-- Optional: GIN index if you plan to query inside the JSONB custom field
-- CREATE INDEX idx_inventory_custom ON inventory USING GIN (custom);

-- ==========================================
-- 3. PRODUCTS
-- ==========================================
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    "desc" TEXT,
    tag TEXT,
    label TEXT,
    price BIGINT NOT NULL DEFAULT 0,
    avail BOOLEAN NOT NULL DEFAULT TRUE,
    items JSONB,  -- Array of strings (slugs) for bundles
    recipe JSONB, -- Map of string:int for inventory usage
    custom JSONB
);

-- Indexes for filtering
CREATE INDEX idx_products_tag ON products(tag);
CREATE INDEX idx_products_label ON products(label);
CREATE INDEX idx_products_price ON products(price);
CREATE INDEX idx_products_avail ON products(avail);

-- ==========================================
-- 4. ORDERS
-- ==========================================
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    items JSONB NOT NULL, -- Stores []string (product slugs)
    clerk_id INTEGER NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    total BIGINT NOT NULL DEFAULT 0,
    paid BIGINT NOT NULL DEFAULT 0,
    change BIGINT NOT NULL DEFAULT 0,
    custom JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for reporting and history
CREATE INDEX idx_orders_clerk_id ON orders(clerk_id);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_orders_total ON orders(total);
