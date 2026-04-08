import sqlite3
import hashlib
import hmac
import time
import secrets
from config import DB_PATH

SECRET = None

def get_conn():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    global SECRET
    conn = get_conn()
    conn.executescript("""
        CREATE TABLE IF NOT EXISTS licenses (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            username TEXT,
            full_name TEXT,
            license_key TEXT UNIQUE NOT NULL,
            platform TEXT DEFAULT 'all',
            created_at REAL NOT NULL,
            payment_method TEXT,
            confirmed INTEGER DEFAULT 0
        );
        CREATE TABLE IF NOT EXISTS pending_payments (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            username TEXT,
            full_name TEXT,
            created_at REAL NOT NULL,
            status TEXT DEFAULT 'pending'
        );
        CREATE TABLE IF NOT EXISTS feedback (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            username TEXT,
            message TEXT NOT NULL,
            created_at REAL NOT NULL
        );
        CREATE TABLE IF NOT EXISTS meta (
            key TEXT PRIMARY KEY,
            value TEXT
        );
    """)
    # Generate or load signing secret
    row = conn.execute("SELECT value FROM meta WHERE key='secret'").fetchone()
    if row:
        SECRET = row["value"]
    else:
        SECRET = secrets.token_hex(32)
        conn.execute("INSERT INTO meta (key, value) VALUES ('secret', ?)", (SECRET,))
        conn.commit()
    conn.close()

def generate_key(user_id: int) -> str:
    ts = hex(int(time.time()))[2:]
    uid = hex(user_id)[2:]
    payload = f"{uid}-{ts}"
    sig = hmac.new(SECRET.encode(), payload.encode(), hashlib.sha256).hexdigest()[:12].upper()
    # Format: RS-XXXX-XXXX-XXXX
    raw = (uid + ts + sig).upper()[:16].ljust(16, '0')
    key = f"RS-{raw[:4]}-{raw[4:8]}-{raw[8:12]}-{raw[12:16]}"
    return key

def save_license(user_id, username, full_name, key, method="manual"):
    conn = get_conn()
    conn.execute(
        "INSERT INTO licenses (user_id, username, full_name, license_key, created_at, payment_method, confirmed) VALUES (?,?,?,?,?,?,1)",
        (user_id, username, full_name, key, time.time(), method)
    )
    conn.commit()
    conn.close()

def save_pending(user_id, username, full_name):
    conn = get_conn()
    conn.execute(
        "INSERT INTO pending_payments (user_id, username, full_name, created_at) VALUES (?,?,?,?)",
        (user_id, username, full_name, time.time())
    )
    conn.commit()
    conn.close()
    return conn

def confirm_pending(user_id):
    conn = get_conn()
    conn.execute(
        "UPDATE pending_payments SET status='confirmed' WHERE user_id=? AND status='pending'",
        (user_id,)
    )
    conn.commit()
    conn.close()

def get_pending():
    conn = get_conn()
    rows = conn.execute(
        "SELECT * FROM pending_payments WHERE status='pending' ORDER BY created_at DESC"
    ).fetchall()
    conn.close()
    return rows

def has_license(user_id):
    conn = get_conn()
    row = conn.execute(
        "SELECT license_key FROM licenses WHERE user_id=? AND confirmed=1", (user_id,)
    ).fetchone()
    conn.close()
    return row["license_key"] if row else None

def save_feedback(user_id, username, message):
    conn = get_conn()
    conn.execute(
        "INSERT INTO feedback (user_id, username, message, created_at) VALUES (?,?,?,?)",
        (user_id, username, message, time.time())
    )
    conn.commit()
    conn.close()

def get_stats():
    conn = get_conn()
    total = conn.execute("SELECT COUNT(*) as c FROM licenses WHERE confirmed=1").fetchone()["c"]
    pending = conn.execute("SELECT COUNT(*) as c FROM pending_payments WHERE status='pending'").fetchone()["c"]
    feedback_count = conn.execute("SELECT COUNT(*) as c FROM feedback").fetchone()["c"]
    conn.close()
    return {"licenses": total, "pending": pending, "feedback": feedback_count}
