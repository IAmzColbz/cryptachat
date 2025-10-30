import os
import time
import datetime
import jwt # PyJWT
import psycopg2
import psycopg2.extras # For DictCursor
from functools import wraps
from flask import Flask, request, jsonify, g
from werkzeug.security import generate_password_hash, check_password_hash
from flask_limiter import Limiter
from flask_limiter.util import get_remote_address
from dotenv import load_dotenv

# Load environment variables from .env file
# This is useful for local dev; Docker Compose handles it in production
load_dotenv(dotenv_path='../.config/docker.env') 

# --- CONFIGURATION ---
SECRET_KEY = os.getenv('SECRET_KEY', 'a-very-secret-key-that-you-should-change')

# Build database URL from environment variables
DB_NAME = os.getenv('POSTGRES_DB')
DB_USER = os.getenv('POSTGRES_USER')
DB_PASS = os.getenv('POSTGRES_PASSWORD')
DB_HOST = os.getenv('DB_HOST')
DB_PORT = os.getenv('DB_PORT')
DATABASE_URL = f"postgresql://{DB_USER}:{DB_PASS}@{DB_HOST}:{DB_PORT}/{DB_NAME}"

app = Flask(__name__)
app.config['SECRET_KEY'] = SECRET_KEY

# --- RATE LIMITING ---
limiter = Limiter(
    get_remote_address,
    app=app,
    default_limits=["200 per day", "50 per hour"],
    storage_uri="memory://",
    strategy="fixed-window"
)

# --- DATABASE HELPERS ---

def get_db():
    """Get a database connection from the Flask global context."""
    db = getattr(g, '_database', None)
    if db is None:
        try:
            db = g._database = psycopg2.connect(DATABASE_URL)
        except psycopg2.OperationalError as e:
            app.logger.error(f"Failed to connect to database: {e}")
            # This might happen if the DB is not ready, even with healthcheck
            time.sleep(1) # Wait and retry once
            db = g._database = psycopg2.connect(DATABASE_URL)
            
    return db

@app.teardown_appcontext
def close_connection(exception):
    """Close the database connection at the end of the request."""
    db = getattr(g, '_database', None)
    if db is not None:
        db.close()

def init_db():
    """Initialize the database schema."""
    with app.app_context():
        db = get_db()
        cursor = db.cursor()
        # Correct path to schema.sql from /app/server/server.py
        with app.open_resource('schema.sql', mode='r') as f:
            cursor.execute(f.read())
        db.commit()
        cursor.close()
        app.logger.info("Database tables initialized.")

# --- AUTHENTICATION & DECORATORS ---

def token_required(f):
    """A decorator to protect routes that require authentication."""
    @wraps(f)
    def decorated(*args, **kwargs):
        token = None
        if 'Authorization' in request.headers:
            token = request.headers['Authorization'].split(" ")[1]

        if not token:
            return jsonify({'message': 'Token is missing!'}), 401

        try:
            data = jwt.decode(token, app.config['SECRET_KEY'], algorithms=["HS256"])
            db = get_db()
            cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
            cursor.execute(
                'SELECT * FROM users WHERE id = %s', (data['user_id'],)
            )
            current_user = cursor.fetchone()
            cursor.close()
            if not current_user:
                 return jsonify({'message': 'Token is invalid!'}), 401
        except jwt.ExpiredSignatureError:
            return jsonify({'message': 'Token has expired!'}), 401
        except (jwt.InvalidTokenError, psycopg2.Error) as e:
            return jsonify({'message': f'Token is invalid or DB error: {e}'}), 401
            
        return f(current_user, *args, **kwargs)
    return decorated

# --- API ENDPOINTS ---

@app.route('/register', methods=['POST'])
@limiter.limit("10 per hour")
def register_user():
    data = request.get_json()
    if not data or not data.get('username') or not data.get('password'):
        return jsonify({'message': 'Missing username or password'}), 400

    username = data.get('username')
    password_hash = generate_password_hash(data.get('password'), method='pbkdf2:sha256')

    db = get_db()
    try:
        cursor = db.cursor()
        cursor.execute(
            'INSERT INTO users (username, password_hash) VALUES (%s, %s)',
            (username, password_hash)
        )
        db.commit()
        cursor.close()
        return jsonify({'message': 'New user registered successfully!'}), 201
    except psycopg2.errors.UniqueViolation:
        db.rollback()
        return jsonify({'message': 'Username already exists.'}), 409
    except psycopg2.Error as e:
        db.rollback()
        return jsonify({'message': f'Database error: {e}'}), 500

