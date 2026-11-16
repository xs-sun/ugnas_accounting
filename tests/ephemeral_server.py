#!/usr/bin/env python3
"""
tests/ephemeral_server.py — Ephemeral SQLite-backed backend server for Seven Cats Accounting.
Strictly implements the REST API and database contract specified in PROJECT.md.
Used for isolated test suite verification and headless CI execution.
"""
import os
import sys
import json
import sqlite3
import hashlib
import tarfile
import tempfile
import argparse
import re
import email
from email import policy
from datetime import datetime
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

CONFIG = {
    "data_dir": os.environ.get("UGAPP_DATA_DIR", "./data"),
    "log_dir": os.environ.get("UGAPP_LOG_DIR", "./logs"),
    "port": 21010,
    "strict_auth": False
}

def get_db_path():
    os.makedirs(CONFIG["data_dir"], exist_ok=True)
    return os.path.join(CONFIG["data_dir"], "accounting.db")

def get_attachments_dir():
    path = os.path.join(CONFIG["data_dir"], "attachments")
    os.makedirs(path, exist_ok=True)
    return path

def get_snapshots_dir():
    path = os.path.join(CONFIG["data_dir"], "snapshots")
    os.makedirs(path, exist_ok=True)
    return path

def get_db():
    conn = sqlite3.connect(get_db_path(), timeout=5.0)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode = WAL;")
    conn.execute("PRAGMA busy_timeout = 5000;")
    conn.execute("PRAGMA mmap_size = 268435456;")
    conn.execute("PRAGMA synchronous = NORMAL;")
    conn.execute("PRAGMA foreign_keys = ON;")
    return conn

def init_db():
    conn = get_db()
    cur = conn.cursor()
    cur.executescript("""
    CREATE TABLE IF NOT EXISTS books (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        currency TEXT NOT NULL DEFAULT 'CNY',
        description TEXT DEFAULT '',
        owner_id TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE TABLE IF NOT EXISTS book_members (
        book_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        user_name TEXT NOT NULL,
        role TEXT NOT NULL DEFAULT 'editor',
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        PRIMARY KEY (book_id, user_id),
        FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS wallets (
        id TEXT PRIMARY KEY,
        book_id TEXT NOT NULL,
        type TEXT NOT NULL,
        name TEXT NOT NULL,
        balance INTEGER NOT NULL DEFAULT 0,
        credit_limit INTEGER NOT NULL DEFAULT 0,
        billing_day INTEGER DEFAULT 1,
        repayment_day INTEGER DEFAULT 20,
        card_last_four TEXT DEFAULT '',
        exclude_net_asset INTEGER NOT NULL DEFAULT 0,
        is_archived INTEGER NOT NULL DEFAULT 0,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS categories (
        id TEXT PRIMARY KEY,
        book_id TEXT NOT NULL,
        type TEXT NOT NULL,
        name TEXT NOT NULL,
        icon TEXT NOT NULL DEFAULT 'tag',
        color TEXT NOT NULL DEFAULT '#0E8C7F',
        parent_id TEXT DEFAULT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS tags (
        id TEXT PRIMARY KEY,
        book_id TEXT NOT NULL,
        name TEXT NOT NULL,
        color TEXT NOT NULL DEFAULT '#5D6470',
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS transactions (
        id TEXT PRIMARY KEY,
        book_id TEXT NOT NULL,
        type TEXT NOT NULL,
        amount INTEGER NOT NULL,
        date TEXT NOT NULL,
        wallet_id TEXT NOT NULL,
        to_wallet_id TEXT DEFAULT NULL,
        category_id TEXT DEFAULT NULL,
        note TEXT DEFAULT '',
        order_no TEXT DEFAULT '',
        created_by TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
        FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE RESTRICT
    );

    CREATE TABLE IF NOT EXISTS transaction_tags (
        transaction_id TEXT NOT NULL,
        tag_id TEXT NOT NULL,
        PRIMARY KEY (transaction_id, tag_id),
        FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
        FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS attachments (
        id TEXT PRIMARY KEY,
        transaction_id TEXT NOT NULL,
        file_name TEXT NOT NULL,
        file_path TEXT NOT NULL,
        file_size INTEGER NOT NULL,
        mime_type TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS daily_summaries (
        date TEXT NOT NULL,
        book_id TEXT NOT NULL,
        type TEXT NOT NULL,
        category_id TEXT NOT NULL,
        total_amount INTEGER NOT NULL DEFAULT 0,
        count INTEGER NOT NULL DEFAULT 0,
        PRIMARY KEY (date, book_id, type, category_id)
    );

    CREATE INDEX IF NOT EXISTS idx_daily_summaries_query 
    ON daily_summaries (book_id, type, date, category_id, total_amount, count);

    CREATE TABLE IF NOT EXISTS snapshots (
        id TEXT PRIMARY KEY,
        file_name TEXT NOT NULL,
        file_path TEXT NOT NULL,
        file_size INTEGER NOT NULL,
        sha256 TEXT NOT NULL,
        created_by TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );
    """)

    # Triggers for daily_summaries
    cur.executescript("""
    CREATE TRIGGER IF NOT EXISTS trg_tx_insert_summary
    AFTER INSERT ON transactions
    WHEN NEW.type IN ('expense', 'income')
    BEGIN
        INSERT INTO daily_summaries (date, book_id, type, category_id, total_amount, count)
        VALUES (NEW.date, NEW.book_id, NEW.type, COALESCE(NEW.category_id, 'uncategorized'), NEW.amount, 1)
        ON CONFLICT(date, book_id, type, category_id) DO UPDATE SET
            total_amount = total_amount + NEW.amount,
            count = count + 1;
    END;

    CREATE TRIGGER IF NOT EXISTS trg_tx_delete_summary
    AFTER DELETE ON transactions
    WHEN OLD.type IN ('expense', 'income')
    BEGIN
        UPDATE daily_summaries
        SET total_amount = total_amount - OLD.amount,
            count = count - 1
        WHERE date = OLD.date AND book_id = OLD.book_id AND type = OLD.type 
          AND category_id = COALESCE(OLD.category_id, 'uncategorized');
    END;
    """)
    conn.commit()
    conn.close()

class AccountingHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        if os.environ.get("VERBOSE", "0") == "1":
            super().log_message(format, *args)

    def _get_user_context(self):
        user_id = self.headers.get("Ugreen-User-ID")
        user_name = self.headers.get("Ugreen-User-Name", "admin")
        user_type = self.headers.get("Ugreen-User-Type", "admin")
        
        if not user_id:
            if CONFIG["strict_auth"]:
                return None
            user_id = "local_dev_user"
            user_name = "admin"
            user_type = "admin"
        return {"id": user_id, "name": user_name, "type": user_type}

    def _send_json(self, code, status_code=200, message="success", data=None):
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Ugreen-User-ID, Ugreen-User-Name, Ugreen-User-Type, Ugreen-Ttk")
        self.end_headers()
        resp = {"code": code, "message": message}
        if data is not None:
            resp["data"] = data
        self.wfile.write(json.dumps(resp, ensure_ascii=False).encode("utf-8"))

    def _send_bytes(self, data, content_type="application/octet-stream", headers=None, status_code=200):
        self.send_response(status_code)
        self.send_header("Content-Type", content_type)
        if headers:
            for k, v in headers.items():
                self.send_header(k, v)
        self.end_headers()
        self.wfile.write(data)

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Ugreen-User-ID, Ugreen-User-Name, Ugreen-User-Type, Ugreen-Ttk")
        self.end_headers()

    def do_GET(self):
        ctx = self._get_user_context()
        if not ctx:
            self._send_json(401, 401, "unauthorized")
            return

        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        # Health
        if path == "/api/v1/health":
            self._send_json(0, 200, "success", {"status": "ok", "version": "0.1.0"})
            return

        conn = get_db()
        try:
            # Snapshots list
            if path == "/api/v1/snapshots":
                rows = conn.execute("SELECT id, file_name, file_size, sha256, created_by, created_at FROM snapshots ORDER BY created_at DESC").fetchall()
                data = [dict(r) for r in rows]
                self._send_json(0, 200, "success", data)
                return

            # Snapshot download
            m = re.match(r"^/api/v1/snapshots/([^/]+)/download$", path)
            if m:
                snap_id = m.group(1)
                if ".." in snap_id or "/" in snap_id:
                    self._send_json(400, 400, "invalid snapshot id")
                    return
                row = conn.execute("SELECT * FROM snapshots WHERE id = ?", (snap_id,)).fetchone()
                if not row or not os.path.exists(row["file_path"]):
                    self._send_json(404, 404, "snapshot not found")
                    return
                with open(row["file_path"], "rb") as f:
                    content = f.read()
                self._send_bytes(content, "application/octet-stream", {
                    "Content-Disposition": f'attachment; filename="{row["file_name"]}"'
                })
                return

            # Books list
            if path == "/api/v1/books":
                rows = conn.execute("""
                    SELECT b.*, bm.role FROM books b
                    JOIN book_members bm ON b.id = bm.book_id
                    WHERE bm.user_id = ?
                    ORDER BY b.created_at DESC
                """, (ctx["id"],)).fetchall()
                self._send_json(0, 200, "success", [dict(r) for r in rows])
                return

            # Single book
            m = re.match(r"^/api/v1/books/([^/]+)$", path)
            if m:
                book_id = m.group(1)
                row = conn.execute("""
                    SELECT b.*, bm.role FROM books b
                    JOIN book_members bm ON b.id = bm.book_id
                    WHERE b.id = ? AND bm.user_id = ?
                """, (book_id, ctx["id"])).fetchone()
                if not row:
                    self._send_json(404, 404, "book not found")
                    return
                self._send_json(0, 200, "success", dict(row))
                return

            # Book members
            m = re.match(r"^/api/v1/books/([^/]+)/members$", path)
            if m:
                book_id = m.group(1)
                rows = conn.execute("SELECT * FROM book_members WHERE book_id = ?", (book_id,)).fetchall()
                self._send_json(0, 200, "success", [dict(r) for r in rows])
                return

            # Wallets list
            if path == "/api/v1/wallets":
                book_id = query.get("book_id", [""])[0]
                sql = "SELECT * FROM wallets WHERE 1=1"
                params = []
                if book_id:
                    sql += " AND book_id = ?"
                    params.append(book_id)
                rows = conn.execute(sql, params).fetchall()
                self._send_json(0, 200, "success", [dict(r) for r in rows])
                return

            # Wallets summary (Net assets)
            if path == "/api/v1/wallets/summary":
                book_id = query.get("book_id", [""])[0]
                sql = "SELECT * FROM wallets WHERE exclude_net_asset = 0 AND is_archived = 0"
                params = []
                if book_id:
                    sql += " AND book_id = ?"
                    params.append(book_id)
                wallets = conn.execute(sql, params).fetchall()
                total_assets = 0
                total_liabilities = 0
                for w in wallets:
                    bal = w["balance"]
                    wtype = w["type"]
                    if wtype == "credit":
                        if bal < 0:
                            total_liabilities += abs(bal)
                        else:
                            total_assets += bal
                    else:
                        if bal >= 0:
                            total_assets += bal
                        else:
                            total_liabilities += abs(bal)
                net_assets = total_assets - total_liabilities
                self._send_json(0, 200, "success", {
                    "total_assets": total_assets,
                    "total_liabilities": total_liabilities,
                    "net_assets": net_assets
                })
                return

            # Categories list
            if path == "/api/v1/categories":
                book_id = query.get("book_id", [""])[0]
                sql = "SELECT * FROM categories WHERE 1=1"
                params = []
                if book_id:
                    sql += " AND book_id = ?"
                    params.append(book_id)
                rows = conn.execute(sql, params).fetchall()
                self._send_json(0, 200, "success", [dict(r) for r in rows])
                return

            # Tags list
            if path == "/api/v1/tags":
                book_id = query.get("book_id", [""])[0]
                sql = "SELECT * FROM tags WHERE 1=1"
                params = []
                if book_id:
                    sql += " AND book_id = ?"
                    params.append(book_id)
                rows = conn.execute(sql, params).fetchall()
                self._send_json(0, 200, "success", [dict(r) for r in rows])
                return

            # Transactions list
            if path == "/api/v1/transactions":
                book_id = query.get("book_id", [""])[0]
                start_date = query.get("start_date", [""])[0]
                end_date = query.get("end_date", [""])[0]
                page = int(query.get("page", ["1"])[0])
                page_size = int(query.get("page_size", ["20"])[0])

                if page < 1:
                    page = 1
                if page_size > 100:
                    page_size = 100

                if start_date and end_date and start_date > end_date:
                    self._send_json(400, 400, "invalid date range")
                    return

                conditions = ["1=1"]
                params = []
                if book_id:
                    conditions.append("t.book_id = ?")
                    params.append(book_id)
                if start_date:
                    conditions.append("t.date >= ?")
                    params.append(start_date)
                if end_date:
                    conditions.append("t.date <= ?")
                    params.append(end_date)

                where_clause = " AND ".join(conditions)
                total = conn.execute(f"SELECT COUNT(*) FROM transactions t WHERE {where_clause}", params).fetchone()[0]
                
                offset = (page - 1) * page_size
                sql = f"""
                    SELECT t.*, c.name as category_name, c.icon as category_icon, c.color as category_color,
                           w.name as wallet_name, tw.name as to_wallet_name
                    FROM transactions t
                    LEFT JOIN categories c ON t.category_id = c.id
                    LEFT JOIN wallets w ON t.wallet_id = w.id
                    LEFT JOIN wallets tw ON t.to_wallet_id = tw.id
                    WHERE {where_clause}
                    ORDER BY t.date DESC, t.created_at DESC
                    LIMIT ? OFFSET ?
                """
                params.extend([page_size, offset])
                rows = conn.execute(sql, params).fetchall()
                items = []
                for r in rows:
                    d = dict(r)
                    tag_rows = conn.execute("""
                        SELECT tg.id, tg.name, tg.color FROM tags tg
                        JOIN transaction_tags tt ON tg.id = tt.tag_id
                        WHERE tt.transaction_id = ?
                    """, (d["id"],)).fetchall()
                    d["tags"] = [dict(tr) for tr in tag_rows]
                    items.append(d)

                self._send_json(0, 200, "success", {
                    "items": items,
                    "total": total,
                    "page": page,
                    "page_size": page_size
                })
                return

            # Single transaction
            m = re.match(r"^/api/v1/transactions/([^/]+)$", path)
            if m:
                tx_id = m.group(1)
                row = conn.execute("SELECT * FROM transactions WHERE id = ?", (tx_id,)).fetchone()
                if not row:
                    self._send_json(404, 404, "transaction not found")
                    return
                d = dict(row)
                tag_rows = conn.execute("""
                    SELECT tg.id, tg.name, tg.color FROM tags tg
                    JOIN transaction_tags tt ON tg.id = tt.tag_id
                    WHERE tt.transaction_id = ?
                """, (d["id"],)).fetchall()
                d["tags"] = [dict(tr) for tr in tag_rows]
                self._send_json(0, 200, "success", d)
                return

            # Monthly report (O(1) pre-agg)
            if path == "/api/v1/reports/monthly":
                year = query.get("year", [str(datetime.now().year)])[0]
                month = query.get("month", [f"{datetime.now().month:02d}"])[0]
                book_id = query.get("book_id", [""])[0]

                if not month.isdigit() or int(month) < 1 or int(month) > 12:
                    self._send_json(400, 400, "invalid month parameter")
                    return

                month_str = f"{int(month):02d}"
                prefix = f"{year}-{month_str}"

                conditions = ["date LIKE ?"]
                params = [f"{prefix}%"]
                if book_id:
                    conditions.append("book_id = ?")
                    params.append(book_id)

                where_clause = " AND ".join(conditions)
                rows = conn.execute(f"""
                    SELECT type, category_id, SUM(total_amount) as amount, SUM(count) as cnt
                    FROM daily_summaries
                    WHERE {where_clause}
                    GROUP BY type, category_id
                """, params).fetchall()

                total_expense = 0
                total_income = 0
                category_map = {}

                for r in rows:
                    amt = r["amount"] or 0
                    t = r["type"]
                    cat_id = r["category_id"]
                    if t == "expense":
                        total_expense += amt
                        category_map[cat_id] = category_map.get(cat_id, 0) + amt
                    elif t == "income":
                        total_income += amt

                cat_list = []
                for cid, camt in category_map.items():
                    cname = "未分类"
                    cicon = "tag"
                    ccolor = "#0E8C7F"
                    if cid != "uncategorized":
                        crow = conn.execute("SELECT name, icon, color FROM categories WHERE id = ?", (cid,)).fetchone()
                        if crow:
                            cname, cicon, ccolor = crow["name"], crow["icon"], crow["color"]
                    pct = round(camt / total_expense * 100, 2) if total_expense > 0 else 0.0
                    cat_list.append({
                        "category_id": cid,
                        "category_name": cname,
                        "category_icon": cicon,
                        "category_color": ccolor,
                        "amount": camt,
                        "percentage": pct
                    })
                cat_list.sort(key=lambda x: x["amount"], reverse=True)

                self._send_json(0, 200, "success", {
                    "year": int(year),
                    "month": int(month),
                    "total_expense": total_expense,
                    "total_income": total_income,
                    "net_savings": total_income - total_expense,
                    "categories": cat_list
                })
                return

            # Trend report
            if path == "/api/v1/reports/trend":
                months_cnt = int(query.get("months", ["6"])[0])
                if months_cnt < 1: months_cnt = 1
                if months_cnt > 24: months_cnt = 24
                book_id = query.get("book_id", [""])[0]

                rows = conn.execute("""
                    SELECT substr(date, 1, 7) as ym, type, SUM(total_amount) as total
                    FROM daily_summaries
                    GROUP BY ym, type
                    ORDER BY ym DESC
                    LIMIT ?
                """, (months_cnt * 2,)).fetchall()
                trends = {}
                for r in rows:
                    ym = r["ym"]
                    if ym not in trends:
                        trends[ym] = {"month": ym, "expense": 0, "income": 0}
                    if r["type"] == "expense":
                        trends[ym]["expense"] = r["total"]
                    elif r["type"] == "income":
                        trends[ym]["income"] = r["total"]
                res = sorted(list(trends.values()), key=lambda x: x["month"])
                self._send_json(0, 200, "success", {"months": res})
                return

            # Daily breakdown report
            if path == "/api/v1/reports/daily":
                year = query.get("year", [str(datetime.now().year)])[0]
                month = query.get("month", [f"{datetime.now().month:02d}"])[0]
                prefix = f"{year}-{int(month):02d}"
                rows = conn.execute("""
                    SELECT date, type, SUM(total_amount) as amount
                    FROM daily_summaries
                    WHERE date LIKE ?
                    GROUP BY date, type
                    ORDER BY date ASC
                """, (f"{prefix}%",)).fetchall()
                daily = {}
                for r in rows:
                    d = r["date"]
                    if d not in daily: daily[d] = {"date": d, "expense": 0, "income": 0}
                    if r["type"] == "expense": daily[d]["expense"] = r["amount"]
                    elif r["type"] == "income": daily[d]["income"] = r["amount"]
                self._send_json(0, 200, "success", list(daily.values()))
                return

            # Full Ledger CSV export
            if path == "/api/v1/export":
                book_id = query.get("book_id", [""])[0]
                conditions = ["1=1"]
                params = []
                if book_id:
                    conditions.append("t.book_id = ?")
                    params.append(book_id)
                sql = f"""
                    SELECT t.date, t.type, COALESCE(c.name, '未分类') as category,
                           w.name as wallet, t.amount, t.note
                    FROM transactions t
                    LEFT JOIN categories c ON t.category_id = c.id
                    LEFT JOIN wallets w ON t.wallet_id = w.id
                    WHERE {" AND ".join(conditions)}
                    ORDER BY t.date DESC
                """
                txs = conn.execute(sql, params).fetchall()
                csv_lines = ["\ufeff交易日期,交易类型,分类,账户,金额(元),备注"]
                for r in txs:
                    amt_str = f"{r['amount'] / 100:.2f}"
                    note_esc = '"' + (r["note"] or "").replace('"', '""') + '"'
                    csv_lines.append(f'{r["date"]},{r["type"]},{r["category"]},{r["wallet"]},{amt_str},{note_esc}')
                csv_bytes = ("\n".join(csv_lines) + "\n").encode("utf-8")
                self._send_bytes(csv_bytes, "text/csv; charset=utf-8", {
                    "Content-Disposition": 'attachment; filename="ledger_export.csv"'
                })
                return

            self._send_json(404, 404, f"not found: {path}")
        finally:
            conn.close()

    def do_POST(self):
        ctx = self._get_user_context()
        if not ctx:
            self._send_json(401, 401, "unauthorized")
            return

        parsed = urlparse(self.path)
        path = parsed.path
        ctype = self.headers.get("Content-Type", "")

        # Multipart handlers (import CSV, snapshots restore, attachment)
        if "multipart/form-data" in ctype:
            self._handle_multipart(path, ctx)
            return

        # JSON body handling
        content_len = int(self.headers.get("Content-Length", 0))
        if content_len >= 5 * 1024 * 1024:
            self._send_json(413, 413, "payload too large")
            return

        body_raw = self.rfile.read(content_len).decode("utf-8", errors="replace")
        try:
            body = json.loads(body_raw) if body_raw.strip() else {}
        except Exception:
            self._send_json(400, 400, "invalid json")
            return

        conn = get_db()
        try:
            # Create book
            if path == "/api/v1/books":
                name = body.get("name", "").strip()
                if not name:
                    self._send_json(400, 400, "book name cannot be blank")
                    return
                book_id = body.get("id") or f"book_{os.urandom(4).hex()}"
                currency = body.get("currency", "CNY")
                desc = body.get("description", "")
                conn.execute("""
                    INSERT INTO books (id, name, currency, description, owner_id)
                    VALUES (?, ?, ?, ?, ?)
                """, (book_id, name, currency, desc, ctx["id"]))
                conn.execute("""
                    INSERT INTO book_members (book_id, user_id, user_name, role)
                    VALUES (?, ?, ?, 'owner')
                """, (book_id, ctx["id"], ctx["name"]))
                conn.commit()
                row = conn.execute("SELECT * FROM books WHERE id = ?", (book_id,)).fetchone()
                d = dict(row)
                d["role"] = "owner"
                self._send_json(0, 201, "success", d)
                return

            # Add book member
            m = re.match(r"^/api/v1/books/([^/]+)/members$", path)
            if m:
                book_id = m.group(1)
                user_id = body.get("user_id")
                user_name = body.get("user_name", "member")
                role = body.get("role", "editor")

                caller_mem = conn.execute("SELECT role FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, ctx["id"])).fetchone()
                if not caller_mem or caller_mem["role"] != "owner":
                    self._send_json(403, 403, "only owner can add members")
                    return

                exist = conn.execute("SELECT 1 FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, user_id)).fetchone()
                if exist:
                    self._send_json(409, 409, "member already exists")
                    return

                conn.execute("""
                    INSERT INTO book_members (book_id, user_id, user_name, role)
                    VALUES (?, ?, ?, ?)
                """, (book_id, user_id, user_name, role))
                conn.commit()
                self._send_json(0, 201, "success", {"book_id": book_id, "user_id": user_id, "role": role})
                return

            # Create wallet
            if path == "/api/v1/wallets":
                book_id = body.get("book_id")
                name = body.get("name", "").strip()
                wtype = body.get("type", "custom")
                balance = int(body.get("balance", 0))
                credit_limit = int(body.get("credit_limit", 0))
                billing_day = int(body.get("billing_day", 1))
                repayment_day = int(body.get("repayment_day", 20))
                card_last_four = body.get("card_last_four", "")
                exclude = 1 if body.get("exclude_net_asset") else 0

                if book_id:
                    caller_mem = conn.execute("SELECT role FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, ctx["id"])).fetchone()
                    if caller_mem and caller_mem["role"] == "viewer":
                        self._send_json(403, 403, "viewers cannot create wallets")
                        return

                if not name:
                    self._send_json(400, 400, "wallet name required")
                    return
                if billing_day < 1 or billing_day > 31:
                    self._send_json(400, 400, "billing_day must be between 1 and 31")
                    return
                if repayment_day < 1 or repayment_day > 31:
                    self._send_json(400, 400, "repayment_day must be between 1 and 31")
                    return
                if card_last_four and not re.match(r"^\d{4}$", card_last_four):
                    self._send_json(400, 400, "card_last_four must be exactly 4 digits")
                    return

                wallet_id = body.get("id") or f"w_{os.urandom(4).hex()}"
                conn.execute("""
                    INSERT INTO wallets (id, book_id, type, name, balance, credit_limit, billing_day, repayment_day, card_last_four, exclude_net_asset)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, (wallet_id, book_id, wtype, name, balance, credit_limit, billing_day, repayment_day, card_last_four, exclude))
                conn.commit()
                row = conn.execute("SELECT * FROM wallets WHERE id = ?", (wallet_id,)).fetchone()
                self._send_json(0, 201, "success", dict(row))
                return

            # Create category
            if path == "/api/v1/categories":
                book_id = body.get("book_id")
                ctype_name = body.get("type", "expense")
                name = body.get("name", "").strip()
                icon = body.get("icon", "tag")
                color = body.get("color", "#0E8C7F")
                parent_id = body.get("parent_id")

                if not name:
                    self._send_json(400, 400, "category name required")
                    return

                exist = conn.execute("SELECT 1 FROM categories WHERE book_id = ? AND name = ?", (book_id, name)).fetchone()
                if exist:
                    self._send_json(409, 409, "category already exists in this book")
                    return

                cat_id = body.get("id") or f"cat_{os.urandom(4).hex()}"
                conn.execute("""
                    INSERT INTO categories (id, book_id, type, name, icon, color, parent_id)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                """, (cat_id, book_id, ctype_name, name, icon, color, parent_id))
                conn.commit()
                row = conn.execute("SELECT * FROM categories WHERE id = ?", (cat_id,)).fetchone()
                self._send_json(0, 201, "success", dict(row))
                return

            # Create tag
            if path == "/api/v1/tags":
                book_id = body.get("book_id")
                name = body.get("name", "").strip()
                color = body.get("color", "#5D6470")
                if not name:
                    self._send_json(400, 400, "tag name required")
                    return
                tag_id = body.get("id") or f"tag_{os.urandom(4).hex()}"
                conn.execute("""
                    INSERT INTO tags (id, book_id, name, color)
                    VALUES (?, ?, ?, ?)
                """, (tag_id, book_id, name, color))
                conn.commit()
                row = conn.execute("SELECT * FROM tags WHERE id = ?", (tag_id,)).fetchone()
                self._send_json(0, 201, "success", dict(row))
                return

            # Create transaction
            if path == "/api/v1/transactions":
                book_id = body.get("book_id")
                ttype = body.get("type", "expense")
                amount = body.get("amount")
                date = body.get("date", datetime.now().strftime("%Y-%m-%d"))
                wallet_id = body.get("wallet_id")
                to_wallet_id = body.get("to_wallet_id")
                category_id = body.get("category_id")
                note = body.get("note", "")
                tags = body.get("tags", [])

                if not isinstance(amount, (int, float)) or amount <= 0:
                    self._send_json(400, 400, "amount must be positive integer cents")
                    return
                amount = int(amount)

                caller_mem = conn.execute("SELECT role FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, ctx["id"])).fetchone()
                if caller_mem and caller_mem["role"] == "viewer":
                    self._send_json(403, 403, "viewers cannot create transactions")
                    return

                if ttype == "transfer":
                    if not to_wallet_id:
                        self._send_json(400, 400, "to_wallet_id required for transfer")
                        return
                    if wallet_id == to_wallet_id:
                        self._send_json(400, 400, "cannot transfer to same wallet")
                        return
                    w1 = conn.execute("SELECT book_id FROM wallets WHERE id = ?", (wallet_id,)).fetchone()
                    w2 = conn.execute("SELECT book_id FROM wallets WHERE id = ?", (to_wallet_id,)).fetchone()
                    if not w1 or not w2 or w1["book_id"] != w2["book_id"]:
                        self._send_json(400, 400, "wallets must belong to the same book")
                        return

                tx_id = body.get("id") or f"tx_{os.urandom(4).hex()}"
                
                conn.execute("BEGIN IMMEDIATE;")
                conn.execute("""
                    INSERT INTO transactions (id, book_id, type, amount, date, wallet_id, to_wallet_id, category_id, note, created_by)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, (tx_id, book_id, ttype, amount, date, wallet_id, to_wallet_id, category_id, note, ctx["id"]))

                if ttype == "expense":
                    conn.execute("UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?", (amount, wallet_id))
                elif ttype == "income":
                    conn.execute("UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?", (amount, wallet_id))
                elif ttype == "transfer":
                    conn.execute("UPDATE wallets SET balance = balance - ?, updated_at = datetime('now') WHERE id = ?", (amount, wallet_id))
                    conn.execute("UPDATE wallets SET balance = balance + ?, updated_at = datetime('now') WHERE id = ?", (amount, to_wallet_id))

                for tid in tags:
                    conn.execute("INSERT OR IGNORE INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)", (tx_id, tid))

                conn.commit()
                row = conn.execute("SELECT * FROM transactions WHERE id = ?", (tx_id,)).fetchone()
                d = dict(row)
                tag_rows = conn.execute("""
                    SELECT tg.id, tg.name, tg.color FROM tags tg
                    JOIN transaction_tags tt ON tg.id = tt.tag_id
                    WHERE tt.transaction_id = ?
                """, (tx_id,)).fetchall()
                d["tags"] = [dict(tr) for tr in tag_rows]
                self._send_json(0, 201, "success", d)
                return

            # Commit imported batch
            if path == "/api/v1/imports/commit":
                book_id = body.get("book_id")
                wallet_id = body.get("wallet_id")
                items = body.get("items", [])
                
                inserted = 0
                conn.execute("BEGIN IMMEDIATE;")
                for it in items:
                    order_no = it.get("order_no", "")
                    if order_no:
                        dup = conn.execute("SELECT 1 FROM transactions WHERE book_id = ? AND order_no = ?", (book_id, order_no)).fetchone()
                        if dup:
                            continue
                    tx_id = f"tx_imp_{os.urandom(4).hex()}"
                    ttype = it.get("type", "expense")
                    amt = int(it.get("amount", 0))
                    dt = it.get("date", datetime.now().strftime("%Y-%m-%d"))
                    cid = it.get("category_id")
                    note = it.get("note", "")

                    conn.execute("""
                        INSERT INTO transactions (id, book_id, type, amount, date, wallet_id, category_id, note, order_no, created_by)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """, (tx_id, book_id, ttype, amt, dt, wallet_id, cid, note, order_no, ctx["id"]))

                    if ttype == "expense":
                        conn.execute("UPDATE wallets SET balance = balance - ? WHERE id = ?", (amt, wallet_id))
                    elif ttype == "income":
                        conn.execute("UPDATE wallets SET balance = balance + ? WHERE id = ?", (amt, wallet_id))
                    inserted += 1
                conn.commit()
                self._send_json(0, 200, "success", {"inserted": inserted})
                return

            # Hot snapshot creation (VACUUM INTO)
            if path == "/api/v1/snapshots/create":
                snap_id = f"snap_{int(datetime.now().timestamp())}_{os.urandom(2).hex()}"
                snaps_dir = get_snapshots_dir()
                raw_db_backup = os.path.join(snaps_dir, f"{snap_id}.sqlite")
                
                conn.execute(f"VACUUM INTO '{raw_db_backup}';")

                ugledger_path = os.path.join(snaps_dir, f"{snap_id}.ugledger")
                with tarfile.open(ugledger_path, "w:gz") as tar:
                    tar.add(raw_db_backup, arcname="accounting.db")
                    att_dir = get_attachments_dir()
                    if os.path.exists(att_dir):
                        tar.add(att_dir, arcname="attachments")
                
                if os.path.exists(raw_db_backup):
                    os.remove(raw_db_backup)

                file_size = os.path.getsize(ugledger_path)
                with open(ugledger_path, "rb") as f:
                    sha = hashlib.sha256(f.read()).hexdigest()

                conn.execute("""
                    INSERT INTO snapshots (id, file_name, file_path, file_size, sha256, created_by)
                    VALUES (?, ?, ?, ?, ?, ?)
                """, (snap_id, f"{snap_id}.ugledger", ugledger_path, file_size, sha, ctx["id"]))
                conn.commit()

                self._send_json(0, 201, "success", {
                    "id": snap_id,
                    "file_name": f"{snap_id}.ugledger",
                    "file_size": file_size,
                    "sha256": sha
                })
                return

            self._send_json(404, 404, f"not found: {path}")
        except sqlite3.IntegrityError as e:
            conn.rollback()
            self._send_json(400, 400, f"database constraint error: {e}")
        finally:
            conn.close()

    def do_PUT(self):
        ctx = self._get_user_context()
        if not ctx:
            self._send_json(401, 401, "unauthorized")
            return

        parsed = urlparse(self.path)
        path = parsed.path
        content_len = int(self.headers.get("Content-Length", 0))
        body_raw = self.rfile.read(content_len).decode("utf-8", errors="replace")
        try:
            body = json.loads(body_raw) if body_raw.strip() else {}
        except Exception:
            self._send_json(400, 400, "invalid json")
            return

        conn = get_db()
        try:
            # Update book
            m = re.match(r"^/api/v1/books/([^/]+)$", path)
            if m:
                book_id = m.group(1)
                name = body.get("name")
                desc = body.get("description")
                updates = []
                params = []
                if name is not None:
                    updates.append("name = ?")
                    params.append(name)
                if desc is not None:
                    updates.append("description = ?")
                    params.append(desc)
                if updates:
                    params.append(book_id)
                    conn.execute(f"UPDATE books SET {', '.join(updates)}, updated_at = datetime('now') WHERE id = ?", params)
                    conn.commit()
                row = conn.execute("SELECT * FROM books WHERE id = ?", (book_id,)).fetchone()
                self._send_json(0, 200, "success", dict(row))
                return

            # Update wallet
            m = re.match(r"^/api/v1/wallets/([^/]+)$", path)
            if m:
                wallet_id = m.group(1)
                updates = []
                params = []
                for field in ["name", "balance", "credit_limit", "billing_day", "repayment_day", "exclude_net_asset", "is_archived"]:
                    if field in body:
                        updates.append(f"{field} = ?")
                        params.append(body[field])
                if updates:
                    params.append(wallet_id)
                    conn.execute(f"UPDATE wallets SET {', '.join(updates)}, updated_at = datetime('now') WHERE id = ?", params)
                    conn.commit()
                row = conn.execute("SELECT * FROM wallets WHERE id = ?", (wallet_id,)).fetchone()
                self._send_json(0, 200, "success", dict(row))
                return

            # Update transaction
            m = re.match(r"^/api/v1/transactions/([^/]+)$", path)
            if m:
                tx_id = m.group(1)
                old_tx = conn.execute("SELECT * FROM transactions WHERE id = ?", (tx_id,)).fetchone()
                if not old_tx:
                    self._send_json(404, 404, "transaction not found")
                    return
                
                new_amount = body.get("amount", old_tx["amount"])
                new_date = body.get("date", old_tx["date"])
                new_wallet = body.get("wallet_id", old_tx["wallet_id"])
                new_cat = body.get("category_id", old_tx["category_id"])
                new_note = body.get("note", old_tx["note"])

                conn.execute("BEGIN IMMEDIATE;")
                old_type = old_tx["type"]
                old_amt = old_tx["amount"]
                old_w = old_tx["wallet_id"]
                if old_type == "expense":
                    conn.execute("UPDATE wallets SET balance = balance + ? WHERE id = ?", (old_amt, old_w))
                elif old_type == "income":
                    conn.execute("UPDATE wallets SET balance = balance - ? WHERE id = ?", (old_amt, old_w))

                if old_type == "expense":
                    conn.execute("UPDATE wallets SET balance = balance - ? WHERE id = ?", (new_amount, new_wallet))
                elif old_type == "income":
                    conn.execute("UPDATE wallets SET balance = balance + ? WHERE id = ?", (new_amount, new_wallet))

                conn.execute("""
                    UPDATE transactions
                    SET amount = ?, date = ?, wallet_id = ?, category_id = ?, note = ?, updated_at = datetime('now')
                    WHERE id = ?
                """, (new_amount, new_date, new_wallet, new_cat, new_note, tx_id))

                if new_date != old_tx["date"] or new_cat != old_tx["category_id"] or new_amount != old_amt:
                    conn.execute("""
                        UPDATE daily_summaries
                        SET total_amount = total_amount - ?, count = count - 1
                        WHERE date = ? AND book_id = ? AND type = ? AND category_id = COALESCE(?, 'uncategorized')
                    """, (old_amt, old_tx["date"], old_tx["book_id"], old_type, old_tx["category_id"]))
                    conn.execute("""
                        INSERT INTO daily_summaries (date, book_id, type, category_id, total_amount, count)
                        VALUES (?, ?, ?, COALESCE(?, 'uncategorized'), ?, 1)
                        ON CONFLICT(date, book_id, type, category_id) DO UPDATE SET
                            total_amount = total_amount + ?, count = count + 1
                    """, (new_date, old_tx["book_id"], old_type, new_cat, new_amount, new_amount))

                conn.commit()
                row = conn.execute("SELECT * FROM transactions WHERE id = ?", (tx_id,)).fetchone()
                self._send_json(0, 200, "success", dict(row))
                return

            self._send_json(404, 404, f"not found: {path}")
        finally:
            conn.close()

    def do_DELETE(self):
        ctx = self._get_user_context()
        if not ctx:
            self._send_json(401, 401, "unauthorized")
            return

        parsed = urlparse(self.path)
        path = parsed.path
        conn = get_db()
        try:
            # Delete book
            m = re.match(r"^/api/v1/books/([^/]+)$", path)
            if m:
                book_id = m.group(1)
                caller_mem = conn.execute("SELECT role FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, ctx["id"])).fetchone()
                if not caller_mem or caller_mem["role"] != "owner":
                    self._send_json(403, 403, "only owner can delete book")
                    return
                conn.execute("DELETE FROM books WHERE id = ?", (book_id,))
                conn.commit()
                self._send_json(0, 200, "success")
                return

            # Remove book member
            m = re.match(r"^/api/v1/books/([^/]+)/members/([^/]+)$", path)
            if m:
                book_id, target_user = m.group(1), m.group(2)
                caller_mem = conn.execute("SELECT role FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, ctx["id"])).fetchone()
                if not caller_mem or caller_mem["role"] != "owner":
                    self._send_json(403, 403, "only owner can remove members")
                    return
                if target_user == ctx["id"]:
                    self._send_json(400, 400, "sole owner cannot remove self")
                    return
                conn.execute("DELETE FROM book_members WHERE book_id = ? AND user_id = ?", (book_id, target_user))
                conn.commit()
                self._send_json(0, 200, "success")
                return

            # Archive / Delete wallet
            m = re.match(r"^/api/v1/wallets/([^/]+)$", path)
            if m:
                wallet_id = m.group(1)
                conn.execute("UPDATE wallets SET is_archived = 1, updated_at = datetime('now') WHERE id = ?", (wallet_id,))
                conn.commit()
                self._send_json(0, 200, "success")
                return

            # Delete transaction
            m = re.match(r"^/api/v1/transactions/([^/]+)$", path)
            if m:
                tx_id = m.group(1)
                tx = conn.execute("SELECT * FROM transactions WHERE id = ?", (tx_id,)).fetchone()
                if not tx:
                    self._send_json(404, 404, "transaction not found")
                    return
                conn.execute("BEGIN IMMEDIATE;")
                ttype = tx["type"]
                amt = tx["amount"]
                wid = tx["wallet_id"]
                if ttype == "expense":
                    conn.execute("UPDATE wallets SET balance = balance + ? WHERE id = ?", (amt, wid))
                elif ttype == "income":
                    conn.execute("UPDATE wallets SET balance = balance - ? WHERE id = ?", (amt, wid))
                elif ttype == "transfer":
                    conn.execute("UPDATE wallets SET balance = balance + ? WHERE id = ?", (amt, wid))
                    conn.execute("UPDATE wallets SET balance = balance - ? WHERE id = ?", (amt, tx["to_wallet_id"]))
                conn.execute("DELETE FROM transactions WHERE id = ?", (tx_id,))
                conn.commit()
                self._send_json(0, 200, "success")
                return

            # Delete snapshot
            m = re.match(r"^/api/v1/snapshots/([^/]+)$", path)
            if m:
                snap_id = m.group(1)
                row = conn.execute("SELECT * FROM snapshots WHERE id = ?", (snap_id,)).fetchone()
                if row:
                    if os.path.exists(row["file_path"]):
                        os.remove(row["file_path"])
                    conn.execute("DELETE FROM snapshots WHERE id = ?", (snap_id,))
                    conn.commit()
                self._send_json(0, 200, "success")
                return

            self._send_json(404, 404, f"not found: {path}")
        finally:
            conn.close()

    def _parse_multipart_parts(self):
        content_len = int(self.headers.get("Content-Length", 0))
        raw_body = self.rfile.read(content_len)
        content_type = self.headers.get("Content-Type", "")
        header_bytes = f"Content-Type: {content_type}\r\n\r\n".encode("utf-8")
        msg = email.message_from_bytes(header_bytes + raw_body, policy=policy.default)
        parts = {}
        for part in msg.iter_parts():
            name = part.get_param("name", header="content-disposition")
            filename = part.get_filename()
            payload = part.get_payload(decode=True)
            if name:
                parts[name] = {"filename": filename, "data": payload}
        return parts

    def _handle_multipart(self, path, ctx):
        parts = self._parse_multipart_parts()

        # WeChat CSV Import
        if path == "/api/v1/imports/wechat":
            if "file" not in parts:
                self._send_json(400, 400, "file field required")
                return
            raw_bytes = parts["file"]["data"]
            if raw_bytes.startswith(b'\xef\xbb\xbf'):
                raw_bytes = raw_bytes[3:]
            text = raw_bytes.decode('utf-8', errors='replace')
            lines = text.splitlines()
            parsed_items = []
            header_found = False
            for line in lines:
                if "----------------------微信支付账单明细列表" in line:
                    header_found = True
                    continue
                if not header_found:
                    continue
                cols = [p.strip() for p in line.split(',')]
                if len(cols) >= 10 and cols[0] != "交易时间":
                    dt = cols[0].split(' ')[0]
                    ttype = "income" if cols[4] == "收入" else "expense"
                    amt_str = cols[5].replace('¥', '').strip()
                    try:
                        amt_cents = int(round(float(amt_str) * 100))
                    except ValueError:
                        continue
                    counterparty = cols[2]
                    cat_id = None
                    if "外卖" in counterparty or "咖啡" in counterparty or "麦当劳" in counterparty or "海底捞" in counterparty:
                        cat_id = "cat_food"
                    elif "滴滴" in counterparty or "地铁" in counterparty:
                        cat_id = "cat_transport"
                    elif "超市" in counterparty:
                        cat_id = "cat_shopping"

                    parsed_items.append({
                        "date": dt,
                        "type": ttype,
                        "counterparty": counterparty,
                        "product": cols[3],
                        "amount": amt_cents,
                        "order_no": cols[8],
                        "suggested_category_id": cat_id,
                        "note": cols[10] if len(cols) > 10 and cols[10] != '/' else cols[3]
                    })
            self._send_json(0, 200, "success", {"items": parsed_items, "count": len(parsed_items)})
            return

        # Alipay CSV Import
        if path == "/api/v1/imports/alipay":
            if "file" not in parts:
                self._send_json(400, 400, "file field required")
                return
            raw_bytes = parts["file"]["data"]
            try:
                text = raw_bytes.decode('utf-8')
            except UnicodeDecodeError:
                text = raw_bytes.decode('gbk', errors='replace')
            lines = text.splitlines()
            parsed_items = []
            header_found = False
            for line in lines:
                if "----------------------------------------------------" in line:
                    header_found = True
                    continue
                if not header_found:
                    continue
                cols = [p.strip() for p in line.split(',')]
                if len(cols) >= 10 and cols[0] != "交易时间":
                    dt = cols[0].split(' ')[0]
                    ttype = "income" if cols[5] == "收入" else "expense"
                    try:
                        amt_cents = int(round(float(cols[6]) * 100))
                    except ValueError:
                        continue
                    counterparty = cols[2]
                    cat_id = None
                    if "外卖" in counterparty or "星巴克" in counterparty or "肯德基" in counterparty:
                        cat_id = "cat_food"
                    elif "滴滴" in counterparty or "铁路" in counterparty or "单车" in counterparty:
                        cat_id = "cat_transport"
                    elif "山姆" in counterparty or "淘宝" in counterparty or "盒马" in counterparty or "京东" in counterparty:
                        cat_id = "cat_shopping"

                    parsed_items.append({
                        "date": dt,
                        "type": ttype,
                        "counterparty": counterparty,
                        "product": cols[4],
                        "amount": amt_cents,
                        "order_no": cols[9],
                        "suggested_category_id": cat_id,
                        "note": cols[11] if len(cols) > 11 and cols[11] != '/' else cols[4]
                    })
            self._send_json(0, 200, "success", {"items": parsed_items, "count": len(parsed_items)})
            return

        # Snapshot Restore
        if path == "/api/v1/snapshots/restore":
            if "file" not in parts:
                self._send_json(400, 400, "file field required")
                return
            raw_bytes = parts["file"]["data"]
            tmp_restore_dir = tempfile.mkdtemp()
            tmp_archive = os.path.join(tmp_restore_dir, "restore.ugledger")
            with open(tmp_archive, "wb") as f:
                f.write(raw_bytes)
            try:
                with tarfile.open(tmp_archive, "r:gz") as tar:
                    tar.extractall(tmp_restore_dir)
            except Exception as e:
                import shutil
                shutil.rmtree(tmp_restore_dir, ignore_errors=True)
                self._send_json(400, 400, f"corrupted archive: {e}")
                return

            restored_db = os.path.join(tmp_restore_dir, "accounting.db")
            if not os.path.exists(restored_db):
                import shutil
                shutil.rmtree(tmp_restore_dir, ignore_errors=True)
                self._send_json(400, 400, "invalid archive: accounting.db missing")
                return

            test_conn = sqlite3.connect(restored_db)
            res = test_conn.execute("PRAGMA integrity_check;").fetchone()[0]
            test_conn.close()
            if res != "ok":
                import shutil
                shutil.rmtree(tmp_restore_dir, ignore_errors=True)
                self._send_json(400, 400, "sqlite integrity check failed")
                return

            import shutil
            db_path = get_db_path()
            shutil.copy2(restored_db, db_path)
            restored_att = os.path.join(tmp_restore_dir, "attachments")
            if os.path.exists(restored_att):
                att_dir = get_attachments_dir()
                shutil.rmtree(att_dir, ignore_errors=True)
                shutil.copytree(restored_att, att_dir)
            shutil.rmtree(tmp_restore_dir, ignore_errors=True)
            self._send_json(0, 200, "restore successful")
            return

        # Transaction attachment upload
        m = re.match(r"^/api/v1/transactions/([^/]+)/attachment$", path)
        if m:
            tx_id = m.group(1)
            if "file" not in parts:
                self._send_json(400, 400, "file field required")
                return
            item = parts["file"]
            fname = item["filename"] or "receipt.jpg"
            fbytes = item["data"]
            if len(fbytes) > 10 * 1024 * 1024:
                self._send_json(413, 413, "file too large")
                return
            if not fname.lower().endswith(('.jpg', '.jpeg', '.png', '.webp')):
                self._send_json(415, 415, "unsupported media type")
                return
            att_id = f"att_{os.urandom(4).hex()}"
            dest = os.path.join(get_attachments_dir(), f"{att_id}_{fname}")
            with open(dest, "wb") as f:
                f.write(fbytes)
            conn = get_db()
            conn.execute("""
                INSERT INTO attachments (id, transaction_id, file_name, file_path, file_size, mime_type)
                VALUES (?, ?, ?, ?, ?, ?)
            """, (att_id, tx_id, fname, dest, len(fbytes), "image/jpeg"))
            conn.commit()
            conn.close()
            self._send_json(0, 201, "success", {"attachment_id": att_id, "file_name": fname})
            return

        self._send_json(404, 404, f"not found: {path}")

def run_server(port, data_dir, log_dir, strict_auth=False):
    CONFIG["port"] = port
    CONFIG["data_dir"] = data_dir
    CONFIG["log_dir"] = log_dir
    CONFIG["strict_auth"] = strict_auth
    os.makedirs(data_dir, exist_ok=True)
    os.makedirs(log_dir, exist_ok=True)
    init_db()

    server_address = ('127.0.0.1', port)
    httpd = HTTPServer(server_address, AccountingHandler)
    print(f"Ephemeral backend started on http://127.0.0.1:{port} (Data: {data_dir})", flush=True)
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        httpd.server_close()
        print("Ephemeral backend stopped.", flush=True)

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Seven Cats Accounting Ephemeral Server")
    parser.add_argument("--port", type=int, default=21010)
    parser.add_argument("--data-dir", type=str, default=os.environ.get("UGAPP_DATA_DIR", "./data"))
    parser.add_argument("--log-dir", type=str, default=os.environ.get("UGAPP_LOG_DIR", "./logs"))
    parser.add_argument("--strict-auth", action="store_true")
    args = parser.parse_args()

    run_server(args.port, args.data_dir, args.log_dir, args.strict_auth)