@app.route('/login', methods=['POST'])
@limiter.limit("20 per hour")
def login():
    data = request.get_json()
    if not data or not data.get('username') or not data.get('password'):
        return jsonify({'message': 'Could not verify'}), 401

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    cursor.execute(
        'SELECT * FROM users WHERE username = %s', (data['username'],)
    )
    user = cursor.fetchone()
    cursor.close()

    if not user or not check_password_hash(user['password_hash'], data['password']):
        return jsonify({'message': 'Could not verify! Check username/password.'}), 401

    token = jwt.encode({
        'user_id': user['id'],
        'username': user['username'],
        'exp': datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(hours=24)
    }, app.config['SECRET_KEY'], algorithm="HS256")

    return jsonify({'token': token})

@app.route('/upload_key', methods=['POST'])
@token_required
def upload_key(current_user):
    data = request.get_json()
    if not data or not data.get('public_key'):
        return jsonify({'message': 'Missing public_key'}), 400

    db = get_db()
    cursor = db.cursor()
    # Use INSERT ... ON CONFLICT (user_id) DO UPDATE ...
    cursor.execute(
        """
        INSERT INTO public_keys (user_id, public_key) VALUES (%s, %s)
        ON CONFLICT (user_id) DO UPDATE SET public_key = EXCLUDED.public_key
        """,
        (current_user['id'], data['public_key'])
    )
    db.commit()
    cursor.close()
    return jsonify({'message': 'Public key uploaded successfully.'}), 200

@app.route('/get_key', methods=['GET'])
@token_required
def get_key(current_user):
    username_to_find = request.args.get('username')
    if not username_to_find:
        return jsonify({'message': 'Missing username query parameter.'}), 400

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    cursor.execute(
        """
        SELECT pk.public_key 
        FROM public_keys pk 
        JOIN users u ON u.id = pk.user_id 
        WHERE u.username = %s
        """,
        (username_to_find,)
    )
    key_data = cursor.fetchone()
    cursor.close()

    if not key_data:
        return jsonify({'message': 'User not found or has no public key.'}), 404
    
    return jsonify({'username': username_to_find, 'public_key': key_data['public_key']})

@app.route('/request_chat', methods=['POST'])
@token_required
@limiter.limit("30 per hour")
def request_chat(current_user):
    data = request.get_json()
    recipient_username = data.get('recipient_username')
    if not recipient_username:
        return jsonify({'message': 'Missing recipient_username'}), 400

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    
    cursor.execute('SELECT id FROM users WHERE username = %s', (recipient_username,))
    recipient = cursor.fetchone()
    if not recipient:
        cursor.close()
        return jsonify({'message': 'Recipient user not found.'}), 404

    try:
        cursor.execute(
            'INSERT INTO chat_requests (requester_id, requested_id, status) VALUES (%s, %s, %s)',
            (current_user['id'], recipient['id'], 'pending')
        )
        db.commit()
        cursor.close()
        return jsonify({'message': f'Chat request sent to {recipient_username}.'}), 201
    except psycopg2.errors.UniqueViolation:
        db.rollback()
        cursor.close()
        return jsonify({'message': 'Chat request already pending or accepted.'}), 409

@app.route('/get_chat_requests', methods=['GET'])
@token_required
def get_chat_requests(current_user):
    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    cursor.execute(
        """
        SELECT u.username AS requester_username, cr.status
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requester_id
        WHERE cr.requested_id = %s AND cr.status = 'pending'
        """, (current_user['id'],)
    )
    requests = cursor.fetchall()
    cursor.close()
    
    pending_requests = [dict(row) for row in requests]
    return jsonify({'pending_requests': pending_requests})

@app.route('/accept_chat', methods=['POST'])
@token_required
@limiter.limit("30 per hour")
def accept_chat(current_user):
    data = request.get_json()
    requester_username = data.get('requester_username')
    if not requester_username:
        return jsonify({'message': 'Missing requester_username'}), 400

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    
    cursor.execute('SELECT id FROM users WHERE username = %s', (requester_username,))
    requester = cursor.fetchone()
    if not requester:
        cursor.close()
        return jsonify({'message': 'Requester user not found.'}), 404
        
    cursor.execute(
        """
        UPDATE chat_requests
        SET status = 'accepted'
        WHERE requester_id = %s AND requested_id = %s AND status = 'pending'
        """,
        (requester['id'], current_user['id'])
    )
    
    rowcount = cursor.rowcount
    db.commit()
    cursor.close()
    
    if rowcount == 0:
        return jsonify({'message': 'No pending request found from that user.'}), 404
        
    return jsonify({'message': f'Chat request from {requester_username} accepted!'}), 200

@app.route('/get_contacts', methods=['GET'])
@token_required
def get_contacts(current_user):
    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    my_id = current_user['id']
    
    cursor.execute(
        """
        SELECT u.username
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requested_id
        WHERE cr.requester_id = %s AND cr.status = 'accepted'
        """, (my_id,)
    )
    i_requested = cursor.fetchall()
    
    cursor.execute(
        """
        SELECT u.username
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requester_id
        WHERE cr.requested_id = %s AND cr.status = 'accepted'
        """, (my_id,)
    )
    they_requested = cursor.fetchall()
    cursor.close()

    contacts = set(row['username'] for row in i_requested)
    contacts.update(row['username'] for row in they_requested)
    
    return jsonify({'contacts': list(contacts)})

@app.route('/send_message', methods=['POST'])
@token_required
@limiter.limit("100 per hour")
def send_message(current_user):
    data = request.get_json()
    recipient_username = data.get('recipient_username')
    sender_blob = data.get('sender_blob')
    recipient_blob = data.get('recipient_blob')
    
    if not recipient_username or not sender_blob or not recipient_blob:
        return jsonify({'message': 'Missing recipient_username, sender_blob, or recipient_blob'}), 400

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)
    
    cursor.execute('SELECT id FROM users WHERE username = %s', (recipient_username,))
    recipient = cursor.fetchone()
    if not recipient:
        cursor.close()
        return jsonify({'message': 'Recipient user not found.'}), 404
    
    cursor.execute(
        # We can omit 'timestamp' and let the DB default handle it
        'INSERT INTO messages (sender_id, recipient_id, sender_blob, recipient_blob) VALUES (%s, %s, %s, %s)',
        (current_user['id'], recipient['id'], sender_blob, recipient_blob)
    )
    db.commit()
    cursor.close()
    return jsonify({'message': 'Message sent successfully.'}), 201

@app.route('/get_messages', methods=['GET'])
@token_required
def get_messages(current_user):
    partner_username = request.args.get('username')
    since_id = request.args.get('since_id', 0, type=int) 
    
    if not partner_username:
        return jsonify({'message': 'Missing username query parameter.'}), 400

    db = get_db()
    cursor = db.cursor(cursor_factory=psycopg2.extras.DictCursor)

    cursor.execute('SELECT id FROM users WHERE username = %s', (partner_username,))
    partner = cursor.fetchone()
    if not partner:
        cursor.close()
        return jsonify({'message': 'Partner user not found.'}), 404

    my_id = current_user['id']
    partner_id = partner['id']

    cursor.execute(
        """
        SELECT 
            m.id, 
            m.sender_id, 
            m.recipient_id, 
            m.timestamp, 
            u_sender.username AS sender_username,
            CASE
                WHEN m.sender_id = %s THEN m.sender_blob
                ELSE m.recipient_blob
            END AS encrypted_blob
        FROM messages m
        JOIN users u_sender ON u_sender.id = m.sender_id
        WHERE 
            ((m.sender_id = %s AND m.recipient_id = %s) OR (m.sender_id = %s AND m.recipient_id = %s))
            AND m.id > %s
        ORDER BY m.timestamp ASC
        """,
        (my_id, my_id, partner_id, partner_id, my_id, since_id)
    )
    messages_rows = cursor.fetchall()
    cursor.close()

    messages = [dict(row) for row in messages_rows]
    return jsonify({'messages': messages})

# --- Main ---

if __name__ == '__main__':
    # Give the DB a moment to start, just in case.
    # The healthcheck in docker-compose is the real solution,
    # but this doesn't hurt.
    time.sleep(3) 
    print("Initializing database...")
    init_db()
    print("Starting Flask server at http://0.0.0.0:5000")
    app.run(debug=True, host='0.0.0.0', port=5000)